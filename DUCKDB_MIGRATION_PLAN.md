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

## 1. Precondition: finish Phase 1 — **done**

All tables migrated off SQLBoiler; every DAO writes SQL with `?`
placeholders and routes through `db.Rebind(db.CurrentDialect(), q)`. The
generated `models/` package and the `vattle/sqlboiler`, `nullbio/null.v6`,
`nullbio/inflect`, `pkg/errors` deps are gone. Landed in commit
`041d1dc`.

Status checklist:

- [x] 1a — Article
- [x] 1b — Events
- [x] 1c — Users
- [x] 1d — Menu defs
- [x] 1e — Sermons
- [x] 1f — Pages (JSONB)
- [x] 1g — Charges
- [x] 1h — Images — **no-op**: the legacy `images` table has no DAO or
      reader in the codebase. Inline image handling lives in
      `resource/chimage` and operates on article HTML, not a DB table.
      The migration file `20170419004747_CreateImagesTable.sql` remains
      only as history; nothing reads it.
- [x] 1i — `models/` deleted; sqlboiler/nullbio/pkg-errors removed from
      `go.mod`; `sqlboiler.toml` files gone.

---

## 2. Driver choice — **`github.com/duckdb/duckdb-go/v2` pinned**

Using the official DuckDB project bindings at
`github.com/duckdb/duckdb-go/v2` (v2.10502.0). These register a
`database/sql` driver named `duckdb`, so `sql.Open("duckdb", path)`
slots into `db/connect.go` with no interface changes.

Notes:

- **Pre-built platform libs.** The v2 bindings pull their DuckDB C
  library from sibling modules
  (`github.com/duckdb/duckdb-go-bindings/lib/<os>-<arch>`), so a working
  build does not require a system-installed DuckDB or a separate cgo
  toolchain step beyond what Go already uses. Pulled platforms: darwin
  amd64/arm64, linux amd64/arm64, windows amd64.
- **Version pinning.** Keep the binding version in `go.mod`. The paired
  C library is resolved transitively, so a single `go get` bump moves
  both ends together — no manual ABI alignment.

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

### 3.4 Bootstrap approach — **done**

Landed: `db/schema_duckdb.sql` is the single consolidated schema. The
file is embedded into the binary (`//go:embed` in `db/connect.go`) and
replayed on every DuckDB open. Every object uses `IF NOT EXISTS`, so
the replay is idempotent — existing databases are untouched and any
future additions pick up on the next start.

If we later need true migrations against DuckDB, revisit whether `goose`
can target `duckdb` via its custom-dialect hook, or adopt a simpler
home-grown runner.

---

## 4. Code-level changes

### 4.1 `db/connect.go` — **done**

- Added `DuckDBPath` to `DBOpts` (empty string → in-memory).
- `openDB` switches on `DBType`: Postgres keeps the legacy DSN string;
  DuckDB calls `sql.Open("duckdb", opts.DuckDBPath)` and then replays
  the embedded schema.
- Blank imports for both `_ "github.com/duckdb/duckdb-go/v2"` and
  `_ "github.com/lib/pq"` live in `connect.go`. `Db()` signature
  unchanged.

### 4.2 `db/dialect.go`

Already in place. `CurrentDialect()` already returns `DialectDuckDB` when
`DBOpts.DBType == "duckdb"`. `Rebind` passes through for DuckDB. No changes
needed beyond flipping config.

### 4.3 DAO scan helpers — **done**

The only driver-specific surface in the DAOs is array scanning:
`pq.StringArray` speaks the Postgres `text[]` wire format and cannot
round-trip DuckDB's `VARCHAR[]`.

Landed in `model/types.go`: `StringSlice []string` implements
`sql.Scanner` and `driver.Valuer`.

- `Scan` accepts `[]any` / `[]string` (DuckDB list shape) and falls
  through to `pq.StringArray.Scan` for `[]byte` / `string` (Postgres
  text-array format).
- `Value` dispatches on `db.CurrentDialect()`: DuckDB receives a bare
  `[]string` (go-duckdb's `NamedValueChecker` binds it as `VARCHAR[]`);
  Postgres receives the `{a,b,...}` literal via `pq.StringArray.Value`.

Struct fields swapped: `Article.Categories`, `Event.Categories`,
`Sermon.ScriptureRefs`, `Sermon.Categories`, `Page.AvailablePositions`.
The DAO files themselves are unchanged — the type swap is contained to
`model/*.go`.

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

Landed:

- [x] `db/schema_duckdb.sql` — full schema for fresh install (embedded
      into the binary, replayed on every open).
- [x] `db/connect.go` — DuckDB DSN branch + bootstrap.
- [x] `model/types.go` — `StringSlice` Scanner/Valuer.
- [x] `go.mod` — `github.com/duckdb/duckdb-go/v2` added; `lib/pq`
      retained through the rollback window.
- [x] `scripts/pg_to_duckdb.sql` — data-copy script.
- [x] `README.md` — DuckDB dev setup section appended.

---

## 10. Open items

- Retire `goose` and `db/migrate/` once a DuckDB file has been proven in
  production for long enough to make a Postgres rollback unnecessary.
- Drop Postgres entirely after the rollback window: remove `lib/pq`
  blank import, the Postgres branch in `openDB`, and the dialect branch
  in `StringSlice.Value`. `db.Rebind` can stay since its fast path for
  DuckDB is a no-op.
- Live validation of array round-trip under DuckDB — unit coverage for
  `StringSlice.Scan`/`Value` against the real go-duckdb driver would
  catch go-duckdb version bumps that change list decoding shape.