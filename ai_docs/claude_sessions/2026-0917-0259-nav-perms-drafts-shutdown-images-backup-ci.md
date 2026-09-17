# Session: Nav/list permission filtering, form drafts, graceful shutdown, atomic sermon uploads, images on e2, dbbackup on replicate/s3, CI

**Session:** 406d7d04-76be-452a-a8ca-fd122499e2ea
**Date:** 2026-09-17
**Continues:** `2026-0913-1820-role-lockout-chat-moderate-summary-csv.md`

## What happened

The user picked previous `## Next` items 10, 15 and 16. Then they asked for
every remaining item that is a code fix in church itself, done in batches,
with a commit and push after each batch.

| Repo | Commit | What |
|---|---|---|
| church | `7bb5df4` | Items 10, 15, 16: admin links in the nav and list actions follow permissions; giving module uses `.TE()` |
| church | `69ebdf2` | Batch 1: graceful shutdown closes the DB (24a), atomic sermon audio upload (29), production refuses test seeds (28) |
| church | `aa0753f` | Batch 2 (30): refused admin saves return to the form with what was typed |
| church | `34cd9a4` | Batch 3 (7, 2): CI workflow; `roles_pg_check` covers lockout and moderation |
| cema | `a61cc65` | Batch 3: site CI workflow |
| ccswm | `b5f5fce` | Batch 3: site CI workflow |
| church | `44604b1` | Batch 4 (23): article images use IDrive e2 as the durable copy; cema drops the uploads mount |
| church | `18713dd` | Batch 5 (24b): dbbackup uses the `replicate/s3` client shared with WAL shipping |
| cema | `f5dabf4` | Re-pin church to `18713dd` |
| ccswm | `9e9e193` | Re-pin church to `18713dd` |

- All pushed.
- Church CI passed on `34cd9a4` and `44604b1`, and both sites' CI passed on
  their workflow commits. The runs for `18713dd` and the two re-pins were still
  in progress when this doc was written.
- Uncommitted user edits were left alone: `README.md` in cema and ccswm, and
  `.cats-todo/` in ccswm.
- Nothing was checked in a browser or on a booted site.

## Behaviour

### Admin links in the nav follow permissions (item 15)

- `resource/menu/admin_links.go` maps each `/admin/...` link to the permission
  its route requires. The map mirrors `router_rweb.go` and has to be kept in
  step by hand.
  - The list page needs read, `/new` needs create, `/edit/` needs update.
  - `/pages/:id` needs read, `/sermons/import` needs sermons.create,
    `/sermons/cleanup` needs sermons.update.
  - `/giving` and everything under it needs charges.read.
  - `/admin`, `/home`, `/logout` and unrecognised admin URLs show for anyone
    with admin access.
  - `/debug` is SuperAdmin-only.
  - Public and external links always show.
- A submenu that filtering empties (shown 0, hidden > 0) is dropped entirely,
  so a chat member no longer sees an "Admin" dropdown. A submenu that was empty
  to begin with renders as before.
- **Where the viewer's permissions come from:**
  - `menu.RenderNav(slug, loggedIn, glob)` now takes `params["_global"]`.
  - On admin pages the permissions are already in `authz.ParamKey`.
  - On public pages a signed-in viewer is looked up with `LoadActor` by
    username. The lookup happens lazily, only when an admin link is reached, at
    most once per menu render.
  - A failed lookup hides admin links.
- The home page and `/login` renders now pass `username` so the nav can do this.
- Inactive menu items still render a bare `<li>`, not `class=""`.

### List actions follow permissions (item 16)

On the article, sermon, event, page and menu lists:

- "+" needs create.
- Edit needs update. On the menu list, the title's editor link needs update too.
- Delete needs delete.

### Giving module (item 10)

Donor text is written with `.TE()`, and the `html` import is gone.

### Graceful shutdown (item 24a)

- rweb's `Run` already traps SIGINT/SIGTERM: it closes the listener and
  returns.
- `ServeRWeb` now installs an in-flight request tracker (`s.Use`). After `Run`
  returns it drains open requests for up to 10s, then calls `db.CloseDB()`
  (`shutdown_rweb.go`).
- SSE streams are why the drain has a cap.
- `CloseDB` now also clears `dbHandle` and is documented as idempotent. The
  sites' `defer db.CloseDB()` still runs afterwards.

### Sermon audio upload (item 29)

- The upload streams to a hidden temp file (`.<name>.upload-*`) in the
  destination directory.
- It is renamed into place only after the row saves. Every earlier failure
  removes the temp file and leaves any old audio intact.
- If the rename fails after the save, the admin gets a flash saying to
  re-upload.
- Upload names containing a path (or starting with a dot) are refused. Before
  this, `../../x` could write outside the sermons directory.
- The audio link is still built from the name as sent.

### Seeds (item 28)

- Every committed `cfg/random_seeds.txt` in church, including the root one, is
  the `test-seed-NN` fixture, and always has been. Nothing needed replacing.
- New guard: with `APP_ENV=production`, `resource/auth`'s init refuses a seeds
  file with fewer than 16 usable seeds. Fixture lines and blank lines don't
  count.
- **Found:** `cema/cfg/random_seeds.txt.sample` (committed; the repo is
  private) is byte-identical to cema's local `cfg/random_seeds.txt`, and
  ccswm's committed sample is the same file. See Next.

### Refused saves keep typed values (item 30)

- **`core/formdraft`:** a one-shot draft in the in-process kvstore.
  - Keyed by session cookie plus form path (query string ignored).
  - 5 minute TTL; `Take` deletes the draft.
  - SameSite=Lax cookies mean a cross-site POST can't plant a draft.
- **Handlers:** article, sermon, event and user handlers read the form first
  and route every return-to-form refusal through `refuse(msg)`, which saves the
  draft.
  - This includes the expired-token warning, whose text now says the changes
    are still in the form.
  - Article inline images are still processed only after the token check.
- **Rendering:**
  - `basectlr` single/new renders put the draft in
    `params["_global"][formdraft.ParamKey]`, for admin pages only.
  - The new-article and new-sermon controllers call `template.Page` directly,
    so they use `base.TakeFormDraft`.
  - Form modules use the draft when `formdraft.SameItem` matches.
- **Users:**
  - `user.FormDraft` holds the presenter plus the posted role ticks.
  - Passwords are blanked before saving.
  - Ticks apply only to roles the viewer may grant; locked roles show their
    stored state.
- **Sermons:** the draft drops `AudioLink`, so the form keeps the stored link.
  The audio file has to be chosen again.
- **Event server faults** still go to the list, because a partial write is
  possible there. No transaction was added (see Next, non-goal).

### CI (item 7)

- **church:** `.github/workflows/ci.yml`, with `GOWORK=off`, installs libvips,
  then runs build, vet and test. The tests include the admin routes smoke test.
- **cema and ccswm:** `.github/workflows/ci.yml` has two jobs, triggered on
  push, pull request and a weekly schedule:
  - `pinned` builds and vets with the pinned church.
  - `church-master` warns when the pin lags church master, then builds against
    `church@master` after `go mod tidy`.

### `roles_pg_check` (item 2)

- It now works on an already-migrated dev database: if the `roles` table
  exists, it empties the three role tables inside its rolled-back transaction.
  The dev DB already has `20260913160000` (roles) and `20260911160000` applied.
- **New Postgres checks:**
  - `CanModerate`: legacy role, granted, and granted on a disabled account.
  - Six `LocksOutRoleManagers` scenarios. Other Administrator holders are
    removed inside the transaction first.
- 25 checks pass. Dev data was unchanged afterwards (still 4 roles).

### Article images on IDrive e2 (item 23)

- **Writes:** `resource/chimage/store.go`. `storeImage` writes
  `dist/img/<name>` (temp file, then rename). With `idrive.enabled` it also
  PUTs `images/<name>` asynchronously.
- **Reads:** `GET /assets/img/:filename` (`ServeImageRWeb`):
  - Serves the local file, or else checks and fetches from e2 and caches it
    locally.
  - 404 when neither has the image; 503 when e2 errors.
  - The name is percent-decoded before validation.
  - Only png/jpg/jpeg/gif/webp/bmp/ico are served inline. Anything else is
    `application/octet-stream` with `Content-Disposition: attachment`.
  - Responses are marked immutable.
- The more specific route beats the `/assets/*path` static route in either
  registration order; verified with a throwaway test.
- **In `ProcessInlineImages`:**
  - Editor filenames are reduced to a base name.
  - An unusable name, or a failed write, leaves the image inline as a data URL
    instead of a broken link.
  - The file name format (`<name>.<xxhash><ext>`) is unchanged.
- **`test_scripts/images_to_e2`:** run from the site directory. Dry run by
  default; `-apply` uploads. Existing keys are skipped, and a failed existence
  check counts as a failure.
- **Deploy:**
  - `cema.yaml` drops the `dist/img` uploads mount; cema's `options.yml` has
    `idrive.enabled: true`.
  - `ccswm.yaml` keeps the mount, with a comment on when to drop it.
  - The README "Uploaded images" section and migration step 4 are updated; the
    Dockerfile comment too.

### dbbackup on `replicate/s3` (item 24b)

- `db.BackupStore()` exports the backup bucket client that WAL shipping uses.
- `dbbackup.Run` snapshots, then `upload()`:
  - PUTs `<prefix>/<UTC ts>/church.db`.
  - Then PUTs `latest/`, only after the snapshot succeeds; there is no
    server-side copy call.
  - Then prunes via `Storage.List`.
  - All under a 10 minute timeout.
- No AWS SDK in the backup path. The SDK stays in the module for `core/s3ops`
  (the media bucket).
- `go mod tidy` only removed stale `go.sum` lines.
- Readiness doc item 6 is struck through as done.

## Verification

- **church**
  - `go build ./...`, `go vet` on touched packages, and `go test ./...` all
    pass.
  - The admin routes smoke test has 45 checks for nav/list, plus 3 for upload
    and 7 for drafts; all pass.
- **New unit tests**
  - `resource/menu`: link permissions and menu rendering
  - `church`: in-flight drain
  - `resource/auth`: seeds env check
  - `core/formdraft`
  - `resource/chimage`: names, store/serve, encoded names, non-image type
  - `resource/dbbackup`: in-memory store covering key layout, latest/ on
    failure, pruning
- **Other checks**
  - `go run ./test_scripts/roles_pg_check`: 25/25 against local Postgres,
    rolled back.
  - Both sites build with `GOWORK=off` (`go build`, `go vet ./...`) before and
    after the re-pin.
  - The `church-master` CI job was simulated on a scratch copy of ccswm, and
    its build passed.
- **Not done**
  - Nothing in a browser.
  - No site boot, so the shutdown log lines haven't been seen.
  - No live e2 image upload or fetch.
  - No live backup run against object storage.
  - The sermon upload's failed-save path (hard to trigger from a form).
  - The nav's public-page permission lookup against a real DB.

## Gotchas found

- **rweb path params are not percent-decoded.** `PathParam` returns `%20` as
  is.
- **The home page and `/login` passed no `username` in `_global`.** They do now.
- **Some admin "new" controllers call `template.Page` directly** (article,
  sermon, and menu/page controllers), so anything added to
  `basectlr.RenderPage*` params doesn't reach them.
- **Controller helpers are regex-replaced carelessly at your peril.** A bulk
  replace turned `refuse`'s own body into `return refuse(msg)`, which failed to
  compile, and was fixed by hand.
- **`/dev/tcp` probes don't work in this zsh.** Postgres is Homebrew
  `postgresql@16`: `/opt/homebrew/opt/postgresql@16/bin/pg_isready`. There is
  no `psql` on PATH.
- **No python `yaml` module;** use `ruby -ryaml`.
- **gofmt:** many files were already unformatted before this session (e.g.
  `router_rweb.go`, `basectlr`, `user_controller`, several list modules,
  `event_controller`, `user_presenter.go`). Only files that were clean at HEAD
  were formatted.
- **Editor noise, unchanged:** gopls "go.work requires go >= 1.26.1 (running go
  1.25.4)".

## Next

1. **New:** cema's committed `cfg/random_seeds.txt.sample` is identical to
   cema's local `cfg/random_seeds.txt`, and ccswm's sample is the same file.
   - Replace both samples with placeholders.
   - Check cema's production seeds on the live server and rotate them if they
     match. Rotation is safe: salts live in the DB, so logins survive.
2. **New:** before cema's k8s cutover, run
   `APP_ENV=production go run github.com/rohanthewiz/church/test_scripts/images_to_e2`
   from a directory holding the live `dist/img/`. Dry run, then `-apply`, until
   it reports `would copy: 0`.
3. **New:** ccswm: enable `idrive` in its `options.yml` (see 22), copy its
   images with `images_to_e2`, then drop the uploads mount from `ccswm.yaml`.
4. **New:** browser/site click-through of this session:
   - nav Admin dropdown as Editor, users-only and chat-member on a public page
   - list "+", Edit and Delete as a read-only role
   - drafts after a blank title, an expired form (leave a form open past the
     token TTL) and a password mismatch on events, sermons and users
   - the role ticks kept on the user form
   - an article image upload with IDrive on: object appears under `images/`;
     delete the local file and reload
5. **New:** boot a site and send SIGTERM. Confirm the "draining in-flight
   requests" and "Database closed" log lines, and a clean bytdb reopen.
6. **New:** trigger `POST /api/admin/db/backup` against real object storage
   (e.g. `./deploy/deploy.sh verify`, or curl with the token). Confirm both
   keys, and pruning with a small `retain`.
7. **New:** watch the first scheduled and pinned site CI runs. The last church
   (`18713dd`) and site re-pin runs were still in progress when this doc was
   written.
8. **New, optional:** a test for the sermon upload failed-save path, e.g. by
   injecting an upsert failure.
9. **New, reminder:** keep `resource/menu/admin_links.go` in step with
   `router_rweb.go` and the dashboard cards in `page/admin_home.go` when adding
   admin routes.
10. Decide whether cema's `feature/site-themes` branch can be deleted. cema
    `master` is pushed and contains it.
11. **New:** mobile moderation UI for a permission-only moderator. Check on a
    device or emulator that chat pin/delete and prayer answered controls appear
    once the server sends `can_moderate: true`.
12. **Optional:** existing sites whose default roles were already seeded lack
    `chat.moderate` on Publisher and Editor. Add it by hand if wanted.
13. **Recurring:** after each church push a site depends on, re-pin ccswm and
    cema. Both are on `18713dd` now, and site CI warns when the pin lags.
14. Browser click-through from earlier sessions:
    - the "Moderate" role column
    - lockout refusal flashes
    - `/debug/show` as Administrator vs SuperAdmin
    - both giving export buttons at phone width
    - the summary CSV opened in Excel
15. Add `TIME_ZONE` to `deploy/k8s/sites/cema.yaml` and `ccswm.yaml`, and set
    `time_zone` in each site's real `cfg/options.yml`.
16. Boot a site with `time_zone` set (and once with a bad name) to confirm the
    startup log line and the fatal message.
17. Browser click-through of `/admin/giving`:
    - year limits
    - month anchors
    - the unpaid note
    - phone-width scrolling
    - CSV in Excel
18. Check the giving report and summary CSV against real charge data on
    Postgres, and against Stripe for one month, with `time_zone` set.
19. Run the roles and `event_locations` migrations on any Postgres site. The
    dev DB already has both.
20. Browser click-through of the role screens:
    - role form
    - user form Roles card and locked states
    - refusal flashes
    - Editor's disabled publish switch
21. Boot cema locally with no `db.type` against Postgres. Confirm there is no
    `bytdb serving` line and pages render.
22. Document the `db:` block in the cema and ccswm `options-sample.yml`.
23. Start Docker and run `./deploy/deploy.sh images`. The CGO/libvips build is
    still unproven in Docker; CI now proves it on Ubuntu.
24. Provision LKE + Object Storage and fill `deploy/backup.env`. Then run
    `preflight infra`, point DNS, and run
    `base seeds secrets images sites verify`.
25. Create `ccswm/cfg/options.yml` from the sample, with a real `pg:` block (or
    `db.type: bytdb`), a real `time_zone` and an `idrive` block (see 3).
26. **Optional hardening:** `imagePullSecrets` and a www→apex redirect.
27. Live-check the 2026-09-12 flash changes:
    - bad event coordinates / expired csrf
    - empty menu items / page modules
    - referrer guard
    - `token.txt` permissions
28. Live-check the admin save flashes from `2026-0912-2125-…`. Expired csrf and
    blank title now also return a draft.
29. **Postgres coverage still missing:**
    - sermon create/import + audio
    - Stripe intent/history/webhook
    - `/chat/stream` SSE
    - image upload
    - giving report with data
    - Optionally let `bytdb_wire_check` take a Postgres DSN.
30. Add `bytdb_to_pg`, the `pg_to_bytdb` date fix and the Postgres smoke result
    to readiness doc §7.
31. **Optional:** per-table content checksum in `bytdb_to_pg`.
32. **Optional:** move `core/s3ops` (media bucket) off aws-sdk-go-v2 onto
    `replicate/s3` to drop the SDK entirely. It would need a HEAD/exists call
    (`ObjectInfo`), which the replicate client lacks.
33. **Optional:** gofmt the files that were already unformatted before this
    session (list under Gotchas), in a formatting-only commit.
34. **Non-goal (declined):** wrapping `UpsertEvent`'s writes in a transaction.
    The Executor seam has no `Begin`. Refused input returns a draft, and a
    server fault goes to the list because a partial write is possible.
35. **Non-goal (declined):** refilling the sermon audio file input from a
    draft. Browsers don't allow it; the admin re-selects the file.
36. **Non-goal (declined):** keeping typed passwords in a user form draft.
37. **Non-goal (declined):** serving editor-uploaded SVG/HTML-named images
    inline. Only raster types are, to keep script off the site's origin.
38. **Non-goal (declined):** a slug pre-check on article/sermon create.
39. **Non-goal (declined):** a catalog permission for `/debug/*`.
40. **Non-goal (declined):** moving moderation entirely off the legacy
    `users.role`.
41. **Non-goal (declined):** applying the lockout guard to SuperAdmin actors.
42. **Non-goal (declined):** a config-threaded `*time.Location` instead of
    replacing `time.Local`.
43. **Non-goal (declined):** reusing `TZ` as the override name.
44. **Non-goal (declined):** a `-truncate` flag on `bytdb_to_pg`.
45. **Non-goal (no action):** page `opts.item_ids` saved as `[]` instead of
    `null`.
46. **Non-goal (no action):** GET form/list/show handlers return errors rather
    than flashes.
47. **Non-goal (declined):** a transaction around role/permission writes.
48. **Non-goal (declined):** a totals row in the per-gift giving CSV.
49. **Non-goal (declined):** `GROUP BY`/`SUM` for the giving report.
