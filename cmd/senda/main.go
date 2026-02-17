package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/senda-app/senda/config"
	"github.com/senda-app/senda/internal/app"
	"github.com/senda-app/senda/internal/metrics"
)

func main() {
	cfgPath := os.Getenv("SENDA_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	level := slog.LevelInfo
	_ = level.UnmarshalText([]byte(cfg.Log.Level))

	var handler slog.Handler
	if cfg.Log.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	logger := slog.New(handler)

	metrics.Register()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.Bootstrap(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to bootstrap application", "error", err)
		os.Exit(1)
	}
	defer application.Close(ctx)

	// Start River workers in background.
	go func() {
		if err := application.RiverClient.Start(ctx); err != nil {
			logger.Error("river client error", "error", err)
		}
	}()

	if err := application.Server.Start(ctx); err != nil {
		logger.Error("server shutdown", "error", err)
	}
}
