# SQLBoiler → `database/sql` Migration — Phase 1 Complete

Date: 2026-04-19 21:11
Session: `ceba54e4-dcaa-4347-9102-f696559b1c44`

## Goal

Replace the SQLBoiler ORM with hand-written `database/sql` DAOs as the interim
step on the way to swapping Postgres for DuckDB. Single-instance write-once-
read-many workload; no ORM justified.

End-state of this session: **SQLBoiler is fully removed from the codebase.**
The app builds and all tests pass against Postgres with `lib/pq`. Phase 2
(the actual DuckDB cutover) is documented in `DUCKDB_MIGRATION_PLAN.md` at
the project root.

## Design decisions (recap, for context)

- **Placeholders**: all SQL is written with `?` and rebound at call time by
  a quote-aware `db.Rebind(dialect, q)` helper. Postgres gets `$1…$N`,
  DuckDB gets pass-through.
- **Nullable types**: standardized on stdlib `sql.Null{String,Time,Bool,Int64}`
  in place of `gopkg.in/nullbio/null.v6`. Presenter layer flattens to plain
  Go strings / ints for the view.
- **Arrays**: `pq.StringArray` for now. The DuckDB cutover will swap in a
  `StringSlice` wrapper in `model/scan_types.go` (see the Phase 2 plan).
- **JSONB**: stored as `[]byte` on the model, marshalled/unmarshalled in the
  presenter layer. Keeps `model/` free of domain-shape imports.
- **DAO pattern**: every table has `model/<name>.go` (struct + column list +
  `scanX` helper) and `model/<name>_dao.go` (ByID, BySlug, Query, Insert,
  Update, Delete, plus targeted `Exists…` helpers for bootstrap).
- **Trust model for `condition`/`order` fragments**: same as the legacy
  SQLBoiler signature — internal module config, not user input. Documented
  inline in each `Query…` DAO.

## What was done in this session (Phases 1e–1i)

### Phase 1e — Sermons
- Added `model/sermon.go` (Slug as `sql.NullString`; DateTaught as `time.Time`;
  `ScriptureRefs`/`Categories` as `pq.StringArray`).
- Added `model/sermon_dao.go` (SermonByID/BySlug, QuerySermons, Insert/Update/
  Delete).
- Rewrote `resource/sermon/sermon_queries.go`, `sermon_presenter.go`,
  `api.go`, and `api_rweb.go` to use the new model package.

### Phase 1f — Pages (JSONB + text[])
- Added `model/page.go` (`Data []byte` for the jsonb modules column;
  `AvailablePositions pq.StringArray`).
- Added `model/page_dao.go` including `ExistsPageBySlug` (bootstrap
  idempotency).
- Added `model.ExistsMenuDefBySlug` to `model/menu_def_dao.go` and
  `model.AnyArticleExists` to `model/article_dao.go` so `admin/bootstrap.go`
  no longer needs any SQLBoiler Exists() call.
- Rewrote `page/page_queries.go`, `page/page_presenter.go`,
  `page/presenter_to_from_model.go`.
- Rewrote `admin/bootstrap.go` to seed menus / home page via
  `model.InsertMenuDef` and `model.InsertPage`, and to check for an existing
  welcome article via `model.AnyArticleExists`.

### Phase 1g — Charges
- Added `model/charge.go` (every non-required column is `sql.Null*`; amounts
  as `sql.NullInt64` — cents).
- Added `model/charge_dao.go` (ChargeByID, Insert/Update/Delete).
- Rewrote `resource/payment/payment_model.go`; dropped the `null.v6` wrappers
  in favor of `sql.Null*` literals.

### Phase 1h — Images
- No-op. `resource/chimage/image.go` only processes inline HTML images via
  goquery / bimg; it never touched the `images` table. The generated
  `models/images.go` had no application consumers, so nothing to migrate.

### Phase 1i — Remove SQLBoiler dependencies
- Deleted `church/models/` (the generated SQLBoiler package).
- Deleted `sqlboiler.toml` and `sqlboiler.toml.sample`.
- `go mod tidy` cleaned out: `github.com/vattle/sqlboiler`,
  `gopkg.in/nullbio/null.v6`, `github.com/nullbio/inflect`,
  `github.com/pkg/errors`, `gopkg.in/DATA-DOG/go-sqlmock.v1`.
- `go build ./...` clean; `go test ./...` passes
  (`util/stringops/slugify` test suite; other packages have no tests).
- Pre-existing (unrelated) `go vet` findings remain:
  - `resource/calendar/fullcalendar_events.go:21` — struct tag spacing
  - `auth_controller/auth_middleware.go:44` — unkeyed struct literal

## File inventory (this session)

Created:
- `model/sermon.go`, `model/sermon_dao.go`
- `model/page.go`, `model/page_dao.go`
- `model/charge.go`, `model/charge_dao.go`
- `ai_docs/claude_sessions/2026-0419-2111-sqlboiler-to-duckdb-phase-1.md` (this file)

Modified:
- `model/menu_def_dao.go` — added `ExistsMenuDefBySlug`
- `model/article_dao.go` — added `AnyArticleExists`
- `resource/sermon/{sermon_queries,sermon_presenter,api,api_rweb}.go`
- `page/{page_queries,page_presenter,presenter_to_from_model}.go`
- `resource/payment/payment_model.go`
- `admin/bootstrap.go`

Deleted:
- `church/models/` (all SQLBoiler-generated files)
- `church/sqlboiler.toml`, `church/sqlboiler.toml.sample`

## Verification

```
go build ./...    # clean
go test ./...     # 1 package with tests, passes
```

No credentials or tokens are embedded in these files. Any connection strings
that might appear in the Phase 2 plan reference the existing migration
command and do not contain real production secrets.

## Next step (future session)

Execute **Phase 2 — DuckDB cutover**, following `DUCKDB_MIGRATION_PLAN.md`:

1. Translate schema (BIGSERIAL → BIGINT + sequence; TIMESTAMPTZ → TIMESTAMP;
   text[] → VARCHAR[]; jsonb → JSON).
2. Replace `pq.StringArray` with a local `StringSlice` wrapper in
   `model/scan_types.go`; swap out `lib/pq` for the official
   `github.com/duckdb/duckdb` driver.
3. One-time data migration via DuckDB's `postgres_scanner` extension.
4. Flip `config.Options.DBType` to `"duckdb"` and run full smoke tests.

Rollback plan is documented inline in `DUCKDB_MIGRATION_PLAN.md` under the
"Rollback" section.
