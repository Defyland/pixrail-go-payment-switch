package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	databaseURL := os.Getenv("PIXRAIL_DATABASE_URL")
	if databaseURL == "" {
		logger.Error("missing_database_url", "env", "PIXRAIL_DATABASE_URL")
		os.Exit(1)
	}

	migrationPath := os.Getenv("PIXRAIL_MIGRATION_PATH")
	if migrationPath == "" {
		migrationPath = "db/migrations/0001_pixrail_core.sql"
	}
	sql, err := os.ReadFile(migrationPath)
	if err != nil {
		logger.Error("migration_read_failed", "path", migrationPath, "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		logger.Error("postgres_pool_failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		logger.Error("migration_apply_failed", "path", migrationPath, "error", err)
		os.Exit(1)
	}
	logger.Info("migration_applied", "path", migrationPath)
}
