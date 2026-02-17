package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/senda-app/senda/config"
	sendahttp "github.com/senda-app/senda/internal/http"
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

	srv := sendahttp.NewServer(cfg, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Start(ctx); err != nil {
		logger.Error("server shutdown", "error", err)
	}
}
