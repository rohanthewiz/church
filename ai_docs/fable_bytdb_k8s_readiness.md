# bytdb Migration & Kubernetes Readiness Assessment

**Date:** 2026-07-19
**Scope:** Replacing PostgreSQL with `github.com/rohanthewiz/bytdb` in the church platform, and deploying each church site as a single pod on a shared Linode/Akamai (LKE) cluster.

---

## 1. Verdict

Switching to bytdb is feasible and much closer than a typical DB swap. bytdb speaks the
Postgres wire protocol, its SQL dialect covers everything the church schema and queries
actually use, and the legacy SQLBoiler models do **not** need regeneration. The migration
shrinks to: embed/point at pgwire, clean up migration files, import data, smoke-test.

For deployment: one pod per site on shared LKE is the right architecture. The live WAL
must go on **Linode Block Storage** (real fsync, ~$1/mo per site) — **not** object
storage. Object storage's role is async WAL shipping and backups.

---

## 2. Current Postgres Coupling (church codebase survey)

Module `github.com/rohanthewiz/church`; runnable binaries are sibling site packages
`cema/main.go` and `ccswm/main.go`, which call `db.InitDB` and `church.ServeRWeb()`.

### Connection layer
- Driver: `github.com/lib/pq`, blank-imported in `db/connect.go` / `db/connect2.go`.
- `db/connect.go` — global singleton `*sql.DB`; DSN built by hand, `sslmode=disable`.
- `db/connect2.go` — second handle used only by the sermon importer (`resource/sermon/import2.go`).
- `db/executor.go` — the key seam: `db.Executor` interface (`Exec`/`Query`/`QueryRow`)
  compatible with `*sql.DB`, `*sql.Tx`, and SQLBoiler's `boil.Executor`.

### Data access — two generations
1. **Legacy SQLBoiler** (`vattle/sqlboiler v2.5.0` + `nullbio/null.v6`): 9 tables
   (`articles, charges, events, goose_db_version, images, menu_defs, pages, sermons, users`),
   ~132 call sites across ~22 files in 9 packages (`resource/{article,event,sermon,user,payment,menu}`,
   `page`, `admin`, `admin_controller`). Regeneration is considered risky; newer tables
   deliberately avoid it.
2. **Hand-written SQL via `db.Executor`** (the newer pattern): `resource/chat/queries.go`,
   `resource/prayerwall/prayerwall.go`, `resource/apitoken/apitoken.go`,
   `resource/event/recurrence_queries.go`, `core/idrive/sermon_cache.go` +
   `sermon_cleanup_service.go`, `resource/sermon/import2.go`.

A per-resource Presenter layer (`presenterFromModel` / `modelFromPresenter`) isolates
controllers and views from model shape — only the query files touch the DB API.

### Postgres features actually used
- `BIGSERIAL` PKs on every table; `text[]` on 6 columns (categories, scripture_refs,
  available_positions); `jsonb` on 3 columns (`pages.data` — the page/module layout,
  `users.prefs`, `menu_defs.items`) — treated as **opaque blobs**, all JSON parsing in Go,
  no `->`/`->>` in queries.
- Raw SQL uses `$n` placeholders, `RETURNING id` (chat, prayerwall),
  `ON CONFLICT ... DO UPDATE ... excluded.*` (event_recurrences, sermon_cache_access),
  `ILIKE` + `array_to_string(scripture_refs, ',')` (sermon search in `resource/sermon/api_rweb.go`).
- 4 FKs with `ON DELETE CASCADE` (`event_recurrences→events`, `api_tokens→users`,
  `chat_messages→users`, `prayer_requests→users`); CHECK constraints on `event_recurrences`.
- **Not used anywhere:** triggers, full-text search, stored procedures, uuid/gin,
  explicit transactions (all writes are single-statement autocommit), `numeric`/`decimal`.
- Full column-type inventory across all 12 migrations: `text` (77), `timestamptz`/
  `timestamp with time zone` (24), `boolean` (6), `jsonb` (3), `date` (3), `text[]` (6 cols),
  int variants, `smallint` (2), `bigserial`/`bigint`.

### Storage independent of Postgres
- Sessions/CSRF/form tokens: in-process `core/kvstore` (Redis retired; a leftover live
  `roredis.InitRedis` call remains in `cema/main.go` — dead code, `ccswm` has it commented).
- Sermon media: IDrive e2 S3 via `core/s3ops` / `core/idrive`; FTP mirror via `chftp`.
- Stripe (`charges` mirrors Stripe data), gmail_send — none coupled to the SQL backend.

---

## 3. bytdb Coverage — verified against the local repo

Source of truth: `/Users/ro/projs/go/bytdb` and its skill
`.claude/skills/bytdb-fast-memory-based-db/SKILL.md`. (The GitHub README lags the actual
state considerably.)

| Church requirement | bytdb status |
|---|---|
| Postgres wire protocol (`lib/pq`, pgx, psql, `database/sql`) | `pgwire` module, protocol 3.0 |
| `$n` params, prepared statements | Supported |
| `text[]` | Native type |
| `jsonb` | Native, with `-> ->> #> #>> @> <@ ? ?\| ?& \|\| -` operators |
| `timestamptz`, `timestamp`, `date` | Native types; `now()`/`current_date` in DEFAULTs |
| `serial`/`bigserial` | Durable per-column counters; `smallint/bigint/int2/4/8` aliased (`sql/parser.go:1103`) |
| `RETURNING id` | Supported |
| `ON CONFLICT DO NOTHING/UPDATE`, `excluded.*` | Supported |
| `ILIKE`, `LIKE`, regex operators | Supported |
| `array_to_string`, `lower`, `coalesce`, `array_length` | Implemented (`sql/expr.go:982` et al.) — sermon search runs unmodified |
| FKs incl. `ON DELETE CASCADE` | Supported (MATCH SIMPLE; cascades transitive) |
| CHECK / UNIQUE / NOT NULL | Supported |
| Joins, aggregates, GROUP BY/HAVING, window fns, CTEs, views, UNION | Supported |
| `pg_catalog` / `information_schema` | Synthesized (psql backslash commands work) |
| Transactions + SAVEPOINT | Supported (single writer, serializable) |
| Durability | WAL, fsync-before-ack, group commit, power-loss-tested recovery |
| serr structured errors | Native — matches app logging convention (`logger.LogErr`) |

`NUMERIC(p,s)` is unimplemented but the church schema never uses it. MVCC concurrent
writers, RIGHT/FULL joins, triggers, jsonb indexing, COPY, replication: not implemented,
none needed.

---

## 4. Remaining Migration Work

1. ~~**`OWNER TO` statements**~~ — **RESOLVED 2026-07-19**: bytdb v0.6.1 (commit `0b9969d`,
   plus `pgwire/v0.6.1` with the bumped pin, commit `e047b68`) parses and ignores
   `ALTER TABLE ... OWNER TO` as a no-op. The 11 occurrences in the goose migrations now
   go through unmodified. Pull with `go get github.com/rohanthewiz/bytdb/pgwire@v0.6.1`
   (or `.../bytdb@v0.6.1` for direct engine imports); if the module proxy serves a stale
   404, prefix with `GOPRIVATE=github.com/rohanthewiz/bytdb`.
2. **Goose transaction wrapping** — goose wraps migrations in a transaction by default;
   bytdb requires DDL outside transaction blocks. Add `-- +goose NO TRANSACTION`
   annotations, or bootstrap bytdb from one consolidated schema file.
3. **Wire-level smoke tests** (the two load-bearing assumptions):
   - `lib/pq` binding `types.StringArray` (`{a,b}` array-literal text) into `text[]`.
   - `time.Time` scanning from `timestamptz` over pgwire.
4. **SQLBoiler-over-the-wire check** — generated SQL is vanilla; smoke-test `article`
   (simplest resource) first. No model regeneration needed since the schema is unchanged.
5. **Data migration** — one-time `pg_dump --data-only` → import script.
6. **Future-migration pattern** — bytdb's `ALTER TABLE ADD COLUMN` with `DEFAULT`/`NOT NULL`
   requires an empty table: use add-nullable → backfill → (constraint) on live data.
7. Cleanup: remove dead `roredis.InitRedis` in `cema/main.go`.

### Recommended phasing
1. Prove the wire with the five raw-SQL tables (`chat_messages`, `prayer_requests`,
   `api_tokens`, `event_recurrences`, `sermon_cache_access`) — already `db.Executor`-based.
2. Smoke-test one SQLBoiler resource (article), then the rest.
3. Consolidated schema + data import; retire `lib/pq` DSN pointing at Postgres,
   `sqlboiler.toml`, and the external Postgres dependency.

Embedding option: `pgwire.NewServer(bsql.New(e)).ListenAndServe("127.0.0.1:5433")`
inside each site's `main.go` — one self-contained binary per site, or run standalone
`bytdbd -db app.db -addr 127.0.0.1:5433`.

---

## 5. Kubernetes / Linode Deployment

### Target architecture
One pod per church site on a shared LKE cluster. Each site = single self-contained
binary with embedded bytdb.

### WAL on object storage: NO (for the hot path)
bytdb's durability is fsync-before-ack: small sequential appends synced to stable media
before the caller proceeds. S3-compatible object storage (Linode Object Storage) breaks
every assumption:
- Objects are immutable — no append, no partial write; a WAL does thousands of small appends.
- No real fsync: FUSE shims (s3fs/rclone mount) either fake it (ack before durable —
  silently voids the power-loss guarantee) or do a full object upload per sync
  (50–200 ms per commit vs sub-millisecond).
- No crash-consistency semantics for a recovery path that assumes filesystem ordering.

It would appear to work until the first unclean pod kill — exactly when the WAL matters.

### Correct storage layout
- **Hot path: Linode Block Storage via the CSI driver.** PVC per site; real block device,
  real fsync. Minimum 10 GB @ $0.10/GB/mo = **$1/mo per site** — far more space than needed.
  Standard pattern for single-writer embedded DBs (identical to SQLite/Litestream deployments).
- **Object storage: async replication + backup.**
  - Nightly snapshot CronJob of the db file to a bucket (reuse `core/s3ops` — works
    against Linode Object Storage as-is).
  - Better: litestream-style **WAL shipping** — batch sealed WAL segments to the bucket
    for point-in-time recovery with a seconds-wide data-loss window. Candidate
    `bytdb/replicate` module; the WAL is already the replication log. High-value feature
    beyond the church use case.

### Pod-level considerations
- **replicas: 1, `strategy: Recreate`.** Single-writer + ReadWriteOnce volume means two
  pods can't share the DB; RollingUpdate would deadlock on volume attach. Recreate gives
  seconds of downtime per deploy — acceptable. StatefulSet adds nothing at replicas=1.
- **Node failure:** PVC detaches/reattaches where the pod reschedules (~minutes of
  downtime, no data loss).
- **Sessions** are in-process (`kvstore`) — every pod restart logs users out. Known side
  effect of deploys.
- **Sermon media** already on IDrive e2; local LRU cache can be `emptyDir` (re-warms).
- **Memory:** bytdb requires dataset-in-RAM; church structured data is a few MB.
  256–512 Mi pod request is plenty — one shared 4 GB node (~$24/mo) hosts several sites.
- **Cost per site:** ~$1/mo block volume + a slice of a shared node + pennies of object
  storage.

---

## 6. Phase 1 — COMPLETED 2026-07-19

bytdb is now the **default backend** for both site binaries; Postgres remains an explicit
fallback (`db.type: postgres` in options.yml, or `DB_TYPE=postgres`). Goose was dropped
for the bytdb path — the schema ships as an in-code, idempotent bootstrap.

### Implementation
- `db/connect.go` — bytdb branch: opens the embedded engine, bootstraps schema, serves
  pgwire on a loopback listener (listen-then-serve, no readiness race), then dials it
  with the existing `lib/pq` driver. SQLBoiler + raw `$n` queries run unchanged; only
  the endpoint differs. `db.BytDBWireAddr()` exposes the live address for psql.
- `db/bytdb_schema.go` — consolidated schema (12 tables + indexes), flattened from the
  goose chain, created per-table when missing (checked via `information_schema.tables`).
  `db/bytdb_schema_probe_test.go` regression-tests every DDL statement against a scratch
  engine.
- `config` — new `db: {type, file, listen}` block + `DB_TYPE`/`DB_FILE`/`DB_LISTEN` env
  overrides. Defaults: bytdb, `data/church.db`, `127.0.0.1:0` (ephemeral port so several
  sites share a host without port coordination).
- `cema/main.go`, `ccswm/main.go` — build DBOpts from the new block; PG fallback wired;
  dead `roredis.InitRedis` in cema commented out.
- Dependencies: `bytdb v0.6.2`, `bytdb/pgwire v0.6.2`; church module now `go 1.26.1`
  (go.work files bumped; a local Go ≥1.26 install stops gopls complaining).

### Wire proof: `test_scripts/bytdb_wire_check` — ALL 35 CHECKS PASS
Drives the REAL query functions (chat, prayerwall, apitoken, event recurrence) plus the
load-bearing wire assumptions. Confirmed over lib/pq↔pgwire: `types.StringArray` `{a,b}`
binding and scanning on `text[]`; lossless `timestamptz`↔`time.Time` round-trip; `jsonb`
storage + `->>`; `RETURNING id`; `ON CONFLICT DO UPDATE`; UNIQUE enforcement;
`array_to_string` + `ILIKE` sermon search; FK `ON DELETE CASCADE` fan-out across three
child tables; JOIN + windowed selects.

### New bytdb findings from this phase
1. ~~**`BETWEEN` doesn't parse inside CHECK constraints**~~ — **RESOLVED 2026-07-19**:
   fixed upstream in bytdb v0.6.2; the event_recurrences CHECKs use `BETWEEN` again,
   matching the goose originals.
2. ~~**Placeholders are rejected in LIMIT/OFFSET**~~ (`pq: XX000`) — **RESOLVED
   2026-07-19**: fixed upstream in bytdb v0.6.2; the three raw queries (chat ×2,
   prayerwall ×1) bind `$n` placeholders again.
3. **Every table requires a PRIMARY KEY** — fine, the schema always had them.

## 7. Next Steps

1. Smoke-test a SQLBoiler resource end-to-end (boot a site on bytdb, exercise
   article/page/menu admin flows), then the rest of the legacy surface.
2. `pg_dump --data-only` → bytdb import script for cutover of existing sites.
3. Per-site k8s manifest: Deployment (Recreate) + PVC (linode-block-storage) + backup
   CronJob (engine already exposes `Backup`/`BackupTo`).
4. Design `bytdb/replicate` — WAL shipping to S3-compatible storage; `Engine.ReadLogRange`
   and `LogState` already exist as primitives.
5. ~~Upstream: BETWEEN-in-CHECK and LIMIT/OFFSET placeholder support in bytdb.~~
   Done — both shipped in bytdb v0.6.2 (adopted 2026-07-19).
