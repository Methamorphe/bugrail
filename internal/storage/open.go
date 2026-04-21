package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pressly/goose/v3"

	"github.com/Methamorphe/bugrail/internal/config"
	"github.com/Methamorphe/bugrail/migrations"

	// Register the pgx database/sql driver for PostgreSQL support.
	_ "github.com/jackc/pgx/v5/stdlib"
	// Register the pure-Go SQLite driver required by single-binary builds.
	_ "modernc.org/sqlite"
)

// Open opens the configured database backend and returns a storage implementation.
func Open(ctx context.Context, cfg config.Config) (Store, error) {
	if cfg.Driver() == config.DriverSQLite {
		if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}

	db, err := sql.Open(cfg.SQLDriverName(), cfg.SQLSource())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if cfg.Driver() == config.DriverSQLite {
		db.SetMaxOpenConns(1)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if cfg.Driver() == config.DriverPostgres {
		return newPostgresStore(db), nil
	}

	return newSQLiteStore(db), nil
}

func runMigrations(ctx context.Context, db *sql.DB, driver config.Driver) error {
	goose.SetBaseFS(migrations.FS)

	dialect := "sqlite3"
	if driver == config.DriverPostgres {
		dialect = "postgres"
	}

	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func randomHex(bytesLen int) (string, error) {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func projectNumericID(projectCount int64) string {
	return strconv.FormatInt(projectCount+1, 10)
}

func normalizeLimit(limit int, fallback int64) int64 {
	if limit <= 0 {
		return fallback
	}
	return int64(limit)
}

func normalizeLimit32(limit int, fallback int32) int32 {
	if limit <= 0 {
		return fallback
	}
	return int32(limit)
}

func isDuplicateConstraint(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "sqlstate 23505")
}
