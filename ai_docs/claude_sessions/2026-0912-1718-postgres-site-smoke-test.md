# Session: Postgres site smoke test (cema on `DB_TYPE=postgres`)

**Session:** https://claude.ai/code/session_01XmyuJDjLKSFLe5DSTmjVH6
**Date:** 2026-09-12
**Continues:** `2026-0912-1655-bytdb-to-pg-migration-tool.md` (same session)

## What happened

Picked up Next item 8 from the previous doc: smoke-test a site booted on
Postgres. cema was run against a throwaway copy of the dev database and driven
over HTTP through the public site, the mobile API, and the admin UI.
**No Postgres-specific defects found.** No code changes were made; this doc is
the only commit.

Correction to the previous doc: "nothing has run on Postgres since 2026-07-19"
was too strong. The 2026-08-01 emulator smoke (`2026-0801-1149-…`) ran cema on
Postgres for chat + prayer wall. What had not run on Postgres was the work after
that: event locations, app-config location, the admin UI overhaul, Stripe
hardening.

## Recipe (reusable)

- **DB:** `createdb -O <db-user> church_pg_smoke`, then `pg_dump church_development
  --no-owner | psql church_pg_smoke`, then `goose … up`. That also applied the
  `event_locations` migration still pending on dev. `church_development` was only
  read.
- **Admin user:** dev has no role-1 user. Password hashing is
  `scrypt(password, stored_salt)` (`resource/auth/crypt_scrypt.go`), independent
  of `cfg/random_seeds.txt` and of username. So a role-1 row was inserted by
  copying the role-7 test user's `encrypted_password` + `encrypted_salt`.
  cema's and church's `random_seeds.txt` differ, which is irrelevant for login.
- **Binary:** `go build -o <scratch>/cema` from `~/projs/go/church/cema`.
- **Run dir (scratch):** copy of `cema/cfg` (options.yml + random_seeds.txt), with
  `sed` to `port: 8090` and `database: church_pg_smoke` (both the defaults and the
  production `pg:` blocks match the pattern). Symlinks `dist`, `sermons`. Then
  `APP_ENV=development DB_TYPE=postgres ./cema`.
- **Confirming the backend:** the log has **no** `bytdb serving embedded
  database` line. Slack `not_authed` log errors are expected locally.
- **A cema was already serving :8088 (PID 16607)**; left untouched.

## HTTP driving notes (gotchas)

- **CSRF, two forms:** edit/new forms carry `<input type="hidden" name="csrf"
  value="…">`. Admin **list** pages carry the delete token as `data-csrf="…"` on
  the grid wrapper (`grid/grid.go` `CSRFToken`). Tokens live in the in-process
  kvstore for 1h and are not single-use.
- **Menu and page forms** submit `items` / `modules` as empty hidden inputs that
  `preSubmit()` JS fills: `{"items":[{label,url,parent_menu_slug,sub_menu_slug}]}`
  (`resource/menu/form_objects.go`) and `{"mods":[{title,module_type,main_module,
  published,layout_column,items_url_path,item_ids,item_slug,limit,offset,
  show_unpublished,ascending,custom_class}]}` with limit/offset as strings
  (`page/form_objects.go`). A plain re-post gets "No items/modules received" → 500.
- **Event form:** `event_date` `YYYY-MM-DD`, `event_time` `HH:MM` (parsed with
  the server zone, `config.IncomingDateTimeFormat`), `recur_freq` radio
  `""|weekly|monthly`, `recur_weekday` 0–6, `recur_week` 1–4/-1,
  `event_latitude`/`event_longitude`. Checkboxes send `on`.
- **API:** login body `{username,password,device}` → `{token,expires_at,user}`.
  Chat POST → `{"message":{id…}}`. Prayer POST → `{"prayer_request":{id…}}`.
  Answered body `{"note":…}`.
- **Shell (zsh) pitfalls hit this session:** `?` in unquoted URLs globs.
  `$PSQL="psql -h …"` is not word-split. **`local path=` clobbers zsh's `$path`
  (tied to PATH)** → `curl: command not found`. Drive multi-step scripts with
  `bash <<'EOF'`.

## Results (all against Postgres)

| Area | Checks | Result |
|---|---|---|
| Public web | `/`, `/pages/home`, `/articles(/1)`, `/sermons(/1)`, `/events`, `/calendar`, `/prayer-wall` | all 200 |
| Read API | app-config, sermons, articles, events window, feed, chat list, prayer list | all 200 |
| Sermon search (`ILIKE`, `array_to_string`) | `teacher=pastor` → 2; `ref=rom 15` → Walking in Hope; `ref=john 3` → Living by Faith | correct |
| API auth (`api_tokens`) | login ×2, `/auth/me`, logout, logout-all → token 401 | pass; 0 tokens left |
| Chat | POST (RETURNING id) 201, list, keep by editor 200 (DB `keep=t`), keep by role-9 403, delete 200 | pass |
| Prayer wall | POST 201, answered by editor 200, withdraw by owner 200 | pass; row gone |
| Admin login | web form + CSRF → 303, dashboard | pass |
| Admin renders (SQLBoiler reads) | 17 list/new/edit pages: users, articles, sermons, events, pages, menus | all 200 |
| Articles | create (`{smoke,pg}`, published) → API 200; update unpublish → API 404; delete via grid token | pass |
| Event + location + weekly recurrence | DB rows; API `location.configured`, "Every Sunday"; Oct–Nov window → 9 Sundays all located; web detail/list/calendar render | pass |
| Event updates | move point + drop recurrence; lat 300 refused; one coordinate refused (app validation, not DB CHECK); clear both → location row removed; monthly last-Sunday → Oct 4 (base), Oct 25, Nov 29, Dec 27 | pass |
| Event delete | FK `ON DELETE CASCADE` | events/locations/recurrences all 0 |
| Menu 1 save | JS-equivalent `items` JSON | 303; title changed; `items` jsonb identical |
| Page 1 save | JS-equivalent `mods` JSON | 303; title changed; home modules render; only diff `opts.item_ids` `null → []` |
| User 3 save | edit form re-post | 303; password/role/email unchanged |
| Server log | whole run | no `pq:` errors, no panics; only intentionally triggered errors |

**Not exercised:** sermon create/import and audio, Stripe payments, `/chat/stream`
SSE (on PG this session), user create/delete, image uploads, FTP, the backup
endpoint's bytdb-only refusal (it short-circuits earlier with "Backup is not
configured").

## Cleanup

Stopped only the :8090 cema (PID 85203). Dropped `church_pg_smoke`. Deleted the
scratch cookie jars, temp HTML/JSON, the copied `cfg/` (it held the DB password),
and the `token.txt` that `AuthBootstrap` wrote at startup. `church_development`
and the :8088 cema are unchanged.

## Environment/API notes

- Postgres 16 via Homebrew: `/opt/homebrew/opt/postgresql@16/bin`, socket `/tmp`.
- `token.txt` from `AuthBootstrap` landed on disk as `rwxr-xr-x`
  (`os.ModePerm` minus umask): world-readable and executable.
- `.cats-todo/` (personal todo list, untracked) deliberately not committed.

## Next

1. Start Docker and run `./deploy/deploy.sh images`, the one unproven fix from
   2026-08-01. If bimg/libvips 8.15 fights: pin older Alpine or replace bimg with
   a pure-Go resizer (restores `CGO_ENABLED=0` / static image).
2. Provision LKE + Object Storage, fill `deploy/backup.env`, then
   `./deploy/deploy.sh preflight infra` → DNS to NodeBalancer IP →
   `./deploy/deploy.sh base seeds secrets images sites verify`.
3. Create `ccswm/cfg/options.yml` from the sample (cema already has one).
4. Move `resource/chimage` uploads onto IDrive e2 beside sermon media, retiring
   the `dist/img` volume mount.
5. From the readiness doc: SIGTERM → `CloseDB()` in `ServeRWeb`; migrate
   `resource/dbbackup` off aws-sdk-go-v2 onto `replicate/s3`.
6. Optional hardening: `imagePullSecrets` if ghcr packages go private; www→apex
   redirect.
7. `resource/auth` issues: `init()` prints the first 50 crypto seeds to stdout
   on every boot (pod logs), and `AuthBootstrap` writes `token.txt` with
   `os.ModePerm`. Observed on disk this session as `rwxr-xr-x`, so any local user
   can read it. Use 0600.
8. **New:** `UpsertEventRWeb` returns a raw error, so the user gets a bare HTTP
   500 page, for validation failures ("latitude must be between -90 and 90",
   "needs both a latitude and a longitude") and for an expired CSRF token.
   Articles, menus, and deletes redirect with a flash. Same bare-500 pattern for
   menu/page "No items/modules received". Switch to `app.RedirectRWeb`/flash back
   to the form.
9. **Postgres coverage still missing:** sermon create/import + audio, Stripe
   payment intent/history/webhook, `/chat/stream` SSE, user create/delete, image
   upload. Optionally make `test_scripts/bytdb_wire_check` accept a Postgres DSN
   so the same 35 checks prove both backends (carried from previous item 8; the
   HTTP smoke part is done).
10. Apply the pending `event_locations` goose migration to `church_development`.
11. Add `bytdb_to_pg` (and the `pg_to_bytdb` date fix) plus this Postgres smoke
    result to `ai_docs/fable_bytdb_k8s_readiness.md` §7.
12. **Optional:** per-table content checksum in `bytdb_to_pg` beyond row counts.
13. **Non-goal (declined):** a `-truncate` flag on `bytdb_to_pg`. Refusing
    non-empty destinations protects live Postgres sites.
14. **Non-goal (no action):** page `opts.item_ids` saved as `[]` instead of
    `null` after an admin page save. It's Go nil-vs-empty-slice JSON, harmless,
    identical on both backends.
