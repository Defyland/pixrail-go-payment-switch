package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/app"
	"github.com/Defyland/pixrail-go-payment-switch/internal/config"
	"github.com/Defyland/pixrail-go-payment-switch/internal/observability"
	"github.com/Defyland/pixrail-go-payment-switch/internal/switcher"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config_load_failed", "error", err)
		os.Exit(1)
	}

	shutdownTracing := observability.ConfigureTracing("pixrail-worker", cfg.TracingExporter, os.Stdout)
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("pixrail_worker_starting", "interval", cfg.WorkerInterval.String(), "batch_size", cfg.WorkerBatchSize, "environment", cfg.Environment)
	ticker := time.NewTicker(cfg.WorkerInterval)
	defer ticker.Stop()

	runBatch(ctx, logger, service, cfg.WorkerBatchSize)
	for {
		select {
		case <-ctx.Done():
			logger.Info("pixrail_worker_stopped")
			return
		case <-ticker.C:
			runBatch(ctx, logger, service, cfg.WorkerBatchSize)
		}
	}
}

func runBatch(ctx context.Context, logger *slog.Logger, service *switcher.Service, limit int) {
	result, err := service.SubmitPendingSPI(ctx, limit)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		logger.Error("spi_submission_batch_failed", "error", err)
		return
	}
	if result.Scanned == 0 {
		return
	}
	logger.Info("spi_submission_batch_completed",
		"scanned", result.Scanned,
		"submitted", result.Submitted,
		"replayed", result.Replayed,
		"skipped", result.Skipped,
	)
}
