# Replace Redis with in-process kvstore

**Date:** 2026-04-18
**Branch:** `roh/drop-redis`
**Session:** replace-redis-with-kvstore

## Goal

Drop the Redis dependency (via `github.com/rohanthewiz/roredis`) from the
church framework and replace it with an in-process map-based key-value store
that enforces per-entry TTL using a background janitor goroutine plus lazy
expiry on read.

## Why

Redis usage in this repo was minimal — three operations (`Set`/`Get`/`Del`)
across two call sites (sessions, CSRF-like form tokens). For a single-binary
per-site deployment that does not scale horizontally, an external Redis
server is operational overhead without a matching benefit. The workload fits
process memory trivially.

## Scope

Before the change, Redis was used in exactly five places in this repo:

- `resource/session/session.go:52,65,81` — session store, 30 min TTL
- `app/application_controller.go:31,40` — form token store, 1 hr TTL

Plus downstream apps (separate modules) initializing the store:

- `cema/main.go:51` — `roredis.InitRedis(...)` (not part of this repo, left
  for a follow-up bump)
- `ccswm/main.go:45` — same

## Architecture of the replacement

```
                     ┌──────────────────────────────┐
                     │  core/kvstore (singleton)    │
                     │                              │
   Set(k,v,ttl) ───▶ │   items map[string]entry     │
                     │   ┌──────────────────────┐   │
   Get(k) ─────────▶ │   │ value, expiresAt     │   │◀── sweepLoop()
                     │   └──────────────────────┘   │    (60s ticker,
   Del(k) ─────────▶ │        sync.RWMutex          │    drops expired)
                     │                              │
                     │   Lazy eviction on Get:      │
                     │   if now > expiresAt,        │
                     │   delete + return            │
                     │   KeyNotExists.              │
                     └──────────────────────────────┘
```

Design decisions:

- **Singleton package-level store.** The only callers are sessions and form
  tokens; both live in one process. A struct with isolated namespaces is not
  needed today, and the singleton keeps call sites unchanged (same shape as
  `roredis.Set/Get/Del`).
- **`map[string]entry` + `sync.RWMutex`** over `sync.Map`. `Get` may need to
  both read the entry and conditionally delete it under a consistent view for
  lazy expiry, which is awkward on `sync.Map`.
- **Two expiry paths** (lazy-on-read and proactive janitor). The janitor
  bounds memory growth for keys that are never read again (abandoned form
  tokens). Lazy-on-read guarantees correctness regardless of janitor lag.
- **`KeyNotExists` sentinel** is the literal string `"Key does not exist"` —
  the same string the existing auth middleware already searches for via
  `strings.Contains(err.Error(), session.KeyNotExists)`. `session.go`
  re-exports it from `kvstore` so no middleware imports changed.
- **Auto-start janitor in `init()`** with a `sync.Once`. Removes the need
  for downstream apps to call an `Init(...)` function, matching the zero-
  ceremony replacement story.

## Files changed

| File | Status | Notes |
|---|---|---|
| `core/kvstore/kvstore.go` | new | package + Set/Get/Del + janitor |
| `core/kvstore/kvstore_test.go` | new | 8 tests, passes under `-race` |
| `resource/session/session.go` | edit | roredis → kvstore, re-exports `KeyNotExists` |
| `app/application_controller.go` | edit | roredis → kvstore for form tokens |
| `go.mod` / `go.sum` | edit | `roredis` and transitive `go-redis/redis` tidied out |

Diff stat: 4 files changed, 13 insertions(+), 25 deletions(-), plus the new
package (~140 LOC incl. tests and comments).

## Verification

- `go build ./...` — clean
- `go test -race ./core/kvstore/` — passes
- `go test -race ./resource/session/ ./app/` — no tests present in those
  packages; compilation clean
- `go vet ./...` — two warnings, both pre-existing and untouched by this
  change:
  - `resource/calendar/fullcalendar_events.go:22` (struct tag formatting)
  - `auth_controller/auth_middleware.go:44` (unkeyed struct literal)

## Tradeoffs accepted

- **Sessions and form tokens do not survive a process restart.** A deploy
  logs every user out and invalidates any in-flight form submissions. For a
  low-traffic church CMS this is acceptable; users log back in.
- **No horizontal scaling.** Multi-instance deployments would need sticky
  sessions at the load balancer, or a switch back to a shared store (Redis,
  or a Postgres table). Out of scope today.

## Follow-ups

1. **Downstream cleanup.** `cema/main.go:51` and `ccswm/main.go:45` still
   call `roredis.InitRedis(...)`. When those modules bump to this version,
   those calls can be deleted and the `roredis` dep dropped from their
   `go.mod`s.
2. **README update.** `README.md:14,39,99` mention Redis as a requirement
   and installation step. Those paragraphs need a follow-up edit.
3. **Postgres migration (separate decision).** During scoping we also looked
   at replacing Postgres with DuckDB. Verdict: plausible and a clean fit for
   this low-write workload, but the dominant cost is leaving SQLBoiler
   behind (~10k LOC of generated models, no DuckDB driver). Deferred; user
   may revisit.

## Discussion summary

Two exploratory questions preceded the implementation:

1. *"What would it take to drop Redis and use a struct with a TTL
   goroutine?"* — Scoped the surface (5 call sites, 3 operations) and
   flagged that the only real tradeoff was loss of cross-process sharing
   and restart persistence.
2. *"How about using DuckDB as a replacement for both Redis and Postgres?"*
   — Concluded: DuckDB is a bad fit for session/token hot paths (OLAP
   engine, single-writer); it is a reasonable fit for content storage but
   the ~10k LOC SQLBoiler migration cost is the pivotal blocker. Sessions
   should go to an in-memory map regardless of the Postgres decision.

User then approved the Redis drop and deferred the Postgres question.