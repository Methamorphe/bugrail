# Roadmap

## Done

### Jalon 1 — Vertical slice
Sentry-compatible ingestion, event grouping, issue list + detail UI, session auth, CSRF, SSE real-time updates.

### Jalon 2 — Usable daily
Issue lifecycle (resolve / ignore / reopen), filters + cursor pagination, stack trace rendering, event context (user, request, tags), rate limiting (token bucket, 429 + `X-Sentry-Rate-Limits`), ntfy + webhook notifications.

### Jalon 3 — Observability quality
Source maps for JavaScript (frame remapping), breadcrumbs display, level + environment filters, attachments (stored as blobs, downloadable).

### Jalon 4 — Readable at a glance
Dashboard with stat cards and SVG event-volume chart, releases page, persistent navigation, issue text search.

---

## Next

### Jalon 5 — Operations
- Email notifications (SMTP, configurable per-project)
- Notifier management UI (add/remove ntfy and webhook targets without restart)
- `bugrail export` — dump all issues/events to JSON for migration
- Health endpoint (`GET /healthz`) with DB ping
- Metrics endpoint (`GET /metrics`) — Prometheus text format, event ingestion counters

### Jalon 5 — Multi-project
- Multiple projects per org (currently bootstrapped as one)
- Per-project DSN management in UI
- Project-level settings (rate limit override, notification targets)

---

## Intentionally out of scope
- Multi-org / team management
- Source maps for languages other than JavaScript
- Performance monitoring (transactions, spans)
- Session replay
- Public REST API (`internal/api/`)
- Enterprise features (`ee/`)
