//go:build integration

package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Methamorphe/bugrail/internal/config"
	"github.com/Methamorphe/bugrail/internal/server"
	"github.com/Methamorphe/bugrail/internal/storage"
)

func TestSQLiteInitLoginIngestAndBrowse(t *testing.T) {
	ctx := context.Background()
	t.Setenv("BUGRAIL_DATABASE_URL", "")
	t.Setenv("BUGRAIL_DATA_DIR", t.TempDir())
	t.Setenv("BUGRAIL_LISTEN_ADDR", "127.0.0.1:8080")

	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	require.NoError(t, runInit(ctx, cfg, logger, []string{
		"--admin-email=admin@example.com",
		"--admin-password=secret123",
		"--org-name=Acme",
		"--project-name=Bugrail",
	}))

	runHTTPFlow(t, ctx, cfg, logger)
}

func TestPostgresInitLoginIngestAndBrowse(t *testing.T) {
	dbURL := strings.TrimSpace(os.Getenv("BUGRAIL_DATABASE_URL"))
	if dbURL == "" {
		t.Skip("BUGRAIL_DATABASE_URL is not set")
	}

	ctx := context.Background()
	t.Setenv("BUGRAIL_DATA_DIR", t.TempDir())
	t.Setenv("BUGRAIL_LISTEN_ADDR", "127.0.0.1:8080")

	cfg := config.Load()
	require.Equal(t, config.DriverPostgres, cfg.Driver())
	resetDatabase(t, cfg)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	require.NoError(t, runInit(ctx, cfg, logger, []string{
		"--admin-email=admin@example.com",
		"--admin-password=secret123",
		"--org-name=Acme",
		"--project-name=Bugrail",
	}))

	runHTTPFlow(t, ctx, cfg, logger)
}

func runHTTPFlow(t *testing.T, ctx context.Context, cfg config.Config, logger *slog.Logger) {
	t.Helper()

	store, err := storage.Open(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.Migrate(ctx))

	handler, err := server.New(store, logger, 1000, "http://localhost:8080")
	require.NoError(t, err)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	publicKey := lookupPublicKey(t, cfg, "1")

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar}

	loginURL := ts.URL + "/login"
	resp, err := client.Get(loginURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	loginParsed, err := url.Parse(loginURL)
	require.NoError(t, err)
	var csrfToken string
	for _, cookie := range jar.Cookies(loginParsed) {
		if cookie.Name == "bugrail_csrf" {
			csrfToken = cookie.Value
		}
	}
	require.NotEmpty(t, csrfToken)

	form := url.Values{
		"email":      {"admin@example.com"},
		"password":   {"secret123"},
		"csrf_token": {csrfToken},
	}
	resp, err = client.PostForm(loginURL, form)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Issues")
	require.NoError(t, resp.Body.Close())

	eventBody := strings.NewReader(`{"event_id":"evt-123","platform":"javascript","message":"checkout failed","culprit":"checkout","exception":{"values":[{"type":"TypeError","value":"boom"}]}}`)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/1/store/?sentry_key=%s", ts.URL, publicKey), eventBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	resp, err = client.Get(ts.URL + "/issues")
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "TypeError: boom")
	require.NoError(t, resp.Body.Close())

	issueID := lookupIssueID(t, cfg)
	resp, err = client.Get(ts.URL + "/issues/" + issueID)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "evt-123")
	require.Contains(t, string(body), "TypeError")
	require.NoError(t, resp.Body.Close())
}

func lookupPublicKey(t *testing.T, cfg config.Config, projectID string) string {
	t.Helper()

	db, err := sql.Open(cfg.SQLDriverName(), cfg.SQLSource())
	require.NoError(t, err)
	defer db.Close()

	var key string
	require.NoError(t, db.QueryRow(fmt.Sprintf(`SELECT public_key FROM project_keys WHERE project_id = '%s'`, projectID)).Scan(&key))
	return key
}

func lookupIssueID(t *testing.T, cfg config.Config) string {
	t.Helper()

	db, err := sql.Open(cfg.SQLDriverName(), cfg.SQLSource())
	require.NoError(t, err)
	defer db.Close()

	var issueID string
	require.NoError(t, db.QueryRow(`SELECT id FROM issues ORDER BY created_at DESC LIMIT 1`).Scan(&issueID))
	return issueID
}

func resetDatabase(t *testing.T, cfg config.Config) {
	t.Helper()

	db, err := sql.Open(cfg.SQLDriverName(), cfg.SQLSource())
	require.NoError(t, err)
	defer db.Close()

	statements := []string{
		"DROP TABLE IF EXISTS events",
		"DROP TABLE IF EXISTS issues",
		"DROP TABLE IF EXISTS project_keys",
		"DROP TABLE IF EXISTS projects",
		"DROP TABLE IF EXISTS organizations",
		"DROP TABLE IF EXISTS sessions",
		"DROP TABLE IF EXISTS users",
		"DROP TABLE IF EXISTS goose_db_version",
	}
	for _, stmt := range statements {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}
}
