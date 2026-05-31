package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
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
	logger.Info("pixrail_runtime_configured",
		"component", "api",
		"environment", cfg.Environment,
		"gomaxprocs", runtime.GOMAXPROCS(0),
		"num_cpu", runtime.NumCPU(),
		"store_driver", cfg.StoreDriver,
	)

	shutdownTracing := observability.ConfigureTracing("pixrail-api", cfg.TracingExporter, os.Stdout)
	shutdownPprof := app.StartPprofServer(cfg.PprofAddr, logger)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = shutdownTracing(ctx)
		_ = shutdownPprof(ctx)
	}()

	store, closeStore, err := app.BuildStore(context.Background(), cfg)
	if err != nil {
		logger.Error("store_initialization_failed", "error", err)
		os.Exit(1)
	}
	defer closeStore()
	service := app.NewService(cfg, store)
	server := api.NewServer(service, cfg.APIKeys, observability.NewMetrics(), logger, api.SecurityConfig{
		ProviderCallbackSecret: cfg.ProviderCallbackSecret,
		SignatureTolerance:     cfg.ProviderSignatureTolerance,
	})
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		logger.Info("pixrail_api_starting", "addr", cfg.Addr, "environment", cfg.Environment, "pprof_addr", cfg.PprofAddr)
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
