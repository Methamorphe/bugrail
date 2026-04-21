# Changelog

All notable changes to Bugrail are documented here.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Issue lifecycle: resolve, ignore, reopen via HTMX partial updates (no full page reload)
- Issues list: filter by status and platform, cursor-based pagination (50 per page)
- Real-time updates: SSE hub pushes `refresh` events; page content swapped in-place
- Stack trace rendering: frame-by-frame display with in-app/library distinction, context lines, show/hide toggle
- Event context: HTTP request (method, URL, headers), user context (id, email, username, ip), and tags extracted from Sentry payload and displayed per event
- Rate limiting: in-memory token bucket per project (`BUGRAIL_RATE_LIMIT_PER_PROJECT` events/min, default 1000); 429 with `X-Sentry-Rate-Limits` header
- Notifications: ntfy and generic webhook support via `BUGRAIL_NTFY_URL` / `BUGRAIL_WEBHOOK_URL`; fires once on new issue creation
- Raw payload collapsible section per event (Alpine.js toggle)
- `cmd/seed`: test event generator for local development
- Breadcrumbs: displayed per event (newest-first, max 20) with level colorisation
- Filter by environment (dynamic select from distinct issue environments) and level (fatal/error/warning/info/debug)
- Attachments: stored from envelope `attachment` items, downloadable via `GET /attachments/{id}`
- Source maps: JS source maps stored from envelope attachments and used to remap minified stack frames at display time; transparent fallback when no map is found

### Changed
- Issues resolved/ignored are automatically reopened when a new event arrives
- HTMX + Alpine.js replace vanilla JS for all interactive UI elements
- `server.New` accepts `rateLimitPerProject int` and `baseURL string`

## [0.1.0] — Jalon 1

### Added
- Sentry envelope and store endpoint ingestion (`/api/{project_id}/envelope/`, `/api/{project_id}/store/`)
- Gzip and deflate decompression of ingest bodies
- DSN auth from header, query string, and envelope header (in priority order)
- Unknown envelope item types accepted silently (200 OK)
- Event grouping by fingerprint or exception type + value
- Issue and event persistence (SQLite default, Postgres optional)
- Web UI: issues list, issue detail with recent events
- Session-based auth with bcrypt passwords
- CSRF protection (double-submit cookie, `X-CSRF-Token` header for HTMX)
- `bugrail init`: bootstraps first user, org, project, and DSN
- `bugrail serve`: starts the HTTP server
- Automatic database migrations via goose (embedded)
