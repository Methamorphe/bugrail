package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaultsToSQLite(t *testing.T) {
	t.Setenv("BUGRAIL_DATA_DIR", "")
	t.Setenv("BUGRAIL_DATABASE_URL", "")
	t.Setenv("BUGRAIL_LISTEN_ADDR", "")

	cfg := Load()

	require.Equal(t, DriverSQLite, cfg.Driver())
	require.Equal(t, ".data", cfg.DataDir)
	require.Equal(t, ":8080", cfg.ListenAddr)
	require.Equal(t, filepath.Join(".data", "bugrail.sqlite3"), cfg.SQLitePath())
	require.Contains(t, cfg.SQLSource(), "file:")
	require.Equal(t, "sqlite", cfg.SQLDriverName())
	require.Equal(t, "http://localhost:8080", cfg.BaseURL())
}

func TestLoadPrefersPostgresURL(t *testing.T) {
	t.Setenv("BUGRAIL_DATA_DIR", "/tmp/bugrail")
	t.Setenv("BUGRAIL_DATABASE_URL", "postgres://user:pass@localhost:5432/bugrail?sslmode=disable")
	t.Setenv("BUGRAIL_LISTEN_ADDR", "127.0.0.1:9090")

	cfg := Load()

	require.Equal(t, DriverPostgres, cfg.Driver())
	require.Equal(t, "pgx", cfg.SQLDriverName())
	require.Equal(t, "postgres://user:pass@localhost:5432/bugrail?sslmode=disable", cfg.SQLSource())
	require.Equal(t, "http://127.0.0.1:9090", cfg.BaseURL())
}
