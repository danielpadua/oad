package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danielpadua/oad/internal/api"
	"github.com/danielpadua/oad/internal/api/handler"
	"github.com/danielpadua/oad/internal/api/middleware"
	"github.com/danielpadua/oad/internal/audit"
	"github.com/danielpadua/oad/internal/auth"
	"github.com/danielpadua/oad/internal/config"
	"github.com/danielpadua/oad/internal/db"
	"github.com/danielpadua/oad/internal/entity"
	"github.com/danielpadua/oad/internal/entitytype"
	"github.com/danielpadua/oad/internal/logging"
	"github.com/danielpadua/oad/internal/overlay"
	"github.com/danielpadua/oad/internal/overlayschema"
	"github.com/danielpadua/oad/internal/relation"
	"github.com/danielpadua/oad/internal/retrieval"
	"github.com/danielpadua/oad/internal/system"
	"github.com/danielpadua/oad/internal/webhook"
	"github.com/danielpadua/oad/migrations"
)

func main() {
	// Initialize structured JSON logger with context-aware enrichment.
	// The ContextHandler automatically injects correlation_id and actor identity
	// into every log record from the request context.
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(logging.NewContextHandler(jsonHandler,
		middleware.CorrelationIDExtractor,
		auth.IdentityExtractor,
	))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	slog.Info("running database migrations")

	if err := db.Migrate(cfg.Database.URL, migrations.FS); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	slog.Info("migrations applied successfully")

	// Initialize authenticators based on configured auth mode.
	var jwtAuth *auth.JWTAuthenticator
	var mtlsAuth *auth.MTLSAuthenticator

	switch cfg.Auth.Mode {
	case "jwt", "both":
		var err error
		jwtAuth, err = auth.NewJWTAuthenticator(ctx, cfg.Auth.JWKSURLs, cfg.Auth.JWTAudience, cfg.Auth.JWTIssuers)
		if err != nil {
			return fmt.Errorf("initializing JWT authenticator: %w", err)
		}
		if cfg.Auth.Mode == "both" {
			mtlsAuth = auth.NewMTLSAuthenticator(cfg.Auth.MTLSHeader)
		}
	case "mtls":
		mtlsAuth = auth.NewMTLSAuthenticator(cfg.Auth.MTLSHeader)
	}

	// --- Phase 2: Schema Registry ---
	auditSvc := audit.NewService()

	entityTypeRepo := entitytype.NewRepository()
	entityTypeSvc := entitytype.NewService(pool, entityTypeRepo, auditSvc)

	systemRepo := system.NewRepository()
	systemSvc := system.NewService(pool, systemRepo, auditSvc)

	overlaySchemaRepo := overlayschema.NewRepository()
	overlaySchemaSvc := overlayschema.NewService(pool, overlaySchemaRepo, auditSvc)

	// --- Phase 3: Entity & Relation Management ---
	entityRepo := entity.NewRepository()
	entitySvc := entity.NewService(pool, entityRepo, auditSvc)

	relationRepo := relation.NewRepository()
	relationSvc := relation.NewService(pool, relationRepo, auditSvc)

	// --- Phase 4: Overlay System ---
	overlayRepo := overlay.NewRepository()
	overlaySvc := overlay.NewService(pool, overlayRepo, auditSvc)

	// --- Phase 5: Retrieval API ---
	retrievalRepo := retrieval.NewRepository()
	retrievalSvc := retrieval.NewService(pool, retrievalRepo)

	// --- Phase 6: Webhooks ---
	webhookRepo := webhook.NewRepository()
	webhookSvc := webhook.NewService(pool, webhookRepo, auditSvc)
	webhookDispatcher := webhook.NewDispatcher(pool, webhookRepo, slog.Default())

	router := api.NewRouter(api.Dependencies{
		DB:       pool,
		Config:   cfg,
		Logger:   slog.Default(),
		JWTAuth:  jwtAuth,
		MTLSAuth: mtlsAuth,

		EntityTypeHandler:    handler.NewEntityTypeHandler(entityTypeSvc),
		SystemHandler:        handler.NewSystemHandler(systemSvc),
		OverlaySchemaHandler: handler.NewOverlaySchemaHandler(overlaySchemaSvc),

		EntityHandler:   handler.NewEntityHandler(entitySvc),
		RelationHandler: handler.NewRelationHandler(relationSvc, retrievalSvc),

		OverlayHandler: handler.NewOverlayHandler(overlaySvc),

		RetrievalHandler: handler.NewRetrievalHandler(retrievalSvc),

		WebhookHandler: handler.NewWebhookHandler(webhookSvc),

		StatsHandler: handler.NewStatsHandler(pool),
	})

	// Start the webhook dispatcher as a background goroutine.
	// It shares the server's lifecycle: cancellation propagates via ctx.
	dispatchCtx, cancelDispatch := context.WithCancel(ctx)
	defer cancelDispatch()
	go webhookDispatcher.Run(dispatchCtx)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the HTTP server in a goroutine so the main goroutine can
	// block on the OS signal channel below for graceful shutdown.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	// Block until SIGINT / SIGTERM or a fatal server error.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		slog.Info("shutdown signal received", "signal", sig)
	}

	slog.Info("shutting down server", "timeout", cfg.Server.ShutdownTimeout)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	slog.Info("server stopped")

	return nil
}
