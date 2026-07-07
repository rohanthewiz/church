# Session: Admin Sermon Cleanup + Flash Severity

- **Date:** 2026-06-27 10:27
- **Session ID:** `15b8791d-425c-4ccf-90ec-9b355e6d96d6`
- **Primary repo:** `github.com/rohanthewiz/church` (working dir `~/projs/go/church/church`), branch `rel/cema`
- **Sibling repos:** `~/projs/go/church/cema` (master), `~/projs/go/church/ccswm` (master)

> Continuation of the earlier session ([[2026-0626-2245-sermon-cache-eviction]]) which added the
> background LRU eviction + `sermon_cache_access` table. This session adds the admin-facing
> cleanup tool, a config gate, an IDrive-disabled guard, and flash severity styling.

---

## Goals (in order requested)

1. Admin "Sermon Cleanup" feature:
   - Walk the local sermons dir; for each sermon, if it is already on IDrive e2 with a
     case-sensitive matching name, under the correct year (year = parent dir), and non-zero
     size → delete the local copy.
   - Phase 2 (deferred): for sermons NOT yet on e2, find the DB row, copy to e2, update the
     row, verify, then delete local.
   - UI (rweb + serr + element): list eligible sermons grouped by year; select a batch or all;
     per row show IDrive e2 path, file size, and last accessed.
2. Wire interval + TTL into `config.IDrive` (done in prior session; confirmed here).
3. Add `idrive.auto_cleanup` config flag — background eviction OFF unless `true`.
4. Toast/flash when IDrive is not enabled (cleanup depends on IDrive).
5. Flash enhancement — distinct warn/error severities (was info-only).

---

## Key Architecture Facts (discovered)

- **Import cycle constraint:** `core/idrive` imports `resource/sermon` (for `GetSermon` /
  `GetRelAndLocalFileSpecs`), so `resource/sermon` CANNOT import `core/idrive`. Drove the
  decision to put the cleanup UI module in a NEW package `resource/sermoncleanup`.
- **Admin routing:** `router_rweb.go` admin group `ad := s.Group(config.AdminPrefix, ...guards)`;
  prefix is `/admin`. Sermon routes live there.
- **Page/chrome system:** Pages are arrangements of registered modules. `template.Page` provides
  banner/menu/flash/footer chrome. Admin screens are modules + a `page.*` presenter rendered via
  `basectlr.RenderPageListRWeb`. Modules registered in `page/modules_registry.go`.
- **Sermon file layout:** `config.Options.IDrive.LocalSermonsDir/<year>/<file>`; the IDrive
  object key is `year/filename` (also the value matched by HeadObject — case-sensitive).
- **rweb form limitation:** `FormValue` returns only the FIRST value per field (no multi-value
  getter, no exposed `*http.Request`). Batch checkbox selection therefore posts as a single
  newline-delimited hidden field populated by JS at submit time.
- **CSRF:** Redis-backed tokens via `app.GenerateFormToken()` / `app.VerifyFormToken()`.
- **Flash:** `flash.Flash{Info,Warn,Error}`; `Render()` already emits `flash-info/flash-warn/
  flash-error` classes. `app.RedirectRWeb` previously only set `Info`.
- **CSS lives in the SITE repos** (`cema`, `ccswm`): Stylus source `styles/styl/_styl/_flash.styl`
  → compiled `dist/css/app.css` (served via `StaticFiles("/assets/", "dist", 1)`). The `church`
  module itself ships no app CSS.

---

## Changes — `church` repo

### Admin Sermon Cleanup feature (commit `a43ccc6`)
- **`core/s3ops/s3ops.go`**: added `ObjectInfo(key) (exists, size, err)` via `HeadObject`
  (ContentLength); added `BucketName()`; `ObjectExists` now delegates to `ObjectInfo`.
  Conservative contract: `(false,0,err)` means "unknown" → never delete.
- **`core/idrive/sermon_cache.go`**: added `LastAccessedByRelSpec()` (whole-table map) and
  `DeleteCacheRowByRelSpec()`.
- **`core/idrive/sermon_cleanup_service.go`** (new): `LocalSermonInfo`, `ScanEligibleForDeletion()`
  (walks dir, bounded-concurrency cloud checks via worker pool of 8, eligibility = cloud exists
  && size>0), `DeleteVerifiedLocalCopies()` (re-verifies each against e2 before deleting; path-
  traversal guard `safeLocalFileSpec` confining to LocalSermonsDir; clears tracking row).
- **`resource/sermoncleanup/`** (new package): `module_sermon_cleanup.go` (UI module + CSRF +
  grouped-by-year table with per-row/per-year/global select-all + batch delete) and `assets.go`
  (plain-JS, no jQuery dependency, + scoped CSS). Const `CleanupActionPath = "/admin/sermons/cleanup"`.
- **`page/sermon_pages.go`**: `AdminSermonCleanup()` page presenter.
- **`page/modules_registry.go`**: registered the module; excluded `"cleanup"` from
  `availableModuleTypes` so it never appears in the dynamic page builder.
- **`sermon_controller/sermon_controller_rweb.go`**: `AdminSermonCleanupRWeb` (GET) +
  `AdminSermonCleanupRunRWeb` (POST; splits the newline-delimited `selected_specs`, CSRF check,
  calls the service, flashes a summary).
- **`router_rweb.go`**: `GET`/`POST /admin/sermons/cleanup`.

### `auto_cleanup` config gate (commit `e0bd24b`)
- **`config/config.go`**: added `AutoCleanup bool yaml:"auto_cleanup"` (default false) to `IDrive`.
- **`router_rweb.go`**: background loop now starts only when `IDrive.Enabled && IDrive.AutoCleanup`.
  The admin tool is unaffected by the flag.

### IDrive-disabled guard (commit `1c39f4d`)
- Both cleanup handlers short-circuit with a flash + redirect to `/admin/sermons` when
  `config.Options.IDrive.Enabled` is false (POST guard prevents stale/forged deletes).

### Flash severity (commit `cb1b6e3`)
- **`app/application_controller_rweb.go`**: `FlashSeverity` enum; `RedirectRWebSev`; wrappers
  `RedirectRWebWarn` / `RedirectRWebError`; `RedirectRWeb` kept (info) for the 28 existing callers.
- The IDrive-disabled cleanup notices now use `RedirectRWebWarn`.

---

## Changes — `cema` + `ccswm` repos

- **`cfg/options-sample.yml`**: documented `auto_cleanup: false` and the (commented) cache tuning
  keys. (cema commits `070b7d5`→`fa16426`; ccswm `9ca4ddc`→`1c6bf16`, plus prior doc commits.)
- **Flash CSS** (cema `cabb795`, ccswm `b7390e3`): `#flash` container now transparent; per-severity
  message colors — info green (unchanged), warn amber (`#ffd24d`), error red (`#f5a3a3`). Edited
  BOTH the Stylus source `_flash.styl` and the compiled `dist/css/app.css`.

---

## Eligibility Logic (why it satisfies the requirements)

The IDrive key is built as `year/filename`. A successful `HeadObject` on that exact key:
- inherently confirms the **case-exact name** (S3 keys are case-sensitive),
- inherently confirms the **correct year** (it is part of the key path),
- returns **ContentLength** → the **non-zero size** check.

So `CloudExists && CloudSize > 0` covers all three stated conditions. Deletion re-runs this check
immediately before removing each local file, so a stale browser selection cannot delete an only-copy.

---

## Verification

- `go build ./...` and `go vet` clean after each step (church module).
- Sibling repos: hand-edited Stylus + dist CSS kept in sync (no npm rebuild run).
- NOT verified live against a real IDrive bucket (offered; user did not request).

---

## Deployment / Follow-ups

- **Run the migration** from the prior session before relying on the "last accessed" column:
  `cd db/migrate && goose postgres "user=devuser password=<REDACTED> dbname=church_development sslmode=disable" up`
  (Cleanup tool still works without it — last-accessed shows "—".)
- **Admin menu link to add:** `/admin/sermons/cleanup` (label e.g. "Sermon Cleanup").
- `cema`/`ccswm` consume the PUBLISHED `church` module (no local `replace`), so they pick up the
  Go changes only after `church` is tagged/published.
- `church` work is on `rel/cema` (not `master`) — open a PR when ready to merge.
- **Phase 2** (upload-then-delete for files not yet on e2) intentionally NOT implemented; service
  has seams for it.
- Ops notes: GitHub Dependabot flags 8 pre-existing vulns on `church` default branch; Bitbucket app
  password for `ccswm` goes inactive 2026-06-09 (switch to API token).

---

## All commits this session

| Repo | Branch | Commits |
|------|--------|---------|
| church | rel/cema | `a43ccc6` (cleanup feature), `e0bd24b` (auto_cleanup gate), `1c39f4d` (idrive-disabled guard), `cb1b6e3` (flash severity) — all pushed |
| cema | master | `fa16426` (auto_cleanup doc), `cabb795` (flash CSS) — pushed |
| ccswm | master | `1c6bf16` (auto_cleanup doc), `b7390e3` (flash CSS) — pushed |
