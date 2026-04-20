# Bugrail

> Self-hosted error tracking that fits on a Raspberry Pi.
> Single binary. SQLite. Sentry SDK compatible.

[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](./LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Methamorphe/bugrail)](https://goreportcard.com/report/github.com/Methamorphe/bugrail)
[![Release](https://img.shields.io/github/v/release/Methamorphe/bugrail)](https://github.com/Methamorphe/bugrail/releases)

---

## Why Bugrail

Error tracking is essential. Running it shouldn't require a PhD in Kubernetes.

- **Sentry self-hosted** needs Kafka, Redis, PostgreSQL, ClickHouse, Snuba, and 40+ containers. 8 GB RAM minimum.
- **GlitchTip** is lighter but still requires Python, Celery, Redis, and a Docker Compose.
- **SaaS alternatives** (Sentry, Rollbar, Bugsnag) start charging the moment you ship anything real.

Bugrail is **one Go binary**. It uses SQLite by default. It runs on a 5€/month VPS. It speaks the same protocol as every Sentry SDK out there — so you don't rewrite your instrumentation to switch.

## Quick start

```bash
# Install
curl -fsSL https://bugrail.dev/install.sh | sh

# Initialize (creates DB, admin user, first project)
bugrail init

# Run
bugrail serve
```

Open `http://localhost:8080`, grab your DSN, point your existing Sentry SDK at it. Done.

```js
// Node.js
import * as Sentry from "@sentry/node";
Sentry.init({ dsn: "http://<your-key>@localhost:8080/1" });
```

```php
// PHP
\Sentry\init(['dsn' => 'http://<your-key>@localhost:8080/1']);
```

```dart
// Flutter
await SentryFlutter.init((options) {
  options.dsn = 'http://<your-key>@localhost:8080/1';
});
```

## Features

### Included, free, forever

- **Sentry SDK compatible** — works with every official Sentry SDK (JavaScript, PHP, Python, Go, Ruby, Flutter, Java/Kotlin, .NET, Rust, Swift, and more)
- **Single binary** — ~20 MB, zero runtime dependencies
- **SQLite or PostgreSQL** — your choice, same features
- **Error grouping & deduplication** — smart fingerprinting, or use your own
- **Source maps** — upload via CLI, get readable JS stack traces
- **Real-time dashboard** — HTMX-powered, no client build step
- **Notifications out of the box** — Slack, Discord, Telegram, email, ntfy, Gotify, generic webhooks
- **Smart alerting rules** — first seen, regressions, high-frequency bursts
- **Retention policies** — keep what matters, drop the rest
- **Dark mode** — obviously

### Coming soon

- [ ] CSP violation reports
- [ ] Release tracking & regression detection
- [ ] Public REST API
- [ ] Webhooks for custom integrations

### Bugrail Team (commercial license)

For organizations that need:

- SSO (SAML, OIDC)
- Full audit log
- Multi-org on a single deployment
- PagerDuty / Opsgenie escalation
- Unlimited retention
- Priority support

See [bugrail.dev/team](https://bugrail.dev/team) for pricing.

## Installation

### Binary (recommended)

```bash
curl -fsSL https://bugrail.dev/install.sh | sh
```

Or download from [releases](https://github.com/Methamorphe/bugrail/releases) — Linux (amd64, arm64), macOS (Intel, Apple Silicon), Windows.

### Docker

```bash
docker run -p 8080:8080 -v bugrail-data:/data ghcr.io/Methamorphe/bugrail:latest
```

### From source

```bash
git clone https://github.com/Methamorphe/bugrail
cd bugrail
go build -o bugrail ./cmd/bugrail
./bugrail serve
```

## Migration from Sentry / GlitchTip

Both products use the same SDK protocol as Bugrail. Migrating means:

1. Install Bugrail
2. Create a project in the dashboard
3. Update the `DSN` in your app's Sentry configuration
4. Redeploy

That's it. Your existing SDK integration, breadcrumbs, contexts, user data — all of it keeps working.

See [docs/migrating-from-sentry.md](./docs/migrating-from-sentry.md) for details.

## Architecture

```
┌─ Your apps ──────────────────────┐
│  @sentry/node, sentry-php, etc.  │
└────────────┬─────────────────────┘
             │ HTTPS POST
             ▼
┌─ Bugrail (single binary) ────────┐
│  Ingestion → Processing → Store  │
│       UI ← Notifications ←       │
└────────────┬─────────────────────┘
             ▼
     SQLite or PostgreSQL
```

No Redis. No Kafka. No ClickHouse. No workers as separate services.

Read the [architecture doc](./docs/architecture.md) if you want the full picture.

## Performance

Running on a Hetzner CX22 (2 vCPU, 4 GB RAM):

| Metric | SQLite | PostgreSQL |
|--------|--------|------------|
| Ingestion (events/s) | ~6 000 | ~22 000 |
| Dashboard load (p99) | 40 ms | 35 ms |
| RAM usage | 80 MB | 120 MB |
| Disk (1M events) | ~800 MB | ~1.2 GB |

These are real numbers from continuous load testing, not marketing. See [benchmarks](./docs/benchmarks.md).

## FAQ

**Is this a Sentry fork?**
No. Bugrail reimplements the Sentry ingestion protocol from scratch in Go. The server code is original. Only the public HTTP protocol is compatible, which is legal and established practice (GlitchTip, Bugsink, and others do the same).

**Is the Sentry SDK free to use with Bugrail?**
Yes. All official Sentry SDKs are MIT, BSD, or Apache 2.0 licensed. You install them like any other dependency and point them at Bugrail.

**Why Go?**
Single binary. Cross-compilation. Fast enough for ingestion-heavy workloads. Ecosystem maturity for HTTP servers and SQLite. And honestly, because maintaining a Python stack was never going to feel lightweight.

**Why not ClickHouse?**
Because 99% of users will never hit the volume where SQLite or Postgres stop being enough. If you're doing a billion events a month, you probably want SigNoz. For the rest of us, Bugrail is the right shape.

**Is it production-ready?**
See the [roadmap](./ROADMAP.md). Current status: **beta**. Use at your own risk for now.

**What about performance monitoring, session replay, distributed tracing?**
Not planned. Bugrail is focused on error tracking. If you need full observability, use SigNoz or Uptrace. Staying narrow is the product.

## Contributing

Contributions are welcome. Before opening a PR:

1. Read [CONTRIBUTING.md](./CONTRIBUTING.md)
2. Open an issue first if it's a non-trivial change
3. Run `make test lint` locally
4. Sign-off your commits (DCO)

## License

- **Core** (everything in this repo outside `ee/`): [BSD-3-Clause](./LICENSE)
- **Enterprise Edition** (code in `ee/`): commercial license, requires a valid key
- **Documentation**: CC-BY-4.0

Bugrail is not affiliated with, endorsed by, or sponsored by Sentry. "Sentry" is a trademark of Functional Software, Inc.

## Links

- Website: [bugrail.dev](https://bugrail.dev)
- Documentation: [docs.bugrail.dev](https://docs.bugrail.dev)
- Issues: [github.com/Methamorphe/bugrail/issues](https://github.com/Methamorphe/bugrail/issues)
- Discussions: [github.com/Methamorphe/bugrail/discussions](https://github.com/Methamorphe/bugrail/discussions)

---

Built by a solo developer, for developers who want to own their stack.
