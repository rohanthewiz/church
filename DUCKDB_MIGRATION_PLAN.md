# DuckDB Migration Plan

Target end-state: DuckDB replaces Postgres as the sole persistence backend.
Workload profile driving this decision: **write-once-read-many, single server
instance**. DuckDB is embedded (no external service), file-backed, and fast
on read-heavy access patterns.

This document covers **Phase 2** (the DB swap) and assumes **Phase 1**
(removing SQLBoiler in favor of hand-written `database/sql` DAOs) is
complete. See `resource/article/article_queries.go` and `model/` for the
Phase 1a pattern.

---

## 1. Precondition: finish Phase 1

All tables migrated off SQLBoiler, every DAO writes SQL with `?` placeholders
and routes through `db.Rebind(db.CurrentDialect(), q)`. The `models/`
package and the `vattle/sqlboiler`, `nullbio/null.v6`, `nullbio/inflect`,
`pkg/errors` deps are removed.

Status checklist:

- [x] 1a — Article
- [ ] 1b — Events
- [ ] 1c — Users
- [ ] 1d — Menu defs
- [ ] 1e — Sermons
- [ ] 1f — Pages (JSONB — extra care on scan helper)
- [ ] 1g — Charges
- [ ] 1h — Images
- [ ] 1i — Delete `models/`, drop sqlboiler/nullbio/pkg-errors from `go.mod`,
      delete `sqlboiler.toml` and `sqlboiler.toml.sample`.

---

## 2. Driver choice

Use the **official DuckDB project bindings** — sourced from
`https://github.com/duckdb/duckdb` (the DuckDB team ships the Go bindings
out of their main repository / adjacent official repos under the
`duckdb` org). Avoid community forks such as `marcboeker/go-duckdb` so we
track upstream releases directly.

Requirements this imposes:

- **cgo**. Build hosts need a C toolchain and the DuckDB C library
  headers/lib available for the target platform. Not a blocker for a
  single-server deployment but flagged for CI images.
- **Version pinning**. Pin the Go binding to the DuckDB library version
  it was built against in `go.mod`; mismatched C lib and bindings cause
  silent ABI breakage.
- **`database/sql` compatibility**. Official bindings register a
  `database/sql` driver named `duckdb`, so `sql.Open("duckdb", path)`
  slots into the existing `db/connect.go` with no interface changes.

If building cgo becomes painful (cross-compile on CI), fallback options
in order of preference: run a DuckDB HTTP/ADBC front-end in-process,
then community forks. None required for current plan.

---

## 3. Schema translation

### 3.1 Column types

| Postgres       | DuckDB                 | Notes                                    |
|----------------|------------------------|------------------------------------------|
| `BIGSERIAL`    | `BIGINT` + sequence    | DuckDB has no SERIAL. Use `CREATE SEQUENCE …_id_seq; id BIGINT PRIMARY KEY DEFAULT nextval('…_id_seq')`. |
| `TIMESTAMPTZ`  | `TIMESTAMPTZ`          | DuckDB alias for `TIMESTAMP WITH TIME ZONE`. Same wire format. |
| `TEXT`         | `VARCHAR` (unbounded)  | `TEXT` is also accepted as an alias.     |
| `BOOLEAN`      | `BOOLEAN`              | Identical.                               |
| `text[]`       | `VARCHAR[]`            | First-class list type.                   |
| `JSONB`        | `JSON`                 | Single JSON type in DuckDB.              |
| `BIGINT`       | `BIGINT`               | Identical.                               |

### 3.2 Statements to drop

- `ALTER TABLE … OWNER TO "devuser";` — no ownership concept (file-backed).
- `GRANT` / role management — not applicable.
- `sslmode=disable` DSN bits — no network.

### 3.3 Indexes

DuckDB supports `CREATE UNIQUE INDEX` and `CREATE INDEX`. The existing
`idx_articles_slug`, `idx_articles_published`, etc., translate directly.
DuckDB indexes are ART-based and mainly accelerate point lookups and
uniqueness checks — secondary indexes for analytical scans are generally
unnecessary (columnar storage handles that).

### 3.4 Bootstrap approach

The write-once-read-many profile means migration history is low value.
Proposed: collapse `db/migrate/*.sql` into a single
`db/schema_duckdb.sql` file that the app executes if the target DB file
doesn't exist. Migrations-as-history is simpler to retire than to port.

If we later need migrations against DuckDB, revisit whether
`goose` can target `duckdb` via its custom-dialect hook, or adopt a
simpler home-grown migration runner.

---

## 4. Code-level changes

### 4.1 `db/connect.go`

- Add a `DuckDBPath` field to `DBOpts` (file path; empty string → in-memory).
- Branch on `opts.DBType`:
  - `postgres`: unchanged DSN construction.
  - `duckdb`: `sql.Open("duckdb", opts.DuckDBPath)`.
- Import the DuckDB driver with a blank import alongside `_ "github.com/lib/pq"`.
- Keep `Db()` signature unchanged so nothing downstream moves.

### 4.2 `db/dialect.go`

Already in place. `CurrentDialect()` already returns `DialectDuckDB` when
`DBOpts.DBType == "duckdb"`. `Rebind` passes through for DuckDB. No changes
needed beyond flipping config.

### 4.3 DAO scan helpers

The only driver-specific surface in the DAOs is **array scanning**.
`pq.StringArray` implements `sql.Scanner` by parsing Postgres's text array
format; it will **not** correctly scan DuckDB's `VARCHAR[]`.

Preferred approach — **typed wrapper in `model/types.go`**:

```go
// model/types.go (sketch — finalize during Phase 2)
type StringSlice []string

func (s *StringSlice) Scan(src any) error {
    switch v := src.(type) {
    case []any:           // DuckDB drives lists as []any
        out := make([]string, len(v))
        for i, x := range v { out[i], _ = x.(string) }
        *s = out
        return nil
    case string, []byte:  // Postgres text[] format
        return (*pq.StringArray)(s).Scan(src)
    }
    return fmt.Errorf("unsupported array source %T", src)
}

func (s StringSlice) Value() (driver.Value, error) {
    // Insert path: dispatch on db.CurrentDialect() — DuckDB accepts []any,
    // Postgres wants the text array format pq.StringArray emits.
}
```

Every `Article.Categories`, `Page.AvailablePositions`, etc., switches to
`StringSlice`. The DAO files themselves don't change; the type swap lives
in `model/*.go` struct definitions. Rejected alternative: build-tagged
dual DAO files — too much duplication.

### 4.4 JSON (`pages.data`)

Already `json.RawMessage` / `[]byte` in Phase 1 — driver-neutral. Verify
round-trip under DuckDB (DuckDB returns JSON as string; stdlib scan into
`[]byte` still works).

### 4.5 Timestamps

`sql.NullTime` works with both drivers. `NOW()` is valid in both. No change.

### 4.6 RETURNING

Both drivers support `RETURNING id, created_at, updated_at` on INSERT and
`RETURNING updated_at` on UPDATE. No change.

### 4.7 Sequences

When bootstrapping DuckDB from `schema_duckdb.sql` we emit one sequence
per table. INSERT statements continue to omit the id column; the default
expression handles it. Same SQL as Phase 1; no DAO churn.

---

## 5. Data migration

Single cut-over, not online. Steps:

1. Stop the server.
2. Create fresh `church.duckdb` file by running `schema_duckdb.sql`.
3. Copy data from Postgres using DuckDB's `postgres` extension:
   ```sql
   INSTALL postgres; LOAD postgres;
   ATTACH 'postgresql://devuser:secret@localhost/church_development' AS pg (TYPE postgres);

   INSERT INTO articles   SELECT * FROM pg.public.articles;
   INSERT INTO events     SELECT * FROM pg.public.events;
   INSERT INTO users      SELECT * FROM pg.public.users;
   INSERT INTO menu_defs  SELECT * FROM pg.public.menu_defs;
   INSERT INTO sermons    SELECT * FROM pg.public.sermons;
   INSERT INTO pages      SELECT * FROM pg.public.pages;
   INSERT INTO charges    SELECT * FROM pg.public.charges;
   INSERT INTO images     SELECT * FROM pg.public.images;
   ```
4. Advance each sequence past the max id:
   ```sql
   SELECT setval('articles_id_seq', (SELECT COALESCE(MAX(id), 0) FROM articles));
   -- repeat per table
   ```
5. Update app config (`DBType: duckdb`, `DuckDBPath: ./church.duckdb`),
   start server, smoke-test.
6. Keep the Postgres instance + `pg_dump` snapshot for a rollback window.

Alternative if the `postgres` DuckDB extension gives trouble: `pg_dump` →
CSV per table → `COPY articles FROM 'articles.csv' (HEADER, FORMAT CSV)`.
Slightly more manual but deterministic.

---

## 6. Concurrency notes

DuckDB permits one writer process and many reader connections within that
process. Our deployment (single server instance) fits cleanly. The
`*sql.DB` pool can keep its default size; DuckDB serializes writes
internally, and our write volume is minimal anyway.

The kvstore (sessions, form tokens) is already in-process — unaffected.

---

## 7. Rollback

- Postgres stays running during the cut-over window.
- Config-level flip back: set `DBType: postgres` in config, restart. Any
  writes made against DuckDB during the window are lost (acceptable for
  our workload, but can be mitigated by replaying writes to Postgres in
  parallel during a short overlap if needed).
- Git revert of the DuckDB driver import if the driver itself is the
  problem.

---

## 8. Validation plan

Before cut-over:

1. Run the app against a fresh DuckDB file in a dev environment.
2. Full smoke: admin CRUD for every module form (article, event, sermon,
   user, menu, page, payment), public rendering, sermon FTP import,
   payment charge path, image upload.
3. Check JSON round-trip in `pages.data` (module list survives
   save → reload).
4. Check array round-trip in `articles.categories` and
   `pages.available_positions`.
5. Spot-check timestamp formatting (TZ preserved on display).

After cut-over:

1. Re-run the smoke list against production DuckDB file.
2. Monitor log for any `sql.ErrNoRows` / scan errors over the first hour.

---

## 9. Deliverables

- `db/schema_duckdb.sql` — full schema for fresh install.
- `db/connect.go` — DuckDB DSN branch.
- `model/types.go` — `StringSlice` scanner/valuer.
- `go.mod` — official DuckDB bindings added, `lib/pq` eventually removed
  (keep through the rollback window).
- `scripts/pg_to_duckdb.sql` — the data-copy script above, parameterized.
- Updated `README.md` section on local dev (DuckDB file path instead of
  Postgres DSN).

---

## 10. Open items

- Confirm DuckDB version target and matching binding version; pin both.
- Decide whether to keep `goose` for DuckDB or retire it entirely.
- Whether to keep the Postgres code path as a supported alternate (dual
  dialect already works via `db.Rebind`; cost is maintaining two scan
  paths for arrays). Default recommendation: **drop Postgres entirely
  once cut-over is stable** — fewer moving parts.