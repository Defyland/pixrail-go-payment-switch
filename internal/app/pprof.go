package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"time"
)

func StartPprofServer(addr string, logger *slog.Logger) func(context.Context) error {
	if addr == "" {
		return func(context.Context) error { return nil }
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	go func() {
		logger.Info("pixrail_pprof_starting", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("pixrail_pprof_failed", "error", err)
		}
	}()
	return server.Shutdown
}
