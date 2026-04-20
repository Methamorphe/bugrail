# AGENTS.md

This file provides guidance to AI coding agents working with the Bugrail codebase.
It follows the [agents.md](https://agents.md) convention.

For Claude Code-specific workflow preferences, see [CLAUDE.md](./CLAUDE.md).

## Project overview

Bugrail is a self-hosted error tracking service written in Go. It is compatible with the Sentry SDK protocol, meaning any official Sentry SDK can send events to a Bugrail server by changing only the DSN.

Core philosophy:

- **Single binary deployment** is non-negotiable. Never add runtime dependencies on external services (Redis, Kafka, ClickHouse, etc.).
- **Scope is locked to error tracking.** No performance monitoring, no session replay, no logs, no APM, no distributed tracing. If a change expands scope, push back.
- **Simplicity beats cleverness.** Stdlib first, third-party only when stdlib falls short.
- **The SQLite path must always work.** Features must not require PostgreSQL unless explicitly marked.

## Tech stack

- **Language:** Go 1.23+
- **HTTP:** `net/http` (stdlib, with `http.ServeMux`)
- **Database:** SQLite via `modernc.org/sqlite` (pure Go, no CGO). PostgreSQL optional via `pgx/v5`.
- **Queries:** `sqlc` generates typed Go from SQL files. Never write raw queries in handlers.
- **Migrations:** `goose`, embedded in the binary via `//go:embed`.
- **Templates:** `html/template` stdlib, with `//go:embed` of the templates directory.
- **Frontend:** HTMX + Alpine.js + Tailwind (CDN in dev, purged CSS bundled in release).
- **Logging:** `log/slog` stdlib. JSON in production, text in development.
- **Testing:** `testing` stdlib + `github.com/stretchr/testify/require`.
- **Build/release:** `goreleaser`.
- **Lint:** `golangci-lint` with config in `.golangci.yml`.

## Repository layout

```
bugrail/
├── cmd/
│   └── bugrail/                 # main package, CLI entry point
├── internal/
│   ├── ingest/                  # Sentry-compatible HTTP ingestion
│   ├── processor/               # Grouping, dedup, worker pool
│   ├── storage/                 # sqlc-generated code + storage interface
│   │   ├── queries/             # .sql files consumed by sqlc
│   │   └── migrations/          # .sql migration files (goose)
│   ├── notify/                  # Slack, Discord, ntfy, email, webhook...
│   ├── web/                     # UI handlers + embedded templates/static
│   ├── auth/                    # Sessions, login, org management
│   ├── api/                     # Public REST API (phase 2)
│   ├── license/                 # Enterprise license verification
│   └── config/                  # Config loading
├── ee/                          # Enterprise edition (commercial license)
├── migrations/                  # SQL migration files (embedded)
├── testdata/
│   └── envelopes/               # Golden-file Sentry envelopes from real SDKs
├── docs/
├── Makefile
└── go.mod
```

**Never add a new top-level directory without discussing it first.** The layout is part of the product.

## Commands

```bash
# Development
make dev                         # starts bugrail with hot reload (air)
make test                        # runs go test ./...
make test-integration            # runs integration tests (tagged)
make lint                        # runs golangci-lint
make generate                    # runs sqlc generate
make migrate                     # applies pending migrations to local DB

# Release
make build                       # cross-compiles with goreleaser
make docker                      # builds multi-arch docker image

# Database
make db-reset                    # wipes local SQLite dev DB
make db-shell                    # opens sqlite3 CLI on dev DB
```

## Code style and conventions

### Go idioms

- Accept interfaces, return structs.
- Use `context.Context` as the first parameter for any function that does I/O.
- Return `error` as the last return value, never panic in library code.
- Wrap errors with `fmt.Errorf("context: %w", err)` to preserve the chain.
- Use `errors.Is` and `errors.As` for error checking, never string comparison.
- Prefer small interfaces defined at the consumer site.

### Naming

- Package names: short, lowercase, no underscores, no camelCase (`ingest` not `ingestPackage`).
- Exported functions: `PascalCase` with a doc comment starting with the function name.
- Handlers: `handleLogin`, not `LoginHandler` or `HandleLogin`.
- SQL queries: named in UPPER_SNAKE in `queries/*.sql` for sqlc.

### File organization

- One type per file when the type has significant methods.
- Tests in `_test.go` next to the implementation.
- Integration tests under `_integration_test.go` with `//go:build integration`.

### Error handling

Public HTTP handlers must never return a 500 with a raw error message. Use:

```go
if err := doThing(ctx); err != nil {
    slog.ErrorContext(ctx, "do thing failed", "err", err)
    http.Error(w, "internal error", http.StatusInternalServerError)
    return
}
```

For Sentry-compatible ingestion endpoints, follow the [Sentry error response contract](https://develop.sentry.dev/sdk/overview/#usage-for-end-users) strictly. Returning unexpected status codes to Sentry SDKs will log console warnings in user apps.

## Critical constraints

### Do not break Sentry compatibility

The `internal/ingest` package implements the Sentry ingestion protocol. Any change here must:

1. Pass all golden-file tests in `testdata/envelopes/`.
2. Be validated against at least 3 real SDKs from `testdata/e2e/` (JS, PHP, Python minimum).
3. Never return a 4xx or 5xx to a known item type, even if we ignore it (unknown types are silently 200 OK).
4. Preserve `X-Sentry-Rate-Limits` header format when rate limiting.

If you are unsure whether a change affects SDK compatibility, **run the integration test suite** and check for changes in SDK behavior before committing.

### Do not require CGO

`modernc.org/sqlite` is used specifically because it compiles without CGO. Any dependency that requires CGO must be rejected unless there is no reasonable alternative. Single-binary cross-compilation is a product promise.

### Enterprise features go in `ee/`

Anything that requires a valid license key (SSO, audit log, multi-org, PagerDuty, unlimited retention) lives under `ee/`. The BSD-3 core must be fully functional without these features. Check for license presence with:

```go
if err := license.Require("sso"); err != nil {
    return ErrFeatureRequiresLicense
}
```

### Do not add runtime dependencies

Bugrail must keep running with only:

- The binary itself
- A writable data directory
- An HTTP port

**Never** require at runtime: Redis, Kafka, NATS, an external queue, an external search engine, or a separate worker process. Internal goroutines and channels handle background work.

## Testing expectations

Every PR must include:

1. **Unit tests** for new logic (target: 70%+ coverage on `internal/ingest`, `internal/processor`, `internal/storage`).
2. **Golden-file test** if touching ingestion — add a captured envelope to `testdata/envelopes/` and assert round-trip.
3. **Integration test** if changing HTTP surface — lives in `_integration_test.go` with `//go:build integration`.

Run before pushing:

```bash
make lint
make test
make test-integration
```

## Database guidelines

- **Schemas live in `migrations/`**, numbered sequentially: `001_initial.sql`, `002_add_notifiers.sql`, etc.
- **Write reversible migrations** when reasonable. Include both `-- +goose Up` and `-- +goose Down`.
- **SQLite and Postgres compatibility:** use a common SQL subset. Avoid `JSONB` operators (use `JSON` functions), avoid `RETURNING` in INSERTs unless you have a code path for each.
- **Indexes are required** on any column used in WHERE or ORDER BY in hot paths. Bugrail does a lot of `WHERE project_id = ? AND received_at > ? ORDER BY received_at DESC`.
- **Never store secrets in plain text.** Use bcrypt for passwords, random tokens for sessions.

## Security guidelines

- **Input validation first.** Anything from HTTP must be validated before reaching storage or processor.
- **SQL injection:** impossible if you use sqlc. Never concatenate user input into SQL.
- **XSS:** `html/template` auto-escapes. Never use `template.HTML` on user-provided content.
- **CSRF:** enabled on all state-changing routes via middleware.
- **Rate limiting:** per-project on ingestion, per-IP on auth endpoints.
- **Secrets in config:** support env var overrides for all sensitive settings. Never log secrets.

## What NOT to do

- Do not add a new top-level package without opening an issue first.
- Do not add OpenTelemetry, Prometheus scraping endpoints, or APM features.
- Do not add a message queue or background service runner.
- Do not rewrite the frontend in React/Vue/Svelte unless the HTMX approach demonstrably fails.
- Do not add a GraphQL endpoint.
- Do not introduce build steps that require Node.js for the release binary.
- Do not add telemetry / phone-home behavior without explicit opt-in config.
- Do not write documentation that promises features that aren't shipped.

## When in doubt

1. Does this change serve the "single binary, one decision" philosophy?
2. Does this break Sentry SDK compatibility?
3. Does this add runtime dependencies?
4. Does this belong in core or `ee/`?
5. Is there a simpler way?

If you cannot answer "yes → no → no → answered → no" confidently, open an issue for discussion before writing code.

## Contact

- Maintainer: [YOUR NAME] — [your.email@example.com](mailto:your.email@example.com)
- Issues: https://github.com/YOURNAME/bugrail/issues
- Discussions: https://github.com/YOURNAME/bugrail/discussions
