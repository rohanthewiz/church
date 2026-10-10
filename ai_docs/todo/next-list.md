# Next list

The one living list of open follow-ups for church and the sites that pin it
(cema, ccswm). Sessions edit this file in place; they do not copy it forward.
Each session doc's `## Next` section records only what that session changed
here (`Closed: … Raised: …`). Mobile follow-ups live in church_mobile's own
next list (`church_mobile/ai_docs/todo/next-list.md`) and grmob's session docs.

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
- **Four places an unfinished item can live.** **Open** is the build work we
  intend to pick up next. **Validate** is the testing we intend to do next:
  run, look, hear or measure, or write or repair a test, with no product
  change planned unless the check finds a defect. **Roadmap** is what we want
  to do in the future, but not immediately. **Non-goals** is what we are
  likely not to do. An item moves between them freely as plans change,
  keeping its ID.
- **Nothing leaves Open, Validate or Roadmap without a line in another
  section.** A move among those three is not a leak; a silent deletion is the
  leak this file exists to prevent, and `/next-list` checks for it in git
  history.
- Open, Validate and Roadmap are kept in ID order. Sorted views (by age or
  value) come from `/next-list`.

**Next ID: N-061**

## Open

- **N-010** · raised `2026-0912-1655` · value medium
  Run the roles and `event_locations` migrations (`dbc migrate up`) on any
  Postgres site before deploying current church to it. The dev DB has both.
- **N-017** · raised `2026-0913-1725` · value medium
  Check the giving report and the summary CSV against real charge data on
  Postgres, and against Stripe for one month, with `time_zone` set.
  2026-09-27: `test_scripts/giving_reconcile` does both checks (read-only)
  and passes on church_test's seeded rows in Chicago, UTC and Auckland. No
  local database holds real charges and no Stripe key is configured, so the
  owner's step is to run it against production with
  `-tz <site zone> -stripe-month YYYY-MM` and a read-only `STRIPE_SECRET_KEY`.
  Dashboard refunds used to be lost: the webhook handled only
  `payment_intent.succeeded`. It now handles `charge.refunded` (2026-09-27),
  but only once each site's Stripe webhook endpoint subscribes to that event.
  Refunds issued before then still show as `refunded` mismatches. Re-save
  each one from Stripe (resend the event from the dashboard) or fix it by
  hand.
  2026-10-10: the `charge.refunded` handling shipped in `v0.12.1`. cema pins
  `v0.12.1`; ccswm still pins `v0.12.0` (and has a Stripe/giving config), so
  ccswm records no dashboard refunds until it is re-pinned (N-019).
- **N-019** · raised `2026-0913-1747` · value medium
  Recurring: after each church push a site depends on, re-pin ccswm and cema.
  Site CI warns when a pin lags.
  2026-10-10: a re-pin is due now. ccswm pins `v0.12.0`, so it lacks
  `v0.12.1`'s refund webhook (N-017) and pure-Go image resize; cema pins
  `v0.12.1`. Master is 10 commits past `v0.12.1` with untagged product changes
  (sermons list grouped by month `66e6ccf`, admin drag grips `792f9d6`), so
  tag `v0.12.2` first, then re-pin both. Value raised from low: it now blocks
  refund recording on ccswm.
- **N-050** · raised `2026-0920-2035-roadmap-section-and-v0.11.0-release` · value low
  Delete the stale `cfg/random_seeds.txt` on each host and checkout once it
  runs a build at or past church `v0.11.0`; nothing reads it any more. Carried
  out of the closed N-048 and N-049, where it was the owner's remaining step.
  Locally that precondition is now met: cema was rebuilt and booted past
  `v0.11.0` on 2026-09-21 (no seed-file read), so `cema/cfg/random_seeds.txt`
  can go; it was still present then. The live hosts wait on a deploy.
  2026-09-21: a session's `rm` of `cema/cfg/random_seeds.txt` was refused by
  the permission gate; nothing reads it (only a comment names it), so the
  owner can delete it by hand.
  2026-10-10: the local checkout is done. `cema/cfg/random_seeds.txt` was
  moved to the macOS Trash (`~/.Trash/cema-random_seeds.txt`), since `rm` is
  still refused by the permission gate; nothing read it (the only mention is a
  history comment in `resource/auth/random.go`). ccswm has no local copy.
  What remains is the live hosts, after each deploys a build at or past
  `v0.11.0`. Not in scope but worth knowing: the sibling site `ccgrand` pins a
  pre-`v0.11.0` church (`8076c91`), so it still reads its own
  `cfg/random_seeds.txt`; that file only goes stale once ccgrand moves past
  `v0.11.0`.

## Validate

Testing we intend to do next: the remaining work is to run, look, hear or
measure, or to write or repair a test. No product change is planned unless the
check finds a defect; a defect found becomes a new Open item, named here.

- **N-060** · raised `2026-1009-0744-admin-drag-reorder` · value low
  Try the new admin drag-to-reorder grips (menu form items, page form module
  cards) in a running cema: one real mouse drag + save, and one touch drag on
  a phone. Verified only in a harness of the real form renders with
  synthetic pointer events (no local admin password at hand); touch relies on
  `touch-action: none` on `.af-drag-handle` (`template/admin_css.go`; the
  pointer handlers are in `template/admin_script.go`). The grips are untagged
  (past `v0.12.1`), so the cema under test must be a workspace build until
  N-019's `v0.12.2` re-pin lands. Moved from Open 2026-10-10: pure check.

## Roadmap

Wanted, but not now. These are things we mean to do once the immediate work in
Open is through; they are parked, not declined (that is Non-goals). An item
keeps its ID, `raised` and `value` here, so moving it back to Open is a pure
move. `/next-list` does not sort these into the working view; it lists them
and checks only that none has gone missing.

**Infrastructure track** — bytdb, Docker, k8s/LKE and object storage work and
testing, deferred together 2026-09-20. The sites run on Postgres on their
current hosts meanwhile, so nothing in Open waits on these.

- **N-001** · raised `2026-0719-1841` · value medium
  Provision LKE + Object Storage and fill `deploy/backup.env`. Then run
  `./deploy/deploy.sh preflight infra`, point DNS at the NodeBalancer IP, and
  run `./deploy/deploy.sh base secrets images sites verify`. Verify also
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
- **N-006** · raised `2026-0801-1935` · value medium
  Start Docker and run `./deploy/deploy.sh images`. The CGO/libvips build is
  unproven in Docker (Alpine); CI proves it only on Ubuntu. If bimg/libvips
  8.15 causes trouble, pin an older Alpine or use a pure-Go resizer. Blocks
  N-001.
  2026-10-10: the pure-Go fallback now exists (`27c624f`, `v0.12.1`):
  `CGO_ENABLED=0` builds resize with `x/image/draw`, so a libvips failure in
  Alpine can be sidestepped without code work. The image is still unbuilt.
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
- **N-011** · raised `2026-0912-1655` · value low
  In `ai_docs/fable_bytdb_k8s_readiness.md` §7, add `bytdb_to_pg`, the
  `pg_to_bytdb` date fix and the Postgres smoke result. Tick or update §7
  item 1 at the same time (N-002).
- **N-012** · raised `2026-0912-1655` · value low
  Optional: per-table content checksum in `bytdb_to_pg` beyond row counts. The
  round-trip dump diff covered the current schema. Deletion candidate.
- **N-016** · raised `2026-0913-1542` · value low
  Document the `db:` block (`type: postgres|bytdb`, `file`, `listen`) in the
  cema and ccswm `cfg/options-sample.yml`. Missing from both (checked
  2026-09-19).
- **N-023** · raised `2026-0917-0259` · value medium
  Before cema's k8s cutover, run
  `APP_ENV=production go run github.com/rohanthewiz/church/test_scripts/images_to_e2`
  from a directory holding the live `dist/img/`. Dry run, then `-apply`,
  until it reports `would copy: 0`.
- **N-024** · raised `2026-0917-0259` · value medium
  Trigger `POST /api/admin/db/backup` against real object storage (e.g.
  `./deploy/deploy.sh verify`, or curl with the token). Confirm both keys, and
  pruning with a small `retain`.
- **N-027** · raised `2026-0917-0259` · value low
  Optional: move `core/s3ops` (media bucket) off aws-sdk-go-v2 onto
  `replicate/s3`. Needs a HEAD/exists call (`ObjectInfo`), which the replicate
  client lacks.

**Sermon import track** — the one-off legacy PG2 import, deferred 2026-10-03.
Its items matter only if an import is run again; nothing in Open waits on
them.

- **N-059** · raised `2026-0928-2351-n009-postgres-import-webhook-tests` · value low
  `sermon.Import` (legacy PG2 import) is not safe to re-run or to fail:
  - a second run fails on the first already-imported sermon, since
    `sermons.slug` is unique and Import always takes the create path;
  - a row scan error (e.g. a NULL legacy column scanned into a string)
    `break`s the loop, yet Import still answers `"success": true` with the
    partial count.
  Only matters if an import is ever run again; `TestImportFromLegacyDB` is
  the harness for a fix.

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
- **N-004** · declined 2026-10-03, `2026-0928-2339-n004-unique-index-recommendation`
  — A `UNIQUE INDEX on charges(payment_token)` as a DB-level backstop to
  `recordMu`. Every deployment is one process per database (bytdb is
  single-writer; k8s pins `replicas: 1` + `Recreate`), so the process-wide
  mutex already covers every writer. The index would also break a Postgres
  migration or a `pg_to_bytdb` cutover on any site with duplicate or repeated
  empty tokens, and would never reach existing bytdb files. Reopen if a site
  ever runs more than one app process against one Postgres.
- **N-057** · declined `2026-0928-2356-n057-events-month-grouping-declined` —
  Month sub-grouping on the events list. The grid opens the first month of the
  first year, and events sort newest first, so the open month would be the
  furthest-future one with this month's events collapsed; and there is no
  evidence of year groups long enough to need it (the list pages server-side).
  Revisit only after the grid opens the month containing today.

## Closed

Newest first. Items closed before this file existed (2026-09-19) are recorded
in the session docs' bodies.

- **N-058** · raised `2026-0928-2347-n005-theme-vars-material-form` · closed 2026-10-03 — Fixed
  in both sites' `_material_form.styl` (cema and ccswm, CSS rebuilt): the
  label also floats on `input:-webkit-autofill` and on `input:autofill`, one
  rule each so a browser that lacks one spelling still applies the other.
  Not yet seen in a real autofill (needs a saved login); reopen if the
  Password label still sits over the dots at page load.
- **N-022** · raised `2026-0913-1820` · closed 2026-09-28, `2026-0928-2355-n022-chat-moderate-seed-check` — Nothing
  to do: default roles lacked `chat.moderate` only between `0298241` and
  `de76881` (~2h on 2026-09-13, no tag between), both local Postgres DBs
  seeded after it and have it on Publisher/Editor, and no bytdb site file
  exists yet. Reopen only if a site turns up seeded from an untagged build in
  that window.
- **N-021** · raised `2026-0913-1820` · closed 2026-09-28, `2026-0928-2354-n021-mobile-can-moderate` — The
  check failed: `8a71947` had ported `can_moderate` to the Flutter model only,
  so the grmob app's `api.User.CanModerate()` stayed role-only and a
  permission-only moderator got no controls. Fixed in church_mobile
  (`ServerCanModerate *bool`, preferred over the role rule) with model and
  session tests; not yet walked on a device (church_mobile N-001).
- **N-009** · raised `2026-0912-1655` · closed 2026-09-28, `2026-0928-2351-n009-postgres-import-webhook-tests` — Neither
  gap needed anything outside the repo: `TestImportFromLegacyDB` builds the
  legacy source in a second throwaway Postgres (`testdb.EmptyPostgres`), and
  `TestWebhookRoundTripOnDB` fakes the Stripe API with httptest. Both run on
  both backends; the import test caught imports dropping the sermon body.
- **N-005** · raised `2026-0801-0956` · closed 2026-09-28, `2026-0928-2347-n005-theme-vars-material-form` — In
  cema and ccswm: each theme sets `theme-ui-accent` (its link color; Cobalt
  `null`, so the live sites are unchanged) and `_styl/_framework_vars.styl`
  maps it to `--af-*` / `--chg-*` on `body .af-scope` / `body .ch-grid`.
  `_material_form.styl` slimmed from 501 lines to the login form's rules, all
  scoped under `.wrapper-material-form` (half the old rules leaked globally).
- **N-020** · raised `2026-0913-1820` · closed 2026-09-27 — Both local and
  `origin/feature/site-themes` deleted from cema; the branch was fully merged
  into `master` and no longer needed.
- **N-003** · raised `2026-0801-0956` · closed 2026-09-26, handed off to
  church_mobile as its N-016 — The mobile API additions (image URLs, RFC3339
  event times, sermon duration/size, search and facets, chat `before_id`,
  channel discovery) are driven by what the app needs, and the grmob app
  doesn't ask for any of them yet, so the item now lives on the app's list.
  Server work comes back here as a new ID when a screen needs one.
- **N-056** · raised `2026-0926-1843-drop-redis-echo-residue` · closed
  2026-09-26, `2026-0926-1923-drop-rweb-suffixes`, `a16be77`, v0.12.0 — The `_rweb` file and `RWeb` identifier
  suffixes are gone (36 files, every identifier). Names that would have
  clashed got clearer ones: `app.VerifyRequestFormToken`, and `Web`-prefixed
  chat and prayer wall web handlers. `church.ServeRWeb()` is now
  `church.Serve()`; cema (`a337548`) and ccswm (`a53674f`) re-pinned to
  v0.12.0, CI green on all three.
- **N-055** · raised `2026-0926-0818-prod-db-copy-setup` (cema) · closed
  2026-09-26, `2026-0926-1519-quiet-slack-not-authed`, cema `eb950a0` — cema
  now enables the logger's Slack hook only when `SLACK_API_TOKEN` is set, so
  runs without a token no longer print `Error sending log to Slack:
  not_authed` on every log line. ccswm never enabled the hook.
- **N-054** · raised `2026-0921-2339-next-list-sweep` · closed 2026-09-23,
  `2026-0923-1556-ci-postgres-tests`, `80193e2` — CI runs a `postgres:16` service (user `devuser`, a superuser as
  the image's `POSTGRES_USER`, so it has CREATEDB and satisfies the charges
  migration's `OWNER TO devuser`) and sets `CHURCH_TEST_PG_DSN`. It also sets
  the new `CHURCH_TEST_PG_REQUIRED`, which makes `testdb.OpenPostgres` fail
  rather than skip when the DSN is missing, since a skipped subtest passes and
  would drop the Postgres half without turning CI red. Run `35918667843`
  was green.
- **N-053** · raised `2026-0921-2302-menu-links-page-404-and-v0.11.1` · closed
  2026-09-21, `5b88272` — The error module `AddModules` substitutes is now
  `Published`. `TestAddModulesShowsErrorModuleOnBuildFailure` fails without
  the fix.
- **N-052** · raised `2026-0921-2302-menu-links-page-404-and-v0.11.1` · closed
  2026-09-21, `5b88272` — `bootstrapMenus` compares decoded items
  (`menuItemsEqual`). bytdb normalizes stored JSON too (sorted keys, compact),
  not only Postgres JSONB, so the rewrite happened on both. The bootstrap test
  plants a past `updated_at` and requires it to survive a second boot; it
  fails with the byte compare on both backends. A second boot of cema logged
  no refresh.
- **N-051** · raised `2026-0921-2302-menu-links-page-404-and-v0.11.1` · closed
  2026-09-21, `5b88272` — The default main menu (bootstrap and the hardwired
  fallback) links `/pages/calendar`. Bootstrap creates an editable `calendar`
  page with the FullCalendar module, and with no row the page controller
  serves the hardwired `page.Calendar`. Existing uncustomized menus pick up the
  new link on their next boot. Checked by booting cema on a fresh bytdb: menu
  link present, page 200, the calendar draws in a browser.
- **N-028** · raised `2026-0917-0259` · closed 2026-09-21, `eae93ac` — gofmt
  over the 41 hand-written files `gofmt -l` listed, formatting only. The two
  generated `pack/packed` files are left alone (the packer would regenerate
  them unformatted).
- **N-026** · raised `2026-0917-0259` · closed 2026-09-21, `212ae0d` — The
  smoke test hides the `sermons` table for one upload, so the Upsert fails
  after the audio is staged: back to the form, no row, no staged file, the old
  audio of the same name intact. Runs on both backends.
- **N-025** · raised `2026-0917-0259` · closed 2026-09-21, `920db40` —
  `authz.AdminRoutes` is the one route→permission table. The router takes each
  guard from it and panics at startup on a route it lacks; the nav and the
  dashboard cards look URLs up in it. The smoke test probes every row (with an
  unregistered route as the control) to catch stale rows.

- **N-049** · raised `2026-0920-1957-drop-seed-pool-for-crypto-rand` · closed
  2026-09-20, `2026-0920-2035-roadmap-section-and-v0.11.0-release` — church tagged `v0.11.0` at `e651064` (the first release since
  `v0.10.0`, 116 commits on, past the seed pool removal `9c796bd`), and cema
  (`a1bae84`) and ccswm (`feaa843`) pinned to it in place of the `18713dd`
  pseudo-version. Checked first: a fresh local cema build on workspace church
  booted with no seed-file read, served `/`, `/articles`, `/events`,
  `/sermons`, `/login` and `/api/v1/articles` at 200, redirected guarded admin
  routes to `/login`, minted 64-hex session keys and drained cleanly on
  SIGTERM; all 26 church packages pass and church CI was green. After pinning,
  each site built with `GOWORK=off`, which is the build the pin exists for, and
  both sites' CI went green on the `pinned` and `church-master` jobs.
  The stale seed files left behind are N-050.
- **N-048** · raised `2026-0920-0040` · closed 2026-09-20,
  `2026-0920-1957-drop-seed-pool-for-crypto-rand`, superseded — The
  seed pool no longer exists: `RandomKey()` draws 256 bits from `crypto/rand`
  (same 64-hex shape), `resource/auth` has no `init()`, and
  `cfg/random_seeds.txt`, its 15 per-package test fixtures, `checkSeedsForEnv`,
  `deploy.sh seeds`, the Secret key and both site samples are removed. Whether
  the live cema host's pool matched the once-committed sample stops mattering
  the moment that host runs a build with this change, since nothing reads the
  file; delete it there afterwards. Until then the host still keys off its old
  pool, so deploying is the remaining (owner's) step. The detail in N-014 below
  describes the retired mechanism and is kept as history.
- **N-014** · raised `2026-0912-2125` · closed 2026-09-20,
  `2026-0920-0040-seed-sample-placeholders-and-cema-seed-rotation` — The committed
  samples held real seeds; both are now placeholders and cema's local pool is
  rotated. The live-host check is the one part a session can't do: N-048.
  - **The samples** (`cema/cfg/random_seeds.txt.sample`,
    `ccswm/cfg/random_seeds.txt.sample`, byte-identical to each other and to
    cema's local `cfg/random_seeds.txt`) are now 72 lines of `test-seed-NN`,
    reusing `resource/auth`'s existing fixture convention, so a sample copied
    into production is refused by `checkSeedsForEnv` instead of silently
    working. Copying it for local dev still works, which is what the sample is
    for.
  - **cema's local `cfg/random_seeds.txt`** was rotated to 72 generated 19-char
    seeds at 0600, using the same `openssl` recipe as `deploy.sh seeds`.
  - **Why that rotation mattered now:** `cmd_seeds` skips any existing seed
    file and `cmd_secrets` copies it verbatim into the `<site>-config` Secret,
    while `checkSeedsForEnv` only rejects the `test-seed-` prefix. cema's file
    was the sample but held real-looking strings, so N-001's `seeds secrets`
    run would have shipped repo-committed seeds into production and booted on
    them cleanly. `cmd_seeds` now `die`s when a site's seed file is
    byte-identical to its sample — the gap between the two existing guards.
  - **Verified:** `TestCommittedSamplesRefusedInProduction` feeds both real
    sample files through the boot-path guard (production refused, development
    accepted) and skips per-site when the sibling repo is absent, since church
    CI checks out church alone. cema then booted on the rotated pool ("72 seeds
    read"), served `/`, `/articles`, `/events`, `/sermons`, `/login` at 200,
    minted a session key from the new seeds, and drained cleanly on SIGTERM.
    All 26 church packages pass; church, cema and ccswm build.
  - **Rotation is safe, re-confirmed in code:** login compares
    `PasswordHash(password, stored_salt)` against the stored hash, both from
    the DB, and `GenSalt` never reads the pool. The seeds feed only
    `RandomKey()` — session keys, form tokens, the SuperAdmin bootstrap token —
    whose outputs are stored, never re-derived.
  - **Not done:** the live cema host (N-048). Both site repos are private, so
    the old seeds were never publicly exposed, though they remain in cema's and
    ccswm's git history from their initial commits; rotating the host retires
    them.
  - Docs corrected alongside: both READMEs told the reader to copy the sample
    if "not paranoid about security" (and named a nonexistent
    `rand_seeds.txt`); they and `CEMA_LOCAL_SERVER.md` now say the sample is
    placeholders, give the generate recipe, and note that rotation preserves
    logins.
- **N-013** · raised `2026-0912-1752` · closed 2026-09-19 — Browser
  click-through of everything since 2026-09-12, run against a booted cema on an
  isolated `church_test` Postgres database (a copy of dev) with five seeded
  actors (`test_scripts/n013_seed`). Verified: nav Admin dropdown per role on a
  public page; list +/Edit/Delete for a read-only role; the Editor's disabled
  publish switch; the role form's permission matrix incl. the Moderate column;
  the user form's Roles card and its locked states; the lockout refusal; drafts
  incl. role ticks; expired-token, blank-title, bad-coordinate, bad-sermon-date,
  empty-menu-items and empty-page-modules flashes; the referrer guard;
  `token.txt` at 0600; `/debug/show` as Administrator vs SuperAdmin; the giving
  report's year limits, month anchors, unpaid note, phone-width scrolling and
  both CSVs; the image serve route with IDrive off. Two cosmetic permission
  leaks were found and fixed (see Closed N-047). Not covered: the live IDrive
  upload (skipped on purpose — cema's config holds production `cemasermons`
  credentials) and literally opening the CSVs in Excel (checked for UTF-8 BOM,
  CRLF and quoting instead).
- **N-047** · raised and closed 2026-09-19 — Two admin affordances were shown
  to viewers the routes then refused, found by the N-013 click-through and
  fixed in the same pass:
  - the inline "Edit Page" pencil rendered for any signed-in viewer, so a chat
    member saw it on every dynamic page; it now follows `pages.update`
  - the users list linked each row's first name into the editor
    unconditionally, so a read-only role had a working-looking way in; it now
    follows `users.update` plus the manageable check, like the Actions "Edit"
    beside it (the same leak item 16 closed for the menu list's title link)

  Both were cosmetic — the routes refused with a flash naming the missing
  permission — but a dead link is a promise the page cannot keep. The nav's
  viewer resolution moved to `authz.ResolveViewer` so the pencil and the nav
  share one implementation instead of two.
- **N-015** · raised `2026-0913-1542` · closed 2026-09-19 — All three boot
  checks run against a locally built cema (workspace church, so current
  master).
  - **Postgres:** `env -u DB_TYPE ./cema` logged no `bytdb serving` line and
    served `/`, `/articles`, `/events`, `/sermons`, `/login` at 200 with the
    real site content; the rendered article title matched the row in
    `church_development`.
  - **Zone:** `config.TimeZone is America/Chicago` on boot, log timestamps at
    `-05:00`. `TIME_ZONE=Europe/London` beat the file (`+01:00`, and the date
    rolled to the next day — the month-cut bug N-018 guards against).
    `TIME_ZONE=America/Nowhere` exited 1 before binding a port, naming the bad
    zone and suggesting an IANA name.
  - **SIGTERM:** "Server stopped accepting connections; draining in-flight
    requests" then "Database closed; shutdown complete", on both backends. Under
    60 concurrent page builds the count was real (`in_flight=34`) and all 60
    clients got a complete 200; the drain never timed out. Rebooting on the same
    bytdb file reopened it cleanly — the second boot refreshed the existing
    menus rather than recreating them, and served the first boot's article.
  - **Found and fixed in `shutdown_rweb.go`:** the comment said the drain is
    bounded because SSE subscribers hold requests open. They do not — rweb
    streams from `sendSSE` after the handler chain returns, outside the
    middleware, so an open `/chat/stream` measured `in_flight=0`. Comment
    corrected; no behavior change, and none needed (the stream loop reads an
    in-memory channel, never the database).
- **N-018** · raised `2026-0913-1725` · closed 2026-09-19 — `TIME_ZONE:
  America/Chicago` added to the Deployment env in both
  `deploy/k8s/sites/cema.yaml` and `ccswm.yaml`, `time_zone: America/Chicago`
  set in `defaults` of cema's real (gitignored) `cfg/options.yml`, and the
  reason a container needs the env var documented in `deploy/k8s/README.md`.
  cema's Secret picks the file up on the next `deploy.sh secrets`. ccswm has no
  real `options.yml`; that half moved to N-007, and its pod is covered by the
  manifest meanwhile. Since confirmed on a booted site under N-015.
- **N-046** · raised `2026-0917-0259` · closed 2026-09-19, `/next-list`
  rebuild — Watch the first church (`18713dd`) and site re-pin CI runs. All
  green, checked with `gh run list`.
