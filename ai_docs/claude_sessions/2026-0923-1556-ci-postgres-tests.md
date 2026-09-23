# CI runs the Postgres half of the tests (N-054)

Session: `f75efb62-4d89-4033-bc98-45e3d905d48a`

The user asked for a recommendation from `ai_docs/todo/next-list.md`. N-054
was picked as the one medium-value item a session could finish end to end:
last session made the DB-backed tests run on both backends, but CI ran only
bytdb, and bytdb has already accepted SQL that Postgres refuses (42P08).

## Done

| Item | Commit | What |
|---|---|---|
| N-054 | `80193e2` | `postgres:16` service in `.github/workflows/ci.yml`; Test step sets `CHURCH_TEST_PG_DSN` and `CHURCH_TEST_PG_REQUIRED` |

- **Service user is `devuser`**: the charges migration runs `OWNER TO
  devuser`. As the image's `POSTGRES_USER` it is a superuser, which covers
  the CREATEDB `internal/testdb` needs for its throwaway databases. A
  `pg_isready` health check holds the steps until the server is up.
- **`CHURCH_TEST_PG_REQUIRED`** (new, `internal/testdb.EnvPostgresRequired`):
  when set, a missing DSN makes `OpenPostgres` fail instead of skip. A
  skipped subtest passes, so without it a lost DSN would quietly drop the
  Postgres half while CI stayed green. Unset locally, so local runs behave as
  before.
- Stale comments fixed: `testdb.go` and `admin_routes_smoke_test.go` said CI
  has no Postgres server.

## Verification

- Locally: `go test ./...` green without the DSN, and with the DSN plus
  `CHURCH_TEST_PG_REQUIRED=1` (as CI runs). `CHURCH_TEST_PG_REQUIRED=1`
  alone fails `TestAdminRoutesSmoke/postgres`, as intended.
- CI run `35918667843` on `80193e2`: success. With the required flag set,
  that pass means the Postgres subtests ran rather than skipped.

## Gotchas

- `gh run list --commit <sha>` returned nothing right after the push; a plain
  `gh run list` found the run.
- `psql` isn't on the PATH here (Homebrew `postgresql@16` is installed but not
  linked); the dev DSN is in `dbc.toml`.

## Next

Closed: N-054. Declined: None. Raised: None. Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
