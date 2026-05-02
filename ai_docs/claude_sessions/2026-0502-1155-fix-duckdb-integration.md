# Fix DuckDB Integration

**Date:** 2026-05-02 11:55
**Session ID:** 2bc6f4f9-434c-4f54-a4ec-6a1be03e2986
**Branch:** roh/drop-sqlboiler

## Context

`ccgrand` is a downstream consumer of the `church` framework. After the
sqlboiler → hand-written DAO migration and the Postgres → DuckDB Phase 2
work, starting `ccgrand` exposed two latent bugs in the new DuckDB code
path. This session fixed both.

---

## Bug 1 — DuckDB schema bootstrap failed with "empty query"

### Symptom

```
Could not setup database -> Error initializing database
  -> Failed to bootstrap DuckDB schema
  -> Failed executing DuckDB schema statement
  error="empty query"
  stmt="-- schema_duckdb.sql\n--\n-- Single-file schema for a fresh DuckDB
        install of the church platform.\n..."
location=ccgrand/main.go:41 -> db/connect.go:55 -> db/connect.go:112
         -> db/connect.go:133
```

### Root cause

`bootstrapDuckDB` in `db/connect.go` split the embedded
`schema_duckdb.sql` on `;` and executed each chunk. The schema's header
comment block contained a `;` *inside* a `--` line comment:

```sql
--   * BIGSERIAL           -> BIGINT PRIMARY KEY DEFAULT nextval('<table>_id_seq')
--                            (DuckDB has no SERIAL; we emit one sequence per table.)
```

Naive split-on-semicolon produced a first chunk that was nothing but
comment text — DuckDB rejects that as "empty query".

### Fix

Strip `-- ...` to end-of-line on every line *before* splitting on `;`.
The fix is in `db/connect.go`:

- New `stripLineComments(src string) string` helper.
- `bootstrapDuckDB` now ranges over
  `strings.SplitSeq(stripLineComments(duckdbSchema), ";")`.

Notes:
- We deliberately do not try to be clever about `--` inside string
  literals — the schema file has none, and quote tracking would only
  add complexity.
- Both range loops were promoted to `strings.SplitSeq` per the lint
  diagnostic (Go modernize/splitseq).

### Why this couldn't be caught earlier

The Phase 2 migration tests almost certainly used a schema variant
without the offending comment, or didn't exercise the bootstrap path
on a fresh DB. Worth keeping an eye on: future edits to the schema
header could re-introduce the same shape if the helper is removed.

---

## Bug 2 — Bootstrap superadmin insert failed with "Malformed JSON"

### Symptom

```
Failed to insert user into db -> Error inserting user
  error="Conversion Error: Malformed JSON at byte 0 of input:
         input length is 0.  Input: \"\""
function=resource/user.SaveUser -> church/model.InsertUser
location=user/user.go:32 -> model/user_dao.go:135
```

Followed by:

```
Bootstrap: failed to create superadmin
function=church/admin.bootstrapSuperAdmin
location=admin/bootstrap.go:71
```

### Root cause

The `users.prefs` column is typed `JSON` in `schema_duckdb.sql`. The
Go-side field is `User.Prefs []byte` (left nil during the bootstrap
path — see `resource/user/user.go:21` `SaveUser`). The go-duckdb
driver serialises a `nil` / empty `[]byte` as the literal `""`, and
DuckDB's JSON parser rejects empty string as malformed.

`lib/pq` (Postgres) already maps `nil []byte` to a real SQL `NULL`,
so this only surfaced under DuckDB.

### Fix

Added a small helper in `model/types.go`:

```go
// jsonArg normalises a []byte payload destined for a JSON/JSONB column.
// Why: the DuckDB driver serialises a nil/empty []byte as the literal
// "" and DuckDB's JSON column rejects "" as malformed JSON. Returning an
// untyped nil makes database/sql send a real SQL NULL instead. Postgres
// jsonb behaves the same way ("" is not valid JSON), so this guard is
// also the right shape under lib/pq — for nil input both drivers already
// produce NULL; the only material difference is that an *empty* []byte
// also collapses to NULL rather than erroring out.
func jsonArg(p []byte) any {
    if len(p) == 0 {
        return nil
    }
    return p
}
```

Applied at all six call sites that bind a `[]byte` into a JSON/JSONB
column:

| File                 | Functions touched         | Field   |
|----------------------|---------------------------|---------|
| `model/user_dao.go`     | `InsertUser`, `UpdateUser`     | `Prefs` |
| `model/page_dao.go`     | `InsertPage`, `UpdatePage`     | `Data`  |
| `model/menu_def_dao.go` | `InsertMenuDef`, `UpdateMenuDef` | `Items` |

### Cross-backend safety

Confirmed dialect-neutral:

- **Postgres `jsonb`** also rejects `""` as invalid JSON. lib/pq already
  maps `nil []byte` → `NULL`, so the only behavioural change vs. before
  is that an *empty* (non-nil) `[]byte` now becomes `NULL` instead of
  raising an insert error. That is the more useful default.
- **DuckDB `JSON`** now sees a real `NULL` when the Go field is nil or
  empty, which the column allows.

---

## Files changed

```
M  db/connect.go             (stripLineComments + SplitSeq)
M  model/types.go            (jsonArg helper)
M  model/user_dao.go         (Prefs guard)
M  model/page_dao.go         (Data guard)
M  model/menu_def_dao.go     (Items guard)
```

(Plus pre-existing modified files from the start of the session:
`config/config.go`, `db/connect.go`.)

## Build status

`go build ./...` clean in both `church/` and `ccgrand/` after each fix.

## Open follow-ups

- None blocking. ccgrand should now boot cleanly against a fresh DuckDB
  file and create its bootstrap superadmin.
- Worth a sanity check: any *future* JSON/JSONB columns added to the
  schema must remember to use `jsonArg(...)` at the bind site, or
  re-introduce the same nil-vs-empty hazard. Consider a `golangci-lint`
  custom analyzer or a code comment on the column definitions if this
  shape recurs.
- The stripLineComments approach is intentionally simple. If the schema
  ever grows string literals containing `--`, the helper will need a
  proper tokenizer pass.