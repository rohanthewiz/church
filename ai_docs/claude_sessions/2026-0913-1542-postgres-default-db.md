# Session: Make Postgres the default DB backend

**Session:** https://claude.ai/code/session_01NQNwAU3z6vZ7GgJDWAPfjE
**Date:** 2026-09-13
**Continues:** `2026-0912-2125-admin-save-failures-flash.md`

## What happened

The user asked to make Postgres the default database. Before this session, an
empty `db.type` selected embedded bytdb and Postgres had to be chosen
explicitly. That is now reversed. An empty type means Postgres, and bytdb is
opt-in with `db.type: bytdb` or `DB_TYPE=bytdb`.

Committed and pushed in three repos:

| Repo | Branch | Commit |
|---|---|---|
| church | `master` | `ede3b32` Make Postgres the default DB backend; bytdb is now opt-in |
| cema | `feature/site-themes` | `12049cf` Load pg: settings for the default (empty) DB type |
| ccswm | `master` | `37a3052` Load pg: settings for the default (empty) DB type |

## Changes

- **`db/connect.go`**
  - `InitDB` resolves an empty `DBType` to `DBTypes.Postgres` up front. Retries
    from `Db()` then see a concrete driver name, and `sql.Open("")` can't hit an
    "unknown driver" error.
  - `openDB` takes the embedded path only when `DBType == DBTypes.BytDB`.
  - The `DBTypes` comment now describes the new default.
- **`cema/main.go`, `ccswm/main.go`:** the `pg:` block used to be applied only
  when `DBType == postgres`. It is now applied when `DBType != bytdb`. Without
  this, the empty default would have reached Postgres with no host or
  credentials.
- **`test_scripts/bytdb_wire_check/main.go`:** it relied on "empty means bytdb"
  and now passes `DBType: db.DBTypes.BytDB`.
- **New `db/connect_default_test.go`** (`TestInitDBDefaultsToPostgres`)
  - Checks that `InitDB` with an empty type resolves to postgres, leaves
    `bytdbEngine` nil, and leaves `BytDBWireAddr()` empty.
  - Needs no server, because lib/pq's `sql.Open` does not dial.
  - Saves and restores the package globals, the same pattern as
    `replicate_integration_test.go`.
- **Comments only:**
  - `config/config.go`: the `DB` struct doc and the `Type` field.
  - `config/env_overrides.go`: the DB_TYPE note.
  - `deploy/k8s/sites/{cema,ccswm}.yaml`: the old "bytdb is already the default
    type" now reads "DB_TYPE is required: Postgres is the default".

## Impact

- **k8s deploys:** unchanged. Both site manifests already pin
  `DB_TYPE=bytdb`, and that env var is now **required**. Removing it would
  switch the site to Postgres.
- **Local dev:** `cema/cfg/options.yml` has no `db:` block, so a local cema now
  boots on **Postgres**. Use the `pg:` block for the environment, or add
  `db: {type: bytdb}` to stay on bytdb.
- **How to tell which backend is running:** the log line
  `bytdb serving embedded database` appears only on bytdb.
- **Other callers were already explicit** and are unaffected:
  - bytdb: `pg_to_bytdb`, `bytdb_to_pg`, `replicate_integration_test`.
  - Postgres: `auth_live_check`, `recurrence_live_check`, `sermon/import2`.

## Verification

- `go build ./...` in church, plus builds of cema and ccswm: OK.
- `go vet` on `./db`, `./config` and `./test_scripts/bytdb_wire_check`: OK.
- `go test ./db/... ./config/... ./resource/dbbackup/...`: pass, including
  the new test.
- `go run ./test_scripts/bytdb_wire_check`: all checks pass with bytdb set
  explicitly.
- **Not done:** booting a real site on the new default against a live Postgres.

## Environment notes

- gopls reports `go.work requires go >= 1.26.1 (running go 1.25.4)`. The editor
  is using an older Go toolchain than the shell. This is not a code problem:
  builds and tests from the shell work.
- `.cats-todo/` (untracked, in church and ccswm) was deliberately not committed.

## Next

1. **New:** cema's `pg:` fix (`12049cf`) is only on `feature/site-themes`. Merge
   it to cema's main branch before building cema from there. Otherwise an empty
   `db.type` reaches Postgres with no host or credentials.
2. **New:** boot cema locally with no `db.type` against a Postgres DB. Confirm
   there is no `bytdb serving` log line and that pages render. Recipe is in
   `2026-0912-1718-…`; drop the `DB_TYPE=postgres` from it, since that is now
   the default.
3. **New:** document the `db:` block (`type: postgres|bytdb`, `file`, `listen`)
   in `cema/cfg/options-sample.yml` and `ccswm/cfg/options-sample.yml`. Neither
   has one, so the default is invisible from config.
4. **New:** update docs that still describe bytdb as the default:
   - `deploy/k8s/README.md` §migration step 6: the rollback is now removing or
     changing `DB_TYPE` in the manifest, not `db.type: postgres`.
   - `ai_docs/fable_bytdb_k8s_readiness.md`.
5. Start Docker and run `./deploy/deploy.sh images`, the one unproven fix from
   2026-08-01. If bimg/libvips 8.15 causes trouble, pin an older Alpine or
   replace bimg with a pure-Go resizer (restores `CGO_ENABLED=0` / static image).
6. Provision LKE + Object Storage and fill `deploy/backup.env`. Then run
   `./deploy/deploy.sh preflight infra`, point DNS to the NodeBalancer IP, and
   run `./deploy/deploy.sh base seeds secrets images sites verify`.
7. Create `ccswm/cfg/options.yml` from the sample (cema already has one). It now
   needs a real `pg:` block, or `db.type: bytdb`.
8. Move `resource/chimage` uploads onto IDrive e2 beside sermon media, retiring
   the `dist/img` volume mount.
9. From the readiness doc:
   - Call `CloseDB()` on SIGTERM in `ServeRWeb`.
   - Migrate `resource/dbbackup` off aws-sdk-go-v2 onto `replicate/s3`.
10. **Optional hardening:**
    - `imagePullSecrets`, if the ghcr packages go private.
    - A www→apex redirect.
11. Live-check the 2026-09-12 flash changes on a running site (recipe in
    `2026-0912-1718-…`):
    - Event with lat 300, only one coordinate, or an expired csrf: back on the
      form with a flash, and no new row.
    - Menu/page post with empty `items`/`modules`: form flash.
    - Successful event edit after a refused save: returns to the list (referrer
      guard).
    - Fresh DB without a superadmin: `token.txt` is `-rw-------`, and the boot
      log shows no seeds.
12. Live-check the admin save flashes from `2026-0912-2125-…`:
    - Expired csrf on article/sermon/user forms and on sermon cleanup: form with
      a warn flash.
    - Form with the reason shown for:
      - a blank article title
      - a bad sermon date, with an audio file chosen on an existing sermon (old
        audio must stay intact)
      - a password mismatch
    - Stopped DB: form with a generic error.
13. A duplicate title on a new article/sermon (unique slug) is the likeliest
    real failure, and it still shows a generic "Error saving".
    - Pre-check the slug on create and refuse it as an InputError.
    - The not-found check has to work on both the Postgres and bytdb executors.
14. Seeds hygiene:
    - Replace the `resource/*/cfg/random_seeds.txt` fixtures with dummy seeds.
    - Consider rotating the live `cfg/random_seeds.txt`, since it is in git
      history.
15. **Optional:** sermon upload file handling.
    - A copy failure leaves a partial local file.
    - A failed DB save leaves an orphan file.
    - A same-name re-upload truncates the existing file before the save is known
      to succeed.
    - Fix: write to a temp file and rename after a successful upsert.
16. Keep typed values on a refused save.
    - Re-render the form from the posted presenter instead of redirecting. This
      applies to events, articles, sermons and users.
    - And/or wrap `UpsertEvent`'s three table writes in a transaction.
17. **Postgres coverage still missing** (more important now that it is the
    default):
    - Not yet run on Postgres: sermon create/import + audio, Stripe payment
      intent/history/webhook, `/chat/stream` SSE, user create/delete, image
      upload.
    - Optionally make `test_scripts/bytdb_wire_check` accept a Postgres DSN, so
      the same checks prove both backends.
18. Apply the pending `event_locations` goose migration to `church_development`.
    This is needed before local dev runs on the new Postgres default.
19. Add `bytdb_to_pg` (and the `pg_to_bytdb` date fix), plus the Postgres smoke
    result, to `ai_docs/fable_bytdb_k8s_readiness.md` §7.
20. **Optional:** per-table content checksum in `bytdb_to_pg` beyond row counts.
21. **Non-goal (declined):** a `-truncate` flag on `bytdb_to_pg`. Refusing
    non-empty destinations protects live Postgres sites.
22. **Non-goal (no action):** page `opts.item_ids` is saved as `[]` instead of
    `null` after an admin page save. It is Go nil-vs-empty-slice JSON: harmless,
    and identical on both backends.
23. **Non-goal (no action):** GET form/list/show handlers in these controllers
    still return errors. The page definitions are static and only fail on a
    code bug.
