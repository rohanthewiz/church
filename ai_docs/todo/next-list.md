# Next list

The one living list of open follow-ups for church and the sites that pin it
(cema, ccswm). Sessions edit this file in place; they do not copy it forward.
Each session doc's `## Next` section records only what that session changed
here (`Closed: … Raised: …`). Mobile follow-ups live in church_mobile's and
grmob's own session docs.

**Conventions**

- **IDs are permanent.** Never renumber or reuse one. Take the next ID from
  the line below and bump it. An item keeps its ID when it moves between
  sections.
- **`raised`** is the session doc (filename stem) where the item first
  appeared. Age is computed from it by `/next-list`; it is never stored here,
  so an untouched item's line never changes.
- **`value`** is the payoff of doing it, not the effort. **high**: something
  is worked around today, or a second consumer has arrived. **medium**: it
  blocks one named thing, or it is a visible problem nobody routes around yet.
  **low**: nobody has hit it, or it is contingent on something that does not
  exist.
- **Nothing leaves Open without a line in Closed or Non-goals.** A silent
  deletion is the leak this file exists to prevent; `/next-list` checks for it
  in git history.
- Open is kept in ID order. Sorted views (by age or value) come from
  `/next-list`.

**Next ID: N-047**

## Open

- **N-001** · raised `2026-0719-1841` · value medium
  Provision LKE + Object Storage and fill `deploy/backup.env`. Then run
  `./deploy/deploy.sh preflight infra`, point DNS at the NodeBalancer IP, and
  run `./deploy/deploy.sh base seeds secrets images sites verify`. Verify also
  checks that WAL generations appear under `<prefix>/wal/gen/` and that
  `lag_seconds` is small. Blocked by N-006.
- **N-002** · raised `2026-0719-1841` · value medium
  Readiness §7 item 1: boot a site on bytdb and exercise the article/page/menu
  admin flows, "the last unchecked item before a real cutover". Lapsed from
  every Next list after `2026-0801-0852`; recovered 2026-09-19.
  - `admin_routes_smoke_test.go` now covers article create on an embedded
    bytdb (`db.InitDB`).
  - Page and menu saves on bytdb are still unproven.
  - Lands with N-011.
- **N-003** · raised `2026-0801-0956` · value low
  API additions for the mobile app, none built (checked 2026-09-19):
  - item image/thumbnail URLs
  - event start/end as RFC3339 datetimes
  - sermon duration/size
  - a search endpoint and filter facets
  - a chat `before_id` for deeper history (from `2026-0801-1124`)
  - a channel discovery endpoint (from `2026-0801-1124`)

  Lapsed after `2026-0801-1124`; recovered 2026-09-19. The app is now on
  grmob and its docs don't ask for these. Candidate for deletion or for
  handing to church_mobile.
- **N-004** · raised `2026-0801-0956` · value low
  Consider a `UNIQUE INDEX on charges(payment_token)` as a DB-level backstop to
  the recording mutex (only if bytdb supports unique indexes). Lapsed after
  `2026-0801-0956`; recovered 2026-09-19.
- **N-005** · raised `2026-0801-0956` · value low
  Site theme stylus tidy-up:
  - add `--af-*` / `--chg-*` overrides to the sites' theme files (none exist)
  - slim the old material-form classes; `page/login_form.go` is their only user

  Lapsed after `2026-0801-0956`; recovered 2026-09-19.
- **N-006** · raised `2026-0801-1935` · value medium
  Start Docker and run `./deploy/deploy.sh images`. The CGO/libvips build is
  unproven in Docker (Alpine); CI proves it only on Ubuntu. If bimg/libvips
  8.15 causes trouble, pin an older Alpine or use a pure-Go resizer. Blocks
  N-001.
- **N-007** · raised `2026-0801-1935` · value medium
  Create `ccswm/cfg/options.yml` from the sample, with a real `pg:` block (or
  `db.type: bytdb`), a real `time_zone` and an `idrive` block. Then copy
  ccswm's images with `images_to_e2` and drop the uploads mount from
  `ccswm.yaml`. (The sample already says `idrive.enabled: true`.) The
  `time_zone` half arrived here from N-018: ccswm has no real `options.yml` to
  set it in yet. Its pod is covered either way — `ccswm.yaml` now pins
  `TIME_ZONE=America/Chicago`, which beats the file.
- **N-008** · raised `2026-0801-1935` · value low
  Optional hardening:
  - `imagePullSecrets`, only if the ghcr packages go private (deletion
    candidate)
  - a www→apex redirect
- **N-009** · raised `2026-0912-1655` · value medium
  Postgres coverage still missing (Postgres is the default):
  - sermon create/import + audio
  - Stripe intent/history/webhook
  - `/chat/stream` SSE
  - image upload
  - the giving report with data
  - optionally let `test_scripts/bytdb_wire_check` take a Postgres DSN
- **N-010** · raised `2026-0912-1655` · value medium
  Run the roles and `event_locations` migrations (`dbc migrate up`) on any
  Postgres site before deploying current church to it. The dev DB has both.
- **N-011** · raised `2026-0912-1655` · value low
  In `ai_docs/fable_bytdb_k8s_readiness.md` §7, add `bytdb_to_pg`, the
  `pg_to_bytdb` date fix and the Postgres smoke result. Tick or update §7
  item 1 at the same time (N-002).
- **N-012** · raised `2026-0912-1655` · value low
  Optional: per-table content checksum in `bytdb_to_pg` beyond row counts. The
  round-trip dump diff covered the current schema. Deletion candidate.
- **N-013** · raised `2026-0912-1752` · value medium
  One browser click-through of everything since 2026-09-12:
  - **09-12 flash changes:** bad event coordinates or an expired csrf; empty
    menu items / page modules; the referrer guard; `token.txt` permissions
  - **Admin save flashes:** expired csrf, blank title, bad sermon date with
    audio chosen, password mismatch, stopped DB; each should also bring back a
    draft
  - **Role screens:** role form; user Roles card and locked states; Editor's
    disabled publish switch; the "Moderate" column; lockout refusal flashes
  - **`/admin/giving`:** year limits; month anchors; the unpaid note;
    phone-width scrolling; both export buttons; both CSVs opened in Excel
  - **Access:** `/debug/show` as Administrator vs SuperAdmin
  - **2026-09-17 work:** the nav Admin dropdown per role on a public page;
    list +/Edit/Delete as a read-only role; drafts, including role ticks; an
    article image upload with IDrive on, then delete the local file and reload
- **N-014** · raised `2026-0912-2125` · value medium
  Seeds: cema's committed `cfg/random_seeds.txt.sample` is identical to cema's
  local `cfg/random_seeds.txt`, and ccswm's sample is the same file (confirmed
  2026-09-19).
  - Replace both samples with placeholders.
  - Check cema's production seeds and rotate them if they match. Rotation is
    safe: salts live in the DB, so logins survive.
- **N-015** · raised `2026-0913-1542` · value medium
  Boot checks on a local site binary:
  - cema with no `db.type` against Postgres: no `bytdb serving` line, pages
    render
  - `time_zone` set, and once with a bad name: startup log line and fatal
    message
  - SIGTERM: "draining in-flight requests", then "Database closed", then a
    clean bytdb reopen
- **N-016** · raised `2026-0913-1542` · value low
  Document the `db:` block (`type: postgres|bytdb`, `file`, `listen`) in the
  cema and ccswm `cfg/options-sample.yml`. Missing from both (checked
  2026-09-19).
- **N-017** · raised `2026-0913-1725` · value medium
  Check the giving report and the summary CSV against real charge data on
  Postgres, and against Stripe for one month, with `time_zone` set.
- **N-019** · raised `2026-0913-1747` · value low
  Recurring: after each church push a site depends on, re-pin ccswm and cema.
  Site CI warns when a pin lags.
- **N-020** · raised `2026-0913-1820` · value low
  Delete cema's `feature/site-themes`. It is fully merged into `master`
  (checked 2026-09-19), so deleting it loses nothing.
- **N-021** · raised `2026-0913-1820` · value low
  Mobile moderation UI for a permission-only moderator: check that the
  controls appear once the server sends `can_moderate: true`. The app is on
  grmob now; this belongs in church_mobile's docs.
- **N-022** · raised `2026-0913-1820` · value low
  Optional: add `chat.moderate` by hand to Publisher and Editor on sites whose
  default roles were seeded before it existed. Legacy roles moderate either
  way.
- **N-023** · raised `2026-0917-0259` · value medium
  Before cema's k8s cutover, run
  `APP_ENV=production go run github.com/rohanthewiz/church/test_scripts/images_to_e2`
  from a directory holding the live `dist/img/`. Dry run, then `-apply`,
  until it reports `would copy: 0`.
- **N-024** · raised `2026-0917-0259` · value medium
  Trigger `POST /api/admin/db/backup` against real object storage (e.g.
  `./deploy/deploy.sh verify`, or curl with the token). Confirm both keys, and
  pruning with a small `retain`.
- **N-025** · raised `2026-0917-0259` · value medium
  Keep `resource/menu/admin_links.go` in step with `router_rweb.go` and the
  dashboard cards in `page/admin_home.go` when adding admin routes. Three
  hand-synced copies of one route→permission table: a single shared table
  would retire this.
- **N-026** · raised `2026-0917-0259` · value low
  Optional: a test for the sermon upload failed-save path, e.g. by injecting
  an upsert failure.
- **N-027** · raised `2026-0917-0259` · value low
  Optional: move `core/s3ops` (media bucket) off aws-sdk-go-v2 onto
  `replicate/s3`. Needs a HEAD/exists call (`ObjectInfo`), which the replicate
  client lacks.
- **N-028** · raised `2026-0917-0259` · value low
  Optional: gofmt the files that were already unformatted (list under Gotchas
  in `2026-0917-0259`), in a formatting-only commit.

## Non-goals

Declined on purpose. Kept so they stay visibly declined rather than silently
dropped; an item can move back to Open if its reason stops holding.

- **N-029** · declined `2026-0917-0259` — A transaction around `UpsertEvent`'s
  writes. The Executor seam has no `Begin`; refused input returns a draft, and
  a server fault goes to the list.
- **N-030** · declined `2026-0917-0259` — Refilling the sermon audio file input
  from a draft. Browsers don't allow it.
- **N-031** · declined `2026-0917-0259` — Keeping typed passwords in a user
  form draft.
- **N-032** · declined `2026-0917-0259` — Serving editor-uploaded SVG/HTML-named
  images inline. Only raster types are, to keep script off the site's origin.
- **N-033** · declined `2026-0913-1820` — A slug pre-check on article/sermon
  create. Slugs carry a nanosecond-hash suffix; duplicate titles save.
- **N-034** · declined `2026-0913-1820` — A catalog permission for `/debug/*`.
  SuperAdmin-only by design.
- **N-035** · declined `2026-0913-1820` — Moving moderation entirely off the
  legacy `users.role`. The rule is legacy OR `chat.moderate`.
- **N-036** · declined `2026-0913-1820` — Applying the lockout guard to
  SuperAdmin actors. They are the recovery path.
- **N-037** · declined `2026-0913-1747` — A config-threaded `*time.Location`
  instead of replacing `time.Local`.
- **N-038** · declined `2026-0913-1747` — Reusing `TZ` as the override name.
- **N-039** · declined `2026-0912-1655` — A `-truncate` flag on `bytdb_to_pg`.
  Refusing non-empty destinations protects live Postgres sites.
- **N-040** · no action `2026-0912-1718` — Page `opts.item_ids` saved as `[]`
  instead of `null`. Harmless, identical on both backends.
- **N-041** · no action `2026-0912-2125` — GET form/list/show handlers return
  errors rather than flashes. Page definitions are static.
- **N-042** · declined `2026-0913-1623` — A transaction around role/permission
  writes. Delete-before-insert bounds a partial failure to fewer grants.
- **N-043** · declined `2026-0913-1725` — A totals row in the per-gift giving
  CSV. The separate summary CSV has one.
- **N-044** · declined `2026-0913-1725` — `GROUP BY`/`SUM` for the giving
  report. Not bytdb-portable.
- **N-045** · declined `2026-0801-0956` — Making the web form token
  single-use. Resubmits after failed validation would break. (Lapsed from the
  lists after `2026-0801-0956`; recovered 2026-09-19.)

## Closed

Newest first. Items closed before this file existed (2026-09-19) are recorded
in the session docs' bodies.

- **N-018** · raised `2026-0913-1725` · closed 2026-09-19 — `TIME_ZONE:
  America/Chicago` added to the Deployment env in both
  `deploy/k8s/sites/cema.yaml` and `ccswm.yaml`, `time_zone: America/Chicago`
  set in `defaults` of cema's real (gitignored) `cfg/options.yml`, and the
  reason a container needs the env var documented in `deploy/k8s/README.md`.
  cema's Secret picks the file up on the next `deploy.sh secrets`. ccswm has no
  real `options.yml`; that half moved to N-007, and its pod is covered by the
  manifest meanwhile. Not yet seen on a booted site — that stays N-015.
- **N-046** · raised `2026-0917-0259` · closed 2026-09-19, `/next-list`
  rebuild — Watch the first church (`18713dd`) and site re-pin CI runs. All
  green, checked with `gh run list`.
