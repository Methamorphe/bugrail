# CLAUDE.md

Instructions for Claude Code when working on the Bugrail codebase.

> **Note:** Much of the foundational context lives in [AGENTS.md](./AGENTS.md). Read it first. This file covers what's specific to working _with Claude Code_ on _this_ project: workflow preferences, personal conventions, and traps to avoid.

## Project in one sentence

Bugrail is a self-hosted, single-binary, Go-based error tracker that speaks the Sentry SDK protocol. Everything else is a consequence of that sentence.

## Read before acting

1. **[AGENTS.md](./AGENTS.md)** — architecture, stack, constraints, testing expectations
2. **`docs/architecture.md`** — data flow and component boundaries
3. **`ROADMAP.md`** — current priorities and what's intentionally out of scope

If you're about to work on ingestion, also read the golden-file tests under `testdata/envelopes/` — they are the ground truth for Sentry protocol behavior.

## How I work

I'm a solo developer shipping Bugrail in my spare time. That shapes what I need from you:

- **Short, decisive output.** Propose one approach, explain tradeoffs briefly, move on. Don't hedge with five options unless I ask.
- **Show me diffs, not essays.** When editing code, produce the edit. When explaining, 3-5 sentences max per point.
- **Ask before inventing scope.** If a task implies building something not in the roadmap (new notifier, new endpoint, new page), confirm before coding.
- **I will push back on bad suggestions.** That's not a signal to cave — if you're right, defend your position with evidence. I prefer an honest disagreement to agreement theater.

## Workflow conventions

### Before writing code

1. **Read the relevant files in full.** Don't skim. If I ask you to change a handler, read the handler, its tests, and the interfaces it depends on.
2. **Run `make test` first** if tests exist for the area. I want to know we're starting from green.
3. **State your plan in 3-5 bullets before writing.** Lets me correct course cheaply.

### While writing code

- **Prefer small, focused edits.** If a change touches 5 files, do it in 5 steps with me confirming in between when possible.
- **Keep functions under ~60 lines.** If a function grows beyond that, extract.
- **Write the test first** when the behavior is non-trivial (ingestion, grouping, notifications).
- **Use `sqlc` for queries.** Never write raw `db.Exec` / `db.Query` in handlers. If a query doesn't exist in `queries/`, add it there and regenerate.
- **Respect the SQLite-or-Postgres rule.** Any SQL must work on both unless the feature is explicitly Postgres-only (and then guarded).

### After writing code

1. Run `make lint` and `make test`. Report results.
2. Update `CHANGELOG.md` under the `Unreleased` section if the change is user-visible.
3. If a new dependency was added, justify it in the PR description (why stdlib isn't enough).

## Things Claude tends to do wrong on this project

These are real patterns I've seen. If you find yourself about to do any of these, stop and reconsider.

### "Let me add a helpful abstraction"

Bugrail is deliberately boring. Don't introduce:

- Generic `Repository[T]` patterns
- Event buses or pub/sub layers
- Dependency injection frameworks (manual wiring in `cmd/bugrail/main.go` is fine and correct)
- "Flexible" config systems with plugins

Boring Go code, assembled by hand, is the target aesthetic.

### "We should use a library for that"

Default answer: no. Go's stdlib is excellent. Before adding a dependency, check:

- Is there a stdlib equivalent? Use it.
- Is the dependency maintained? Check last commit, open issues.
- Does it have transitive dependencies? Count them.
- Does it require CGO? If yes, reject unless absolutely necessary.

Current dependencies are in `go.mod`. That list should grow slowly, not quickly.

### "Let me refactor this while I'm here"

Don't. Make the focused change I asked for. If you see something worth refactoring, mention it at the end in one sentence. I'll decide if it's worth a separate task.

### "I'll add a TODO for later"

Almost never. `TODO` comments rot. Either:

- Do it now
- Open a GitHub issue with context
- Leave it alone

If you do add a TODO, include my username and the issue number: `// TODO(johann, #123): ...`

### "Here's a 400-line response"

Respect my time. If the answer is "yes, change line 42 to `x`", just say that. Long explanations are welcome when I ask "why", not when I ask "what".

## Sentry protocol rules (the most important rules)

This is where most bugs happen. Internalize these:

1. **Unknown envelope item types are accepted silently.** Return 200 OK, increment a metric, move on. Never 4xx for unknown items.
2. **Compressed bodies are common.** Always check `Content-Encoding: gzip` / `deflate` and decompress before parsing.
3. **DSN auth has three possible sources.** Header, query string, envelope body. Check in that order.
4. **Rate limit responses must include `X-Sentry-Rate-Limits`.** Format: `retry_after:categories:scope:reason_code`. SDKs parse this and back off.
5. **Event IDs are 32-char hex strings without dashes.** Not standard UUID format. Do not add dashes.
6. **`timestamp` can be ISO 8601 OR Unix epoch as float.** Handle both.
7. **Fingerprinting rules matter.** If `fingerprint` is present in the event, use it. Otherwise fall back to the default algorithm. Changes here break dedup.

When in doubt, consult:

- Golden files in `testdata/envelopes/`
- The official [Sentry SDK Developer Docs](https://develop.sentry.dev/sdk/)
- GlitchTip's source for sanity checking behavior

## Testing with real SDKs

When making ingestion changes, don't trust unit tests alone. Use the docker-compose in `testdata/e2e/`:

```bash
cd testdata/e2e
docker compose up -d            # starts bugrail + test apps
./run-tests.sh                  # triggers errors in each app, asserts receipt
docker compose down
```

This runs real `@sentry/node`, `sentry-php`, `sentry-python`, and `sentry_flutter` (via a small Dart CLI) against your local Bugrail build. If a change breaks the e2e, it will break users.

## Commit and PR style

### Commit messages

Format: `<area>: <short description>` — lowercase, imperative.

Good:

- `ingest: handle gzip-compressed envelopes`
- `notify: add ntfy priority header support`
- `web: fix issue detail 500 on empty breadcrumbs`

Bad:

- `Fix bug` (too vague)
- `WIP` (use draft PR instead)
- `feat(ingest): Handle Gzip-Compressed Envelopes.` (over-formatted)

### PR descriptions

Three sections, short:

```
## What
One sentence.

## Why
One or two sentences. Link to issue if applicable.

## How to verify
A command or test case the reviewer can run.
```

## Language and tone

- **Write English in code, comments, docs, and commits.** French is fine in chat with me.
- **Avoid flowery marketing language in docs.** "Bugrail is a lightweight error tracker" — not "Bugrail is a revolutionary, cutting-edge observability platform."
- **Be honest about limitations.** The README says "beta" because it is beta. Don't oversell.
- **Don't use emojis in code, commits, or docs** unless they're already a convention in that file.

## If you're unsure

Pick one:

1. **Propose, don't commit.** Show me the plan, wait for "go".
2. **Open a GitHub issue.** Title it `[question]` and I'll reply.
3. **Ask me directly in this session.** Short question, short answer.

Silence is the worst option. Claude making a "reasonable assumption" on a critical path has cost me real time before. Prefer a clarifying question to a guess.

## Quick reference

| I want to...              | Use...                                                                |
| ------------------------- | --------------------------------------------------------------------- |
| Add a route               | `internal/web/handlers/` + wire in `internal/web/router.go`           |
| Add a SQL query           | `internal/storage/queries/*.sql` + `make generate`                    |
| Add a migration           | `migrations/NNN_name.sql` with `-- +goose Up/Down`                    |
| Add a notifier            | `internal/notify/<name>.go` + register in `notify/registry.go`        |
| Add a CLI command         | `cmd/bugrail/<cmd>.go` + register in `main.go`                        |
| Add a test envelope       | Drop a `.bin` file in `testdata/envelopes/` with a descriptive name   |
| Add an enterprise feature | Code under `ee/`, license check via `license.Require("feature-name")` |

## Dogfooding

Bugrail is used to monitor Bugrail. Any change to logging, error handling, or observability should consider: "how would this look in Bugrail's own dashboard?"

---

_This file evolves. If you spot guidance that's outdated, flag it._
