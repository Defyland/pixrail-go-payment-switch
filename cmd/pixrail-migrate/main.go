package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	databaseURL := os.Getenv("PIXRAIL_DATABASE_URL")
	if databaseURL == "" {
		logger.Error("missing_database_url", "env", "PIXRAIL_DATABASE_URL")
		os.Exit(1)
	}

	migrationDir := os.Getenv("PIXRAIL_MIGRATION_PATH")
	if migrationDir == "" {
		migrationDir = "db/migrations"
	}

	migrations, err := postgres.LoadMigrations(os.DirFS("."), migrationDir)
	if err != nil {
		logger.Error("migration_read_failed", "path", migrationDir, "error", err)
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

	result, err := postgres.ApplyMigrations(ctx, pool, migrations, time.Now().UTC())
	if err != nil {
		logger.Error("migration_apply_failed", "path", migrationDir, "error", err)
		os.Exit(1)
	}
	logger.Info("migrations_applied", "path", migrationDir, "applied", len(result.Applied), "skipped", len(result.Skipped))
}
