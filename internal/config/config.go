package config

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultDataDir    = ".data"
	defaultListenAddr = ":8080"
	sqliteFilename    = "bugrail.sqlite3"
)

// Driver identifies the configured database backend.
type Driver string

const (
	// DriverSQLite uses an on-disk SQLite database.
	DriverSQLite Driver = "sqlite"
	// DriverPostgres uses PostgreSQL via pgx stdlib.
	DriverPostgres Driver = "postgres"
)

// Config captures the process configuration derived from environment variables.
type Config struct {
	DataDir             string
	DatabaseURL         string
	ListenAddr          string
	RateLimitPerProject int
	BaseURLOverride     string
}

// Load reads Bugrail configuration from the environment and applies defaults.
func Load() Config {
	rl := 1000
	if s := strings.TrimSpace(os.Getenv("BUGRAIL_RATE_LIMIT_PER_PROJECT")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			rl = n
		}
	}

	cfg := Config{
		DataDir:             strings.TrimSpace(os.Getenv("BUGRAIL_DATA_DIR")),
		DatabaseURL:         strings.TrimSpace(os.Getenv("BUGRAIL_DATABASE_URL")),
		ListenAddr:          strings.TrimSpace(os.Getenv("BUGRAIL_LISTEN_ADDR")),
		RateLimitPerProject: rl,
		BaseURLOverride:     strings.TrimSpace(os.Getenv("BUGRAIL_BASE_URL")),
	}

	if cfg.DataDir == "" {
		cfg.DataDir = defaultDataDir
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}

	return cfg
}

// Driver returns the configured database backend.
func (c Config) Driver() Driver {
	if c.DatabaseURL != "" {
		return DriverPostgres
	}
	return DriverSQLite
}

// SQLitePath returns the SQLite database file path used when no external database
// URL is configured.
func (c Config) SQLitePath() string {
	return filepath.Join(c.DataDir, sqliteFilename)
}

// SQLDriverName returns the database/sql driver name matching the current config.
func (c Config) SQLDriverName() string {
	if c.Driver() == DriverPostgres {
		return "pgx"
	}
	return "sqlite"
}

// SQLSource returns the DSN/connection string used by database/sql.
func (c Config) SQLSource() string {
	if c.Driver() == DriverPostgres {
		return c.DatabaseURL
	}

	abs, err := filepath.Abs(c.SQLitePath())
	if err != nil {
		abs = filepath.Clean(c.SQLitePath())
	}
	u := &url.URL{
		Scheme:   "file",
		Path:     abs,
		RawQuery: "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)",
	}
	return u.String()
}

// BaseURL returns the public base URL used for DSN generation.
// BUGRAIL_BASE_URL takes precedence; otherwise it is derived from BUGRAIL_LISTEN_ADDR.
func (c Config) BaseURL() string {
	if c.BaseURLOverride != "" {
		return strings.TrimRight(c.BaseURLOverride, "/")
	}
	host, port, err := net.SplitHostPort(c.ListenAddr)
	if err != nil {
		return "http://localhost:8080"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}
