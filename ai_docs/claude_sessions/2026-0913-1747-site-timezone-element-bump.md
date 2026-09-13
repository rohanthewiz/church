# Session: Site time zone config option, element v0.7.0, ccswm standalone build

**Session:** https://claude.ai/code/session_01NQNwAU3z6vZ7GgJDWAPfjE
**Date:** 2026-09-13
**Continues:** `2026-0913-1725-giving-by-month-csv.md`

## What happened

The user asked for two items from the previous `## Next`:

- Set the site time zone via a config option (was item 3).
- Bump the element version used (was item 4).

Then: fix the ccswm standalone (`GOWORK=off`) build, which was already broken
before this session.

All three are done, committed and pushed. Nothing was run in a browser and no
site was booted with the new option.

## Behaviour

- **`time_zone`** (yaml, per environment section; the samples put it in
  `defaults`): an IANA name such as `America/Chicago`.
  - **`TIME_ZONE`** env var overrides it (`config/env_overrides.go`).
    - The name is deliberately not `TZ`. The Go runtime reads `TZ` before
      config loads, and an unset `time_zone` must keep honoring it.
  - **Empty** (both unset): `time.Local` is untouched, so the process zone
    applies (`TZ`, else the host). In k8s pods that is **UTC**.
  - **Invalid name:** `log.Fatal` at startup, matching the rest of config
    loading. A silent UTC fallback would put evening events and Dec 31 gifts on
    the wrong day.
  - **Startup log:** `config.TimeZone is <zone>`.

## Design

```
site main.go init()
   └─ config.InitConfig
        ├─ load yaml section for APP_ENV
        ├─ envOverride           (TIME_ZONE → TimeZone)
        └─ applyTimeZone(name)   → time.Local = time.LoadLocation(name)
                                   │
   everything "church-local" already goes through time.Local:
     time.Now()                              giving report now.Location()
     time.LoadLocation("Local")              event/sermon presenters
     time.Date(..., time.Local)              smoke scripts
```

- **Replace `time.Local` instead of threading a `*time.Location`.** A threaded
  location would mean changing every call site above and every future one. One
  assignment covers them all, and it is equivalent to `TZ` but lives in the
  site config.
  - It is safe because `InitConfig` runs from the site's `init()`, before any
    goroutine reads the clock.
  - Times created earlier keep their old zone pointer, so `applyTimeZone` must
    never be called after startup. The doc comment says so.
- **`_ "time/tzdata"`** is imported in `config` (~450KB). `LoadLocation` tries
  the host's zoneinfo first, and this embedded copy is only the fallback. It
  keeps a minimal image or bare host from failing to boot once `time_zone` is
  set. The Docker runtime stage already installs `tzdata`.
- **element v0.5.4/v0.5.6 → v0.7.0:** diff reviewed, with no breaking API
  changes for church.
  - Adds `TE()` (escaped text), `H5`/`H6`, builder pool, `Cached`, components
    and `Pretty()`.
  - Attributes now render in insertion order (previously map order).
  - A `"` inside an attribute value is now written as `&#34;`. It used to
    produce broken HTML. Pre-escaped values are unaffected because `&` is not
    re-escaped.
  - No church test depended on attribute order or raw quotes.

## Changes

- **church** (`a68100c`, pushed):
  - `config/config.go`:
    - `EnvConfig.TimeZone` (`yaml:"time_zone"`).
    - `applyTimeZone` is called in `InitConfig` after `envOverride`.
    - `time` and `time/tzdata` imports.
  - `config/env_overrides.go`: `TIME_ZONE`.
  - `config/timezone_test.go` (new):
    - sets `time.Local`, and `LoadLocation("Local")` follows it
    - 02:00 UTC Jan 1 stays in Dec 2025 in Chicago
    - a blank name keeps `time.Local`
    - an invalid name errors and leaves `time.Local` alone
    - the `TIME_ZONE` override
  - `go.mod`/`go.sum`: element v0.7.0.
- **cema** (branch `feature/site-themes`):
  - `cfg/options-sample.yml`: documented `time_zone` in `defaults`.
  - `go.mod`/`go.sum`: element v0.7.0 and serr v1.3.0 (indirect).
- **ccswm** (`master`):
  - `cfg/options-sample.yml`: documented `time_zone`.
  - `go.mod`/`go.sum`: church re-pinned from July's
    `v0.10.1-0.20260707023449-0dac7cbdec84` to
    `v0.10.1-0.20260913224810-a68100c2be20`, which includes `time_zone`.
    - `go mod tidy` pulled church's requirements: logger v1.2.20 → v1.3.0,
      serr v1.4.0, bytdb/pgwire v0.8.0, rweb, aws-sdk-go-v2 s3, and
      `golang.org/x/*`.
    - The `go` directive moved 1.23.0 → 1.26.1 (church requires it) and the
      `toolchain go1.23.2` line was dropped.
  - `.cats-todo/` (untracked) left out of the commit.

## ccswm standalone build: cause

- **The pin was stale.** `GOWORK=off go build` in ccswm used the July church
  pin, which lacks `config.Options.DB`, `db.DBTypes.BytDB` and
  `db.StartBytDBReplication`, all used by ccswm's `main.go`.
  - Confirmed pre-existing by building an untouched `git archive HEAD` copy:
    same errors.
- **The element bump added a second error.** Its `go get` raised serr to
  v1.3.0, which breaks logger v1.2.20
  (`serr.NewSerrNoContext` now returns `*SErr`).
  - Re-pinning church lifts logger to v1.3.0, which fixes it.
- **Docker was never affected.** It builds every site through `/src/go.work`
  (and deletes cema's nested `go.work`), so local church code and church's
  newer serr/logger win there.

## Verification

- church:
  - `go vet ./config/`, `go build ./...` and `go test ./...` pass.
  - `go test -v -run TimeZone ./config/`: 4/4 pass.
  - `gofmt -l config/` is clean.
- `go run ./test_scripts/roles_smoke` passes all 24 checks with element v0.7.0,
  including escaped donor markup.
- cema builds (workspace).
- ccswm:
  - builds standalone (`GOWORK=off`) and in the workspace
  - `GOWORK=off go vet .` passes
  - no test files
- **Not done:**
  - booting a site with `time_zone` set
  - a browser look at pages under element v0.7.0
  - a Docker image build (Docker still not started)

## Gotchas found

- **A `go get` in a site module can raise transitive deps** (serr here) past
  what the site's own pinned deps support. The workspace build hides this;
  check with `GOWORK=off go build`.
- **ccswm has no nested `go.work`; cema does.** A cema standalone build uses
  `cema/go.work`, which `use`s `../church`. cema's `go.mod` still pins June's
  church (`96257e048c20`), so `GOWORK=off` in cema would likely fail the same
  way ccswm did.
- **Placeholder zone:** the sample configs use `America/Chicago`. Real sites
  must set their own zone.
- **Editor noise, unchanged:** gopls "go.work requires go >= 1.26.1 (running go
  1.25.4)".

## Next

1. **New:** add `TIME_ZONE` to `deploy/k8s/sites/cema.yaml` and `ccswm.yaml`
   with each church's real IANA zone, and set `time_zone` in each site's real
   `cfg/options.yml`. Until then pods cut months (and show event times) in UTC.
2. **New:** boot a site with `time_zone` set (and once with a bad name) to
   confirm the startup log line and the fatal message.
3. **New, optional:** switch the giving module's `html.EscapeString(...)` +
   `.T()` to `.TE()` now that every site is on element v0.7.0. Keep
   `html.EscapeString` for attribute values: element only escapes `"` there.
4. **New, optional:** re-pin cema's `go.mod` to a current church commit (as
   done for ccswm) so `GOWORK=off` works there too. Do it on the branch that
   will be merged (see item 18).
5. **New, recurring:** after each church push that a site depends on, re-pin
   ccswm's church pseudo-version, or its standalone binary lags. Docker and
   workspace builds are unaffected.
6. Browser click-through of `/admin/giving` on a running site:
   - year navigation limits
   - month anchors
   - the unpaid-charge note
   - phone-width table scrolling
   - CSV download opened in Excel (accented names, formula guard, numeric
     amounts)
7. Check the giving report against real charge data on Postgres (e.g. a copy of
   a site DB). The dev DB has none. Compare the monthly totals with Stripe's
   dashboard for one month. Do it with `time_zone` set.
8. **Optional:** a summary-only CSV (month totals), if the treasurer wants one
   alongside the per-gift export.
9. Run `goose up` for the roles migration (`20260913160000_CreateRolesTables.sql`)
   on `church_development` and on any Postgres site. Also still pending: the
   `event_locations` migration.
10. Browser click-through of the role screens on a running site:
    - role form (matrix toggles, auto-Read)
    - user form Roles card and locked states
    - flash messages on refusals
    - publish switch disabled for an Editor
11. Filter the nav's Admin submenu by permission. Map known `/admin/…` URLs to
    their read permission in `menu.buildMenu`. DB-stored admin menus currently
    show dead links to unpermitted users.
12. Add "+" / delete visibility by permission to the other admin list modules
    (articles, sermons, events, pages, menus). Handlers already refuse; this is
    UI polish.
13. **Optional:** a guard against removing the last role-holder with
    `roles.update` (non-SuperAdmin lockout). SuperAdmin remains the recovery
    path.
14. **Optional:** consider an explicit permission (or SuperAdmin-only) for
    `/debug/*`. Today any admin can toggle process-wide debug state.
15. Decide whether chat/prayer-wall moderation should move from legacy
    `users.role` to a permission (e.g. `chat.moderate`). It was left on the
    legacy role to avoid touching the mobile contract.
16. Run `test_scripts/roles_smoke` in CI, or convert it to a Go test with a cfg
    fixture, so route wiring can't silently lose a `Require`. It also covers
    giving and CSV.
17. Add a `GOWORK=off go build` of each site to CI (or to the smoke routine) so
    a stale church pin is caught when it happens.
18. cema's `pg:` fix (`12049cf`), and now its element bump and `time_zone`
    sample, are only on `feature/site-themes`. Merge to cema's main branch
    before building cema from there.
19. Boot cema locally with no `db.type` against Postgres. Confirm there is no
    `bytdb serving` line and that pages render (recipe in `2026-0912-1718-…`,
    minus `DB_TYPE=postgres`).
20. Document the `db:` block (`type: postgres|bytdb`, `file`, `listen`) in
    `cema/cfg/options-sample.yml` and `ccswm/cfg/options-sample.yml`.
    (`time_zone` is now documented there; `db:` still isn't.)
21. Update docs that still describe bytdb as the default:
    - `deploy/k8s/README.md` §migration step 6
    - `ai_docs/fable_bytdb_k8s_readiness.md`
22. Start Docker and run `./deploy/deploy.sh images`, the one unproven fix from
    2026-08-01. If bimg/libvips 8.15 causes trouble, pin an older Alpine or use
    a pure-Go resizer.
23. Provision LKE + Object Storage and fill `deploy/backup.env`. Then run
    `./deploy/deploy.sh preflight infra`, point DNS, and run
    `./deploy/deploy.sh base seeds secrets images sites verify`.
24. Create `ccswm/cfg/options.yml` from the sample. It needs a real `pg:` block
    (or `db.type: bytdb`) and a real `time_zone`.
25. Move `resource/chimage` uploads onto IDrive e2 beside sermon media, retiring
    the `dist/img` volume mount.
26. From the readiness doc:
    - Call `CloseDB()` on SIGTERM in `ServeRWeb`.
    - Migrate `resource/dbbackup` off aws-sdk-go-v2 onto `replicate/s3`.
27. **Optional hardening:** `imagePullSecrets` (if the ghcr packages go private)
    and a www→apex redirect.
28. Live-check the 2026-09-12 flash changes on a running site (recipe in
    `2026-0912-1718-…`):
    - bad event coordinates or an expired csrf
    - empty menu `items` / page `modules`
    - referrer guard after a refused save
    - `token.txt` permissions on a fresh DB
29. Live-check the admin save flashes from `2026-0912-2125-…`:
    - expired csrf
    - blank article title
    - bad sermon date with audio chosen
    - password mismatch
    - stopped DB
30. Duplicate title on a new article/sermon (unique slug) still shows a generic
    "Error saving". Pre-check the slug on create as an InputError, working on
    both executors.
31. Seeds hygiene:
    - Replace the `resource/*/cfg/random_seeds.txt` fixtures with dummy seeds,
      including `resource/authz/cfg`.
    - Consider rotating the live `cfg/random_seeds.txt`.
32. **Optional:** sermon upload file handling (partial file on copy failure,
    orphan on DB failure, truncate-before-save). Fix with a temp file and rename
    after a successful upsert.
33. Keep typed values on a refused save: re-render the form from the posted
    presenter (events, articles, sermons, users) and/or wrap `UpsertEvent`'s
    writes in a transaction.
34. **Postgres coverage still missing:**
    - sermon create/import + audio
    - Stripe intent/history/webhook
    - `/chat/stream` SSE
    - image upload
    - giving report with data (item 7)
    - Optionally let `bytdb_wire_check` take a Postgres DSN.
35. Add `bytdb_to_pg` (and the `pg_to_bytdb` date fix), plus the Postgres smoke
    result, to `ai_docs/fable_bytdb_k8s_readiness.md` §7.
36. **Optional:** per-table content checksum in `bytdb_to_pg` beyond row counts.
37. **Non-goal (declined):** a config-threaded `*time.Location` in place of
    replacing `time.Local`. One startup assignment covers every existing and
    future call site.
38. **Non-goal (declined):** reusing `TZ` as the override name. The runtime
    already reads it, and an unset `time_zone` must keep honoring it.
39. **Non-goal (declined):** a `-truncate` flag on `bytdb_to_pg`. Refusing
    non-empty destinations protects live Postgres sites.
40. **Non-goal (no action):** page `opts.item_ids` saved as `[]` instead of
    `null`. Harmless, and identical on both backends.
41. **Non-goal (no action):** GET form/list/show handlers still return errors
    rather than flashes. Page definitions are static and only fail on a code
    bug.
42. **Non-goal (declined):** a transaction around role/permission writes.
    Delete-before-insert ordering bounds a partial failure to fewer grants, and
    the Executor seam has no `Begin`.
43. **Non-goal (declined):** a totals row in the giving CSV. It breaks sorting
    and filtering; a spreadsheet computes it.
44. **Non-goal (declined):** `GROUP BY`/`SUM` for the giving report. It isn't
    bytdb-portable, and the page lists every gift anyway.
