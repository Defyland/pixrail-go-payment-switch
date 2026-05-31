package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/api"
	"github.com/Defyland/pixrail-go-payment-switch/internal/app"
	"github.com/Defyland/pixrail-go-payment-switch/internal/config"
	"github.com/Defyland/pixrail-go-payment-switch/internal/observability"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config_load_failed", "error", err)
		os.Exit(1)
	}

	shutdownTracing := observability.ConfigureTracing("pixrail-api", cfg.TracingExporter, os.Stdout)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = shutdownTracing(ctx)
	}()

	store, closeStore, err := app.BuildStore(context.Background(), cfg)
	if err != nil {
		logger.Error("store_initialization_failed", "error", err)
		os.Exit(1)
	}
	defer closeStore()
	service := app.NewService(cfg, store)
	server := api.NewServer(service, cfg.APIKeys, observability.NewMetrics(), logger)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		logger.Info("pixrail_api_starting", "addr", cfg.Addr, "environment", cfg.Environment)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http_server_failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("http_shutdown_failed", "error", err)
		os.Exit(1)
	}
	logger.Info("pixrail_api_stopped")
}
