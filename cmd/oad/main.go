package main

import (
	"log/slog"
	"os"

	"github.com/danielpadua/oad/cmd/oad/cmd"
	"github.com/danielpadua/oad/internal/api/middleware"
	"github.com/danielpadua/oad/internal/auth"
	"github.com/danielpadua/oad/internal/logging"
)

func main() {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(logging.NewContextHandler(jsonHandler,
		middleware.CorrelationIDExtractor,
		auth.IdentityExtractor,
	))
	slog.SetDefault(logger)

	cmd.Execute()
}
