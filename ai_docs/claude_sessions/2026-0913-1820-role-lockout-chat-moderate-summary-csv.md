# Session: Role-manager lockout, SuperAdmin-only /debug, chat.moderate, giving summary CSV, roles smoke as a Go test

**Session:** https://claude.ai/code/session_01NQNwAU3z6vZ7GgJDWAPfjE
**Date:** 2026-09-13
**Continues:** `2026-0913-1747-site-timezone-element-bump.md`

## What happened

The user asked for these items from the previous `## Next`:

| Old # | Item |
|---|---|
| 4 | Re-pin cema's church pin |
| 5 | Re-pin ccswm after the church push |
| 8 | Summary-only giving CSV |
| 13 | Guard against removing the last role manager |
| 14 | Restrict `/debug/*` |
| 15 | Decide on chat/prayer-wall moderation |
| 16 | Convert `roles_smoke` into a Go test |
| 21 | Fix docs that call bytdb the default |
| 30 | Duplicate article/sermon title error |

All are done. Everything is committed and pushed except church_mobile. Nothing
was checked in a browser.

| Repo | Branch | Commit |
|---|---|---|
| church | `master` | `de76881` Role-manager lockout guard, SuperAdmin-only /debug, chat.moderate permission, giving summary CSV; roles smoke becomes a Go test |
| ccswm | `master` | `3d611ee` Re-pin church to de76881 |
| cema | `feature/site-themes` | `9c40ebe` Re-pin church to de76881 so GOWORK=off builds work |
| church_mobile | `roh/use-grmob` | **uncommitted**: `lib/src/models/user.dart`, new `test/user_model_test.dart` |

## Behaviour

### Role-manager lockout (item 13)

A **non-SuperAdmin** is refused any write that would leave **zero** enabled,
non-SuperAdmin accounts holding `roles.update`, when at least one holds it now.

- Checked writes:
  - role edit (`UpsertRoleRWeb`, updates only)
  - role delete
  - user update: its final role set, and disabling via `enabled`
  - user delete
- SuperAdmin actors are exempt, because they are the recovery path.
- A site with no holder today (only SuperAdmin manages roles) is not blocked.
- New accounts and new roles are not checked, since they only add.
- Refusal flash: `authz.RoleManagerLockoutMsg` plus "…was not saved" or
  "Nothing was deleted."

### `/debug/*` is SuperAdmin-only (item 14)

- New decorator `auth_controller.RequireSuper`, which wraps `Require("")`.
- A non-super admin is sent to `/admin/home` with a warning. An anonymous
  visitor goes to `/login`.
- **Decision:** SuperAdmin-only rather than a catalog permission. Debug flips
  process-wide render state for every visitor, which makes it an operator
  tool, and a permission would let anyone with `roles.update` hand it out.

### Chat and prayer-wall moderation (item 15)

**Decision:** keep the legacy rule and add a permission alongside it.

```
legacy users.role editor-or-above (1–7, 99) ──┐
                                               ├─► may moderate   (authz.CanModerate)
a role holding chat.moderate ──────────────────┘
```

- **Why not move to the permission outright:**
  - Existing moderators keep working with no data change. Sites seeded their
    default roles once, before `chat.moderate` existed.
  - Old mobile builds mirror the legacy role rule, so they never show buttons
    the server would refuse.
- **`chat.moderate`** is catalog resource `chat` ("Chat & prayer wall"),
  action `moderate`, with a new "Moderate" matrix column.
  - It is marked `SiteOnly`, so `HasAdminAccess` ignores it. A moderator-only
    member gets no admin area.
- The legacy check runs first and needs no query. Other users pay a
  `LoadActor` lookup. A lookup error denies and is logged, so an unmigrated
  Postgres site keeps exactly the legacy behaviour.
- **Call sites switched to `authz.CanModerate`:**
  - chat web `ListMessagesRWeb` (`me.can_moderate`) and `requireModerator`
  - chat API `requireAPIModerator`
  - prayer wall `viewer`, `MarkAnsweredRWeb` and `DeleteRequestRWeb`
  - prayer wall API answered and delete
  - Delete checks ownership first, which is free.
- Denial text changed from "Editor role required" to "Moderator permission
  required".
- **Mobile contract (additive):** `APIUser.can_moderate` on login and
  `/auth/me`. The contract tests now require the key.
- **church_mobile:** `User.serverCanModerate` (nullable) comes from
  `can_moderate`.
  - `canModerate` prefers it and otherwise falls back to the role rule.
  - `toJson` omits the field when it is unknown.
- **Default roles:** Publisher and Editor now include `chat.moderate`. This
  only affects sites seeding roles for the first time.
- `chat.CanModerate(role)` is kept as a wrapper over `authz.LegacyModerator`.

### Giving summary CSV (item 8)

- `GET /admin/giving/csv/summary[?year=]` requires `charges.read`.
- Columns are `Month,Gifts,Gross,Refunded,Net,Pending`: one row per month shown
  (empty months included), then a `Total` row.
- Formatting matches the per-gift export: BOM, CRLF, plain decimals.
- Filename: `giving-<year>[-ytd]-summary.csv`.
- **Decision:** the summary has a Total row but the per-gift export still
  doesn't. The summary is a finished report of about 12 rows; the per-gift file
  is data to sort and filter.
- The toolbar shows "Export summary CSV" beside "Export CSV" (new `.af-actions`
  CSS).

### Duplicate titles (item 30)

**Not a real failure.** `stringops.SlugWithRandomString` appends a time string
plus a SHA-1 of `UnixNano` and the slug. Articles, sermons and pages all use
it, so repeated titles get distinct slugs. The smoke test now saves two
same-title articles and two same-title sermons and asserts distinct slugs. No
pre-check was added.

## Changes (church `de76881`)

- **`resource/authz/`**
  - **`permissions.go`**
    - `ActModerate`, `ChatModerate`, `Resource.SiteOnly`
    - the `chat` catalog row and the `siteOnlyPerms` index
    - package doc updated
  - **`actor.go`:** `HasAdminAccess` ignores site-only permissions.
  - **`moderation.go` (new):** `LegacyModerator(role)`, and
    `CanModerate(exec, username, legacyRole)`.
  - **`lockout.go` (new)**
    - `PendingChange`, `LocksOutRoleManagers` and `countHolders`
    - Whole-table loaders (single-table SELECTs, portable to both backends):
      `loadAllRolePerms`, `loadAllAssignments`, `loadRoleManagerCandidates`.
    - `RoleManagerLockoutMsg`
  - **`module_role_form.go`:** "Moderate" column.
  - **`seed.go`:** Publisher and Editor get `chat.moderate`.
  - **Tests**
    - `permissions_test.go`: catalog includes `ChatModerate`; site-only admin
      access; `TestLegacyModerator`; `TestCountHolders`.
    - `queries_bytdb_test.go`: `CanModerate` on bytdb (legacy, granted,
      disabled) and eight lockout scenarios, including the "second holder makes
      it safe" case.
- **`auth_controller/auth_middleware_rweb.go`:** `RequireSuper`.
- **`router_rweb.go`**
  - Debug routes moved into the exported `RegisterDebugRoutes(s)`, wrapped in
    `RequireSuper`.
  - New `/admin/giving/csv/summary` route.
- **`role_controller/role_controller_rweb.go`:** lockout check on update and
  delete.
- **`user_controller/user_controller_rweb.go`**
  - `finalRoles` is now computed **before** `UpsertUserID` (moved, not
    changed), so the lockout check sees the final assignment.
  - Lockout check on update and on delete.
- **Chat and prayer wall:** `resource/chat/{chat.go,web_rweb.go,api_rweb.go}`
  and `resource/prayerwall/{module_prayer_wall.go,web_rweb.go,api_rweb.go}`.
  - The prayer wall adds a local `canModerate(username, role)` helper.
- **`resource/apitoken`**
  - `api_rweb.go`: `APIUser.CanModerate`, filled on login and `/auth/me`.
    `/auth/me` falls back to `LegacyModerator` when there is no DB handle.
  - `api_contract_test.go`: `WantKeys` includes `can_moderate`.
- **Giving**
  - `resource/payment/giving_report.go`: `SummaryCSVFilename`,
    `GivingSummaryCSVHeadings`, `WriteGivingSummaryCSV`; the header diagram now
    shows both exports.
  - `giving_report_test.go`: `TestWriteGivingSummaryCSV`.
  - `module_giving_list.go`: two export buttons.
  - `payment_controller/admin_giving_rweb.go`: `AdminGivingSummaryCSVRWeb`;
    both exports share `writeGivingCSV(ctx, writer, filenameFn)`.
  - `template/admin_css.go`: `.af-actions`.
- **Smoke test:** `admin_routes_smoke_test.go` (new, `package church_test`)
  replaces the removed `test_scripts/roles_smoke/main.go`.
  - 41 checks in one ordered test, reported individually. Skipped with
    `-short`.
  - It wires `RegisterAdminRoutes`, `RegisterDebugRoutes` and the chat
    `/messages` + `/keep/:id` routes.
  - New checks: debug refusals, summary CSV, refusals on both CSVs, four
    lockout refusals plus the SuperAdmin exemption, duplicate titles,
    `chat.moderate` pin, `can_moderate` in the widget JSON, and no admin access
    for a moderator.
  - It relies on the committed root `cfg/random_seeds.txt`.
- **Docs**
  - `deploy/k8s/README.md`:
    - The intro says bytdb is opt-in and `DB_TYPE=bytdb` is required.
    - Migration step 6 is corrected: `db.type: postgres` in options.yml does
      nothing on k8s, because the env var wins. Roll back via DNS, or remove
      `DB_TYPE` and supply `pg:`.
  - `ai_docs/fable_bytdb_k8s_readiness.md` §6: a dated "superseded" note, and
    the `config` bullet corrected.

## Site re-pins (items 4, 5)

- **ccswm:** church `a68100c2be20` → `de7688131944`. go.mod/go.sum only.
- **cema** (`feature/site-themes`): church June `96257e048c20` → `de7688131944`.
  `go mod tidy` also:
  - raised `go` 1.24.0 → 1.26.1
  - moved logger to v1.3.0, serr to v1.4.0, and rweb, aws-sdk and
    `golang.org/x/*` forward
  - added bytdb, pgwire and stripe-go v86 (indirect)
  - dropped `roredis` (only a commented-out use), echo and redis (indirect)
- Both sites build with `GOWORK=off` (plus `go vet .`) and in the workspace.
- Commands used (`GOPRIVATE` avoids a proxy lag on a fresh commit):

  ```
  GOWORK=off GOPRIVATE=github.com/rohanthewiz/church GOFLAGS=-mod=mod go get github.com/rohanthewiz/church@<sha>
  GOWORK=off … go mod tidy && go build -o /dev/null . && go vet .
  ```

## Verification

- **church**
  - `go build ./...` passes, and `go test ./...` passes in every package.
  - `go test -v -run TestAdminRoutesSmoke .`: 41/41 pass.
  - `gofmt -l` is clean on every file touched, after fixing three alignment
    and import-order slips of mine.
- **church_mobile**
  - `dart analyze lib/src/models/user.dart` finds no issues.
  - `flutter test test/api_client_test.dart` (16) and
    `test/user_model_test.dart` (3) pass.
- **Not done**
  - a browser look at the role matrix "Moderate" column, the lockout flashes,
    the two export buttons at phone width, or the summary CSV in Excel
  - the new authz queries (`LocksOutRoleManagers`, `CanModerate`) on real
    Postgres; `roles_pg_check` doesn't cover them
  - booting a site

## Gotchas found

- **An Edit with `"﻿"` in Go source wrote a literal BOM character.**
  `go build` fails with "invalid BOM in the middle of the file". Fixed with
  `perl -pi -e 's/\x{EF}\x{BB}\x{BF}/\\uFEFF/g'`. Watch for this with any
  escape in edited Go strings.
- **The first cut of the router refactor was broken.** It pasted a fake end of
  `ServeRWeb` that called a nonexistent `registerPublicRoutes`. It was repaired
  by removing the inserted block and appending `RegisterDebugRoutes` at the end
  of the file. Re-read `router_rweb.go` around line 110 if it ever looks odd.
- **church_mobile has `ios/Flutter/{Debug,Release}.xcconfig` changes and an
  untracked `ios/Podfile`.** They were not present at the start and probably
  came from `flutter test` / pub tooling. They were left untouched.
- **zsh globbing:** `grep --include=*.go` fails with "no matches found". Quote
  it as `--include='*.go'`.
- **No `timeout` binary on macOS** in this shell.
- **Editor noise, unchanged:** gopls "go.work requires go >= 1.26.1 (running go
  1.25.4)".

## Next

1. **New:** commit the church_mobile `can_moderate` change on its branch:
   - `lib/src/models/user.dart`
   - `test/user_model_test.dart`

   Then decide what to do with the `ios/Flutter/*.xcconfig` diffs and
   `ios/Podfile`: keep them, or `git checkout` them if they are tooling noise.
2. **New:** extend `test_scripts/roles_pg_check` to run `LocksOutRoleManagers`
   and `CanModerate` on local Postgres, still rolled back.
3. **New:** browser click-through of this session's UI:
   - "Moderate" column on the role form
   - lockout refusal flashes (role edit, role delete, user untick, disable,
     delete)
   - `/debug/show` as Administrator vs SuperAdmin
   - both giving export buttons at phone width
   - the summary CSV opened in Excel
4. **New, optional:** existing sites whose default roles were already seeded
   lack `chat.moderate` on Publisher and Editor. Add it by hand in Role
   Management if wanted. Legacy base roles still moderate either way.
5. **New:** mobile moderation UI for a permission-only moderator. Check on a
   device or emulator that chat pin/delete and prayer answered controls appear
   once the server sends `can_moderate: true`.
6. **Recurring:** after each church push a site depends on, re-pin ccswm, and
   cema on the branch to be merged. Otherwise their `GOWORK=off` builds lag.
   Docker and workspace builds are unaffected.
7. There is no CI in church. Add one that runs `go test ./...` (which now
   includes the admin routes smoke test) and a `GOWORK=off go build` of each
   site, so a stale church pin or a lost `Require` is caught.
8. Add `TIME_ZONE` to `deploy/k8s/sites/cema.yaml` and `ccswm.yaml` with each
   church's real IANA zone, and set `time_zone` in each site's real
   `cfg/options.yml`. Until then pods cut months (and show event times) in UTC.
9. Boot a site with `time_zone` set (and once with a bad name) to confirm the
   startup log line and the fatal message.
10. **Optional:** switch the giving module's `html.EscapeString(...)` + `.T()`
    to `.TE()` now that every site is on element v0.7.0. Keep
    `html.EscapeString` for attribute values: element only escapes `"` there.
11. Browser click-through of `/admin/giving` on a running site:
    - year navigation limits
    - month anchors
    - the unpaid-charge note
    - phone-width table scrolling
    - CSV download opened in Excel (accented names, formula guard, numeric
      amounts)
12. Check the giving report against real charge data on Postgres (e.g. a copy
    of a site DB). Compare the monthly totals, now also the summary CSV, with
    Stripe's dashboard for one month. Do it with `time_zone` set.
13. Run `goose up` for the roles migration (`20260913160000_CreateRolesTables.sql`)
    on `church_development` and on any Postgres site. Also still pending: the
    `event_locations` migration.
14. Browser click-through of the role screens on a running site:
    - role form (matrix toggles, auto-Read)
    - user form Roles card and locked states
    - flash messages on refusals
    - publish switch disabled for an Editor
15. Filter the nav's Admin submenu by permission. Map known `/admin/…` URLs to
    their read permission in `menu.buildMenu`. DB-stored admin menus currently
    show dead links to unpermitted users.
16. Add "+" / delete visibility by permission to the other admin list modules
    (articles, sermons, events, pages, menus). Handlers already refuse; this is
    UI polish.
17. cema's `pg:` fix (`12049cf`), element bump, `time_zone` sample, and now the
    church re-pin (`9c40ebe`) are only on `feature/site-themes`. Merge to cema's
    main branch before building cema from there.
18. Boot cema locally with no `db.type` against Postgres. Confirm there is no
    `bytdb serving` line and that pages render (recipe in `2026-0912-1718-…`,
    minus `DB_TYPE=postgres`).
19. Document the `db:` block (`type: postgres|bytdb`, `file`, `listen`) in
    `cema/cfg/options-sample.yml` and `ccswm/cfg/options-sample.yml`.
20. Start Docker and run `./deploy/deploy.sh images`, the one unproven fix from
    2026-08-01. If bimg/libvips 8.15 causes trouble, pin an older Alpine or use
    a pure-Go resizer.
21. Provision LKE + Object Storage and fill `deploy/backup.env`. Then run
    `./deploy/deploy.sh preflight infra`, point DNS, and run
    `./deploy/deploy.sh base seeds secrets images sites verify`.
22. Create `ccswm/cfg/options.yml` from the sample. It needs a real `pg:` block
    (or `db.type: bytdb`) and a real `time_zone`.
23. Move `resource/chimage` uploads onto IDrive e2 beside sermon media, retiring
    the `dist/img` volume mount.
24. From the readiness doc:
    - Call `CloseDB()` on SIGTERM in `ServeRWeb`.
    - Migrate `resource/dbbackup` off aws-sdk-go-v2 onto `replicate/s3`.
25. **Optional hardening:** `imagePullSecrets` (if the ghcr packages go private)
    and a www→apex redirect.
26. Live-check the 2026-09-12 flash changes on a running site (recipe in
    `2026-0912-1718-…`):
    - bad event coordinates or an expired csrf
    - empty menu `items` / page `modules`
    - referrer guard after a refused save
    - `token.txt` permissions on a fresh DB
27. Live-check the admin save flashes from `2026-0912-2125-…`:
    - expired csrf
    - blank article title
    - bad sermon date with audio chosen
    - password mismatch
    - stopped DB
28. Seeds hygiene:
    - Replace the `resource/*/cfg/random_seeds.txt` fixtures with dummy seeds,
      including `resource/authz/cfg`.
    - Consider rotating the live `cfg/random_seeds.txt`. It is committed, and
      the root smoke test now depends on it existing, so a dummy replacement
      must stay present.
29. **Optional:** sermon upload file handling (partial file on copy failure,
    orphan on DB failure, truncate-before-save). Fix with a temp file and rename
    after a successful upsert.
30. Keep typed values on a refused save: re-render the form from the posted
    presenter (events, articles, sermons, users) and/or wrap `UpsertEvent`'s
    writes in a transaction.
31. **Postgres coverage still missing:**
    - sermon create/import + audio
    - Stripe intent/history/webhook
    - `/chat/stream` SSE
    - image upload
    - giving report with data (item 12)
    - lockout and moderation queries (item 2)
    - Optionally let `bytdb_wire_check` take a Postgres DSN.
32. Add `bytdb_to_pg` (and the `pg_to_bytdb` date fix), plus the Postgres smoke
    result, to `ai_docs/fable_bytdb_k8s_readiness.md` §7.
33. **Optional:** per-table content checksum in `bytdb_to_pg` beyond row counts.
34. **Non-goal (declined):** a slug pre-check on article/sermon create. Slugs
    already carry a nanosecond-hash suffix, and the smoke test proves duplicate
    titles save.
35. **Non-goal (declined):** a catalog permission for `/debug/*`. It is
    SuperAdmin-only by design.
36. **Non-goal (declined):** moving moderation entirely off the legacy
    `users.role`. The rule is legacy OR `chat.moderate`, for no-migration
    continuity and old mobile builds.
37. **Non-goal (declined):** applying the lockout guard to SuperAdmin actors.
    They are the recovery path.
38. **Non-goal (declined):** a config-threaded `*time.Location` in place of
    replacing `time.Local`. One startup assignment covers every existing and
    future call site.
39. **Non-goal (declined):** reusing `TZ` as the override name. The runtime
    already reads it, and an unset `time_zone` must keep honoring it.
40. **Non-goal (declined):** a `-truncate` flag on `bytdb_to_pg`. Refusing
    non-empty destinations protects live Postgres sites.
41. **Non-goal (no action):** page `opts.item_ids` saved as `[]` instead of
    `null`. Harmless, and identical on both backends.
42. **Non-goal (no action):** GET form/list/show handlers still return errors
    rather than flashes. Page definitions are static and only fail on a code
    bug.
43. **Non-goal (declined):** a transaction around role/permission writes.
    Delete-before-insert ordering bounds a partial failure to fewer grants, and
    the Executor seam has no `Begin`.
44. **Non-goal (declined):** a totals row in the per-gift giving CSV. It breaks
    sorting and filtering; the separate summary CSV has one.
45. **Non-goal (declined):** `GROUP BY`/`SUM` for the giving report. It isn't
    bytdb-portable, and the page lists every gift anyway.
