---
date: 2026-04-19 21:55
session: duckdb-phase-2-migration
branch: roh/drop-sqlboiler
commit: bf89418
---

# DuckDB Phase 2 — backend swap

Landed DuckDB alongside Postgres as a selectable `database/sql` backend
for the church platform. Postgres remains default; flipping config
activates DuckDB with no DAO churn.

## Starting state

- Phase 1 (drop SQLBoiler for hand-written DAOs) was already in at
  commit `041d1dc`. `models/` gone; sqlboiler/nullbio/pkg-errors
  removed; `db.Rebind(db.CurrentDialect(), q)` already threaded through
  every DAO with `?` placeholders.
- `DUCKDB_MIGRATION_PLAN.md` existed but was stale on two fronts:
  - Claimed only `1a — Article` was done on the Phase 1 checklist.
  - Named the DuckDB Go driver as "official bindings from
    `github.com/duckdb/duckdb`" — incorrect. User corrected: it's
    `github.com/duckdb/duckdb-go/v2`.

Both worth noting for next time: **the plan doc checklist drifts
behind reality fast**, re-derive status from the code before trusting it.

## Work order — user asked for a 3-split PR-shaped delivery, committed as one

- (i) driver import + `schema_duckdb.sql` + `db/connect.go` branch
- (ii) `model/types.go` StringSlice Scanner/Valuer + struct swaps
- (iii) data-copy script, plan doc refresh, README section

After all three splits landed, user asked for a single Phase-2 commit
rather than three. Worked out to 14 files, +704/-151. Commit `bf89418`.

## Key technical decisions

### Driver: `github.com/duckdb/duckdb-go/v2`, v2.10502.0

- Pulls platform-specific bindings (darwin arm64/amd64, linux
  arm64/amd64, windows amd64) as indirect modules — no system DuckDB
  lib needed, just `go get` and build.
- Registers a `database/sql` driver named `"duckdb"` so
  `sql.Open("duckdb", path)` drops into the existing connect code.

### Schema approach: embed + idempotent replay

- `db/schema_duckdb.sql` is `//go:embed`-ed into the binary and
  replayed on every DuckDB open. Every object uses `IF NOT EXISTS` so
  re-runs are cheap no-ops. Beats shipping a separate migration runner
  for a single-server embedded DB.
- Bootstrap splits the file on `;` and `Exec`s each statement: tested
  observation is that go-duckdb's `Exec` runs only the first statement
  from a multi-statement string. Simple split is safe here because the
  schema has no string literals containing semicolons.

### Type translation

| Postgres      | DuckDB              | Note                                  |
|---------------|---------------------|---------------------------------------|
| `BIGSERIAL`   | `BIGINT` + sequence | `CREATE SEQUENCE IF NOT EXISTS …`     |
| `text[]`      | `VARCHAR[]`         | First-class list type                 |
| `jsonb`       | `JSON`              | Single JSON type                      |
| `timestamptz` | `TIMESTAMPTZ`       | Same name, same behavior              |
| `timestamp`   | `TIMESTAMP`         | Sermons' `date_taught` — TZ-naïve kept|

Legacy `images` migration file dropped — no code reads that table.
Inline image handling lives in `resource/chimage`, operating on article
HTML, not a DB row.

### `StringSlice` — the one piece of driver-specific glue

`model/types.go`. Implements `sql.Scanner` and `driver.Valuer` on a
`type StringSlice []string`.

- **Scan**: dispatches on `src`'s concrete type:
  - `[]any` — DuckDB's default list shape. Type-assert each element as
    string.
  - `[]string` — defensive branch for possible driver future.
  - `[]byte` / `string` — Postgres text-array wire format; delegated
    to `pq.StringArray.Scan` so the canonical parser stays there.
  - `nil` — empty slice, not an error (some tests write NOT NULL
    columns via zero values).
- **Value**: dispatches on `db.CurrentDialect()`:
  - DuckDB — return a fresh `[]string` copy. go-duckdb implements
    `driver.NamedValueChecker` and binds arbitrary Go slices as list
    parameters. Important: returning `pq.StringArray.Value()`'s
    `{a,b,...}` byte format instead would get bound as a single
    VARCHAR — silent-ish data corruption.
  - Postgres — delegate to `pq.StringArray.Value()`.

Gotcha worth remembering: `driver.Value` is typed `any`, so returning
`[]string` compiles. The reason it *works* at the driver is go-duckdb's
`CheckNamedValue` — if we ever swap to a DuckDB driver without that,
this path breaks.

Swapped fields: `Article.Categories`, `Event.Categories`,
`Sermon.ScriptureRefs`, `Sermon.Categories`, `Page.AvailablePositions`.
DAO SQL unchanged; Go's assignability rule (named-to-unnamed-underlying
matches) means callers that assign `[]string{...}` still compile.

### Data cut-over: `scripts/pg_to_duckdb.sql`

- Uses DuckDB's `postgres` extension (`INSTALL postgres; LOAD postgres;
  ATTACH 'postgresql://…' AS pg (TYPE postgres, READ_ONLY);`).
- One explicit-column-list `INSERT … SELECT` per table. Explicit is
  load-bearing — silent column reorder is the failure mode.
- `setval(seq, COALESCE(MAX(id), 0))` per sequence so next app INSERT
  doesn't collide.
- Connection string has a placeholder; user edits in place. *(Any
  placeholder password in the script is a non-secret sample value; the
  real DSN is supplied at runtime.)*

## Pre-existing vet warnings (unrelated, left alone)

- `resource/calendar/fullcalendar_events.go:21` — space in struct tag.
- `auth_controller/auth_middleware.go:44` — unkeyed fields in literal.

## What's NOT done

- No actual cut-over. Postgres remains default. `DBType: duckdb` + a
  `DuckDBPath` in config activates the new path.
- No tests exercise `StringSlice` round-trip against real go-duckdb.
  Noted in plan §10 as a future item — catches go-duckdb version bumps
  that change list decoding shape.
- `lib/pq` still in go.mod — kept for the rollback window. After
  DuckDB is proven in prod, drop `_ "github.com/lib/pq"`, the postgres
  branch in `openDB`, and the dialect branch in `StringSlice.Value`.
- `db/migrate/` and goose still present. Retire after Postgres rollback
  window closes.
- `db/connect2.go` (secondary handle used only by
  `resource/sermon/import2.go`) was not updated. Still Postgres-only.
  Low priority — the blank import in `connect.go` registers both
  drivers process-wide, so DuckDB *could* be used there too if
  `connect2` ever grows a DBType branch.

## Files changed (commit bf89418)

| File                                       | Kind     |
|--------------------------------------------|----------|
| `db/schema_duckdb.sql`                     | new      |
| `db/connect.go`                            | modified |
| `model/types.go`                           | new      |
| `model/article.go`                         | modified |
| `model/event.go`                           | modified |
| `model/page.go`                            | modified |
| `model/sermon.go`                          | modified |
| `resource/article/article_presenter.go`    | modified |
| `resource/sermon/sermon_presenter.go`      | modified |
| `scripts/pg_to_duckdb.sql`                 | new      |
| `DUCKDB_MIGRATION_PLAN.md`                 | refreshed|
| `README.md`                                | appended |
| `go.mod`, `go.sum`                         | modified |

## For future sessions

- Before recommending anything from this plan, verify the current code
  still matches — the plan got stale on Phase 1 status inside a single
  phase-2 work window.
- If you're tempted to "just use pq.StringArray for arrays on DuckDB",
  don't. The wire format mismatch is the whole reason `StringSlice`
  exists.
- `db.CurrentDialect()` reads a package-global (`dbOpts`). Safe today
  because there's one primary handle. If a second `DBOpts` ever gets
  introduced alongside the existing `connect2.go` pattern and diverges
  in `DBType`, the dialect dispatch in `StringSlice.Value` becomes
  ambiguous — rethink.
