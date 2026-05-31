package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Migration struct {
	Version  string
	Name     string
	SQL      string
	Checksum string
}

type MigrationResult struct {
	Applied []Migration
	Skipped []Migration
}

func LoadMigrations(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		raw, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(dir, entry.Name())))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		version, _, ok := strings.Cut(entry.Name(), "_")
		if !ok || version == "" {
			return nil, fmt.Errorf("migration %s must start with a version prefix", entry.Name())
		}
		sum := sha256.Sum256(raw)
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     entry.Name(),
			SQL:      string(raw),
			Checksum: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})
	return migrations, nil
}

func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool, migrations []Migration, now time.Time) (MigrationResult, error) {
	if _, err := pool.Exec(ctx, `
		create table if not exists schema_migrations (
		  version text primary key,
		  name text not null,
		  checksum text not null,
		  applied_at timestamptz not null
		)`); err != nil {
		return MigrationResult{}, fmt.Errorf("ensure schema_migrations: %w", err)
	}

	result := MigrationResult{}
	for _, migration := range migrations {
		var currentChecksum string
		err := pool.QueryRow(ctx, `select checksum from schema_migrations where version = $1`, migration.Version).Scan(&currentChecksum)
		if err == nil {
			if currentChecksum != migration.Checksum {
				return result, fmt.Errorf("migration %s checksum mismatch", migration.Name)
			}
			result.Skipped = append(result.Skipped, migration)
			continue
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return result, fmt.Errorf("begin migration %s: %w", migration.Name, err)
		}
		if _, err := tx.Exec(ctx, migration.SQL); err != nil {
			_ = tx.Rollback(ctx)
			return result, fmt.Errorf("apply migration %s: %w", migration.Name, err)
		}
		if _, err := tx.Exec(ctx, `
			insert into schema_migrations (version, name, checksum, applied_at)
			values ($1, $2, $3, $4)`,
			migration.Version, migration.Name, migration.Checksum, now.UTC()); err != nil {
			_ = tx.Rollback(ctx)
			return result, fmt.Errorf("record migration %s: %w", migration.Name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return result, fmt.Errorf("commit migration %s: %w", migration.Name, err)
		}
		result.Applied = append(result.Applied, migration)
	}
	return result, nil
}
