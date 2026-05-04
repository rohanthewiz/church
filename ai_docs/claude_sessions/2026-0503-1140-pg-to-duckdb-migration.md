# pg-to-duckdb migration endpoint

**Date:** 2026-05-03 11:40
**Session ID:** c3748eac-a6ea-420c-a7ad-c7027c2c82c3

## Goal

Convert the existing one-shot Postgres → DuckDB data migration from a
`duckdb` CLI script into an in-process HTTP endpoint that drives the
load through the running app's go-duckdb driver. The cut-over must be
runnable exactly once against any given DuckDB file, with retries
allowed only after a partial failure.

## Starting state

- `scripts/pg_to_duckdb.sql` was a CLI-oriented script that used
  DuckDB's `postgres` extension (`INSTALL postgres; LOAD postgres;
  ATTACH '<dsn>' AS pg ...`) to copy each table 1:1 into a freshly
  bootstrapped DuckDB file, then bumped sequences past `MAX(id)`.
- `db/connect.go` already executed the embedded `schema_duckdb.sql`
  statement-by-statement on every DuckDB open, with private helpers
  `bootstrapDuckDB` and `stripLineComments`. The split-on-`;` /
  strip-line-comments approach was needed because go-duckdb's
  `database/sql.Exec` only runs the first statement of a multi-stmt
  string.
- Routing for the project lives in `router_rweb.go`. A precedent
  existed for token-gated bootstrap endpoints:
  `SetupSuperAdminRWeb` mounted at `GET /super` and gated by
  `admin.SuperToken` (a randomly-generated token written to
  `token.txt` only when no superadmin exists yet).

## Design decisions

1. **Authorisation: reuse `admin.SuperToken`.**
   The fresh-cut-over moment is exactly when no superadmin exists, so
   the token is naturally available right when needed and gone
   afterwards. Same shape as `SetupSuperAdminRWeb`. Trade-off
   acknowledged: once `CreateSuperUser` clears the token, retriggering
   the endpoint requires re-bootstrapping — which is fine because the
   marker row also blocks a second run.

2. **DSN delivered as a query parameter (`pg_dsn`) rather than
   embedded.** Lets the cut-over happen without redeploying. The DSN
   contains a Postgres password and rweb's verbose access log will
   capture the full URL — acceptable for a local one-shot, called out
   in the handler comment so it's not a silent foot-gun.

3. **One-shot guarantee via marker row in DuckDB itself.**
   New `migration_state(name PRIMARY KEY, completed_at)` table in
   `schema_duckdb.sql`. Probed by the handler before running; written
   by the *very last* statement of the migration script. Layered
   defence: handler check + script-level INSERT + the placement of
   that INSERT *after* every other statement so partial failure leaves
   the marker absent and a retry is allowed.

4. **TRUNCATE before INSERT.** Retries from a clean slate without
   manual cleanup. Safe because the handler-level marker check
   prevents this from ever clobbering live data after a successful
   run.

5. **`{{PG_DSN}}` placeholder substitution at runtime.** Kept as a
   named constant in the handler so a stale embedded script (placeholder
   accidentally removed) fails loudly with a clear error rather than
   silently shipping a stub DSN to DuckDB.

6. **Embed location.** Go's `//go:embed` cannot reach upward, so
   embedding `scripts/pg_to_duckdb.sql` directly from
   `admin_controller/` is impossible. Created a tiny `scripts`
   package (`scripts/scripts.go`) that embeds and exports
   `scripts.PgToDuckDB`. Keeps the SQL file at its original
   discoverable location for CLI users (with a `sed` invocation
   documented at the top of the file) while still letting the handler
   ship it inside the binary.

7. **No transaction wrapping.** ATTACH/DETACH for the postgres
   extension manage connection-level state and don't slot cleanly into
   a `BEGIN ... COMMIT`. Marker-row check + truncate-on-retry already
   yields the desired "done once, retryable on failure" semantics
   without the added complexity.

## Changes

| File | Change |
| --- | --- |
| `db/schema_duckdb.sql` | Added `migration_state` table |
| `db/connect.go` | Extracted `ExecScript` and `StripLineComments` as exported helpers; `bootstrapDuckDB` now delegates to `ExecScript` |
| `scripts/pg_to_duckdb.sql` | Added per-table `TRUNCATE`s, replaced literal DSN with `{{PG_DSN}}` placeholder, added final `INSERT INTO migration_state` marker row |
| `scripts/scripts.go` | New package; embeds `pg_to_duckdb.sql` as `scripts.PgToDuckDB` |
| `admin_controller/migrate_pg_rweb.go` | New handler `MigratePgToDuckDBRWeb` — token check, marker check, placeholder substitution, `db.ExecScript` |
| `router_rweb.go` | Registered `GET /super/pg-to-duckdb` |

## Endpoint contract

```
GET /super/pg-to-duckdb?token=<SuperToken>&pg_dsn=<postgres-uri>
```

Example (password redacted):

```
GET /super/pg-to-duckdb?token=<TOKEN>&pg_dsn=postgresql://devuser:<REDACTED>@localhost/church_development
```

Responses:

- `Migration complete.` on success.
- `ye shalt not pass` — bad / missing token.
- `pg_dsn query parameter is required` — missing DSN.
- `pg_to_duckdb migration has already run; refusing to repeat. Delete
  the row from migration_state to force a re-run.` — marker row
  present.

## To force a re-run during development

```sql
DELETE FROM migration_state WHERE name = 'pg_to_duckdb';
```

…then hit the endpoint again. (Re-bootstrapping `SuperToken` may also
be required if a superadmin already exists — easiest is to delete all
superadmin rows from `users` *and* the marker row, then restart the
app so `AuthBootstrap` re-issues a `SuperToken`.)

## Verification

`go build ./...` — clean.

No runtime test against a live Postgres source executed in this
session; that should be the next step before relying on the endpoint
in anger.

## Open follow-ups

- Consider adding a JSON response with row counts per table so the
  caller can sanity-check the load.
- Consider scrubbing the DSN from rweb's verbose log if this endpoint
  is ever exposed beyond a local cut-over.
- The handler comment notes the script and handler share the literal
  `'pg_to_duckdb'` — if more migrations are added, factor that into a
  shared constant referenced from both sides.
