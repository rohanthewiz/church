# JSON → VARCHAR Cast Fix on Read

**Date:** 2026-05-02 12:17
**Session ID:** 2bc6f4f9-434c-4f54-a4ec-6a1be03e2986
**Branch:** roh/drop-sqlboiler

> Continuation of the same Claude session as
> `2026-0502-1155-fix-duckdb-integration.md`. Saved separately because
> the third bug surfaced after the prior `/exp` and is logically its
> own fix.

## Context

After fixing the schema bootstrap and the JSON-NULL bind path
(see prior session note), `ccgrand`'s admin UI surfaced a third
DuckDB-only failure: scanning a `JSON` column whose stored value is a
JSON *array* fails because the DuckDB driver hands back `[]interface{}`
instead of `[]byte`.

This session also confirmed the existence and shape of
`scripts/pg_to_duckdb.sql` for migrating the live Postgres database
into a fresh DuckDB file (no code changes there — informational).

---

## Bug 3 — `scan` failure on JSON-array columns under DuckDB

### Symptom

```
Error obtaining list of menus -> Error scanning menu def row
  error="sql: Scan error on column index 8, name \"items\":
         unsupported Scan, storing driver.Value type []interface {}
         into type *[]uint8"
function=resource/menu.queryMenus -> church/model.QueryMenuDefs
location=menu/queries.go:74 -> model/menu_def_dao.go:83
```

### Root cause

The `menu_defs.items` column is typed `JSON` in `schema_duckdb.sql`.
go-duckdb v2 *parses* JSON values on the way out and surfaces them as
native Go types — `[]interface{}` for arrays, `map[string]interface{}`
for objects. The Go field is `MenuDef.Items []byte` (kept that way so
the JSON shape stays opaque to the model layer), and database/sql has
no conversion from `[]any` → `[]byte`.

`lib/pq` doesn't have this issue because PG's jsonb wire format is
already text → `[]byte`. So the bug only appears under DuckDB.

The same hazard exists for any JSON column we read into a `[]byte`
field: `users.prefs`, `pages.data`, `menu_defs.items`. (Note:
`charges.meta` is already `VARCHAR` in the schema and is unaffected.)

### Fix

Cast the column to `VARCHAR` in the SELECT column lists. Both DuckDB
and Postgres support `CAST(<col> AS VARCHAR)` and both return the
canonical JSON text — which scans cleanly into `[]byte`.

Files touched:

| File                    | Constant         | Column    |
|-------------------------|------------------|-----------|
| `model/user.go`         | `userColumns`    | `prefs`   |
| `model/page.go`         | `pageColumns`    | `data`    |
| `model/menu_def.go`     | `menuDefColumns` | `items`   |

Example (`model/menu_def.go`):

```go
// items is selected as VARCHAR so the row scans cleanly into []byte
// under both backends. DuckDB's JSON reader returns []any when the
// stored value is a JSON array (it parses on the way out); casting
// to VARCHAR forces the textual representation, which is what the Go
// model carries. Postgres jsonb→text round-trips without loss.
const menuDefColumns = `id, created_at, updated_at, updated_by, title, slug, published, is_admin, CAST(items AS VARCHAR) AS items`
```

The same comment on `menuDefColumns` is referenced from the other two
constants to avoid duplicating the rationale.

### Why this is the right level for the fix

Considered three options:

1. **Cast in SELECT (chosen)** — minimal, dialect-portable, no Go-side
   API changes. Keeps the model field type stable.
2. **Custom Scanner type** — symmetric with `StringSlice`, but a much
   wider blast radius (every caller of `Items`/`Prefs`/`Data` would see
   the new typed wrapper, even when the existing semantics are fine).
3. **Schema change to VARCHAR** — would also work but loses the option
   to use DuckDB's JSON functions later, and requires a migration step
   on existing files.

Option 1 is the smallest change that closes the bug today without
foreclosing on future use of the JSON type. If we ever want
DuckDB-side JSON ops, we can revisit by introducing a Scanner wrapper
on the read side.

### Writes are unaffected

INSERT and UPDATE statements have their own explicit column lists
(they don't go through `*Columns` constants) and continue to bind the
raw bytes via the `jsonArg(...)` NULL guard added earlier in the
session. Round-trip remains: write `[]byte` → store JSON → read text
back as `[]byte`.

---

## Reference: PG → DuckDB data migration

`scripts/pg_to_duckdb.sql` exists and was reviewed during this
session. Highlights:

- DuckDB CLI script (not psql). Loads the `postgres` extension and
  `ATTACH`es the live PG instance read-only.
- `INSERT … SELECT` on every table with explicit column lists. Copies
  `id` verbatim so existing references survive.
- Bumps each `*_id_seq` past `MAX(id)` after the copy so subsequent
  app inserts don't collide.
- The legacy `images` table is intentionally skipped (no DAO
  consumers — image handling lives in `resource/chimage`).

Run order:

```bash
# 1. Initialise schema (the app does this on first open, or manually):
duckdb church.duckdb < db/schema_duckdb.sql

# 2. Edit the DSN on line 33 of pg_to_duckdb.sql, then:
duckdb church.duckdb < scripts/pg_to_duckdb.sql
```

Caveats: the DSN is hardcoded (credentials redacted here:
`postgresql://devuser:<REDACTED>@localhost/church_development`); the
script is one-shot, not idempotent — re-running on a non-empty DuckDB
will fail on PK conflicts.

---

## Files changed in this session

```
M  model/user.go             (CAST prefs AS VARCHAR)
M  model/page.go             (CAST data AS VARCHAR)
M  model/menu_def.go         (CAST items AS VARCHAR)
```

## Build status

`go build ./...` clean.

## Open follow-ups

- Verify ccgrand admin pages render now: list menus, list pages, view
  user prefs. The cast fix should resolve all three; if any new
  `[]any` errors appear, look for additional `JSON` columns added
  outside the three constants above.
- If new JSON columns are added to the schema, remember to: (a) use
  `jsonArg(...)` on bind, and (b) include `CAST(<col> AS VARCHAR) AS
  <col>` in the column constant. Worth a one-liner in CLAUDE.md if
  this shape recurs.