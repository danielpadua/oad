package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/danielpadua/oad/internal/api"
	"github.com/danielpadua/oad/internal/api/handler"
	"github.com/danielpadua/oad/internal/audit"
	"github.com/danielpadua/oad/internal/auth"
	"github.com/danielpadua/oad/internal/config"
	"github.com/danielpadua/oad/internal/db"
	"github.com/danielpadua/oad/internal/entity"
	"github.com/danielpadua/oad/internal/entitytype"
	"github.com/danielpadua/oad/internal/overlay"
	"github.com/danielpadua/oad/internal/overlayschema"
	"github.com/danielpadua/oad/internal/relation"
	"github.com/danielpadua/oad/internal/retrieval"
	"github.com/danielpadua/oad/internal/system"
	"github.com/danielpadua/oad/internal/webhook"
	"github.com/danielpadua/oad/internal/webui"
	"github.com/danielpadua/oad/migrations"
)

var (
	flagDatabase        string
	flagAddr            string
	flagAuthMode        string
	flagShutdownTimeout string
	flagLogLevel        string
	flagLogFormat       string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run OAD components",
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the OAD API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServer()
	},
}

func init() {
	serverCmd.Flags().StringVar(&flagDatabase, "database", "", "PostgreSQL DSN (overrides config file and OAD_DATABASE)")
	serverCmd.Flags().StringVar(&flagAddr, "addr", "", "bind address [host]:port (default :8080)")
	serverCmd.Flags().StringVar(&flagAuthMode, "auth-mode", "", "jwt | mtls | both | none (default jwt)")
	serverCmd.Flags().StringVar(&flagShutdownTimeout, "shutdown-timeout", "", "graceful shutdown deadline, e.g. 30s")
	serverCmd.Flags().StringVar(&flagLogLevel, "log-level", "", "debug | info | warn | error (default info)")
	serverCmd.Flags().StringVar(&flagLogFormat, "log-format", "", "json | text (default json)")

	runCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(runCmd)
}

func runServer() error {
	cfg, err := config.Load(config.CLIOptions{
		ConfigFile:      CfgFile,
		Database:        flagDatabase,
		Addr:            flagAddr,
		AuthMode:        flagAuthMode,
		ShutdownTimeout: flagShutdownTimeout,
		LogLevel:        flagLogLevel,
		LogFormat:       flagLogFormat,
	})
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

	var jwtAuth *auth.JWTAuthenticator
	var mtlsAuth *auth.MTLSAuthenticator

	switch cfg.Auth.Mode {
	case "jwt", "both":
		jwtAuth, err = buildJWTAuthenticator(ctx, cfg.Auth.Providers)
		if err != nil {
			return fmt.Errorf("initializing JWT authenticator: %w", err)
		}
		if cfg.Auth.Mode == "both" {
			mtlsAuth = auth.NewMTLSAuthenticator(cfg.Auth.MTLSHeader)
		}
	case "mtls":
		mtlsAuth = auth.NewMTLSAuthenticator(cfg.Auth.MTLSHeader)
	}

	auditSvc := audit.NewService()

	entityTypeRepo := entitytype.NewRepository()
	entityTypeSvc := entitytype.NewService(pool, entityTypeRepo, auditSvc)

	systemRepo := system.NewRepository()
	systemSvc := system.NewService(pool, systemRepo, auditSvc)

	overlaySchemaRepo := overlayschema.NewRepository()
	overlaySchemaSvc := overlayschema.NewService(pool, overlaySchemaRepo, auditSvc)

	entityRepo := entity.NewRepository()
	entitySvc := entity.NewService(pool, entityRepo, auditSvc)

	relationRepo := relation.NewRepository()
	relationSvc := relation.NewService(pool, relationRepo, auditSvc)

	overlayRepo := overlay.NewRepository()
	overlaySvc := overlay.NewService(pool, overlayRepo, auditSvc)

	retrievalRepo := retrieval.NewRepository()
	retrievalSvc := retrieval.NewService(pool, retrievalRepo)

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

		ConfigHandler: handler.NewConfigHandler(cfg),

		WebUIHandler: func() http.Handler {
			h, err := webui.NewHandler()
			if err != nil {
				slog.Warn("embedded UI unavailable", "err", err)
				return nil
			}
			return h
		}(),
	})

	dispatchCtx, cancelDispatch := context.WithCancel(ctx)
	defer cancelDispatch()
	go webhookDispatcher.Run(dispatchCtx)

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

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

func buildJWTAuthenticator(ctx context.Context, providers []config.ProviderConfig) (*auth.JWTAuthenticator, error) {
	authProviders := make([]auth.Provider, len(providers))
	for i, p := range providers {
		authProviders[i] = auth.Provider{
			JWKSURL:  p.Backend.JWKSURL,
			Issuer:   p.Backend.Issuer,
			Audience: p.Backend.Audience,
			ClaimsMapping: auth.ClaimsMapping{
				RolesClaim:    p.Backend.ClaimsMapping.RolesClaim,
				SystemIDClaim: p.Backend.ClaimsMapping.SystemIDClaim,
				DefaultRoles:  p.Backend.ClaimsMapping.DefaultRoles,
			},
		}
	}
	return auth.NewJWTAuthenticator(ctx, authProviders)
}
