# Session: Sermon Local-Cache Tracking & LRU Eviction

- **Date:** 2026-06-26 22:45
- **Session ID:** `15b8791d-425c-4ccf-90ec-9b355e6d96d6`
- **Branch:** `rel/cema`
- **Module:** `github.com/rohanthewiz/church` (working dir `~/projs/go/church/church`)

---

## Goal

When a single sermon page references an audio file, it is downloaded from IDrive e2
(S3-compatible) and cached locally. Add:

1. A table tracking freshly-downloaded sermons and their last access time.
2. A background process (hourly) that scans the table.
3. Eviction: if a cached copy has been idle > 4h, delete the local copy — **but only
   after verifying a copy exists on IDrive e2**.

Follow-up requests in the same session:
- Wire the scan interval and idle TTL into `config.IDrive`.

---

## Key Findings (existing architecture)

- **Download/serve path:** Route `GET /sermon-audio/:year/:filename` in `router_rweb.go`
  calls `idrive.GetSermon(year, filename)`.
- **`idrive.GetSermon`** (`core/idrive/client.go`): checks local disk via `os.Stat`; on
  miss, downloads from IDrive via `s3ops.GetFileFromS3` and caches locally in a goroutine.
  Had a pre-existing `TODO - LRU cleanup` note.
- **S3 client:** `core/s3ops/s3ops.go`, using `aws-sdk-go-v2`. Had `GetObject`, `PutObject`,
  `ListObjectsV2`, `CopyObject`, `DeleteObject` — **no** existence check (HeadObject).
- **Local path:** `config.Options.IDrive.LocalSermonsDir` joined with `year/filename`
  via `resource/sermon/helpers.go: GetRelAndLocalFileSpecs`. The relative spec doubles
  as the IDrive object key.
- **DB:** PostgreSQL at runtime (`lib/pq`), SQLBoiler-generated models in `models/`.
  Raw SQL is reachable via `db.Db() (*sql.DB, error)`. Migrations use **goose** in
  `db/migrate/`.
- **Startup:** `church.ServeRWeb()` runs `idrive.InitClient()` then starts the rweb
  server. No prior background tickers existed (only ad-hoc upload goroutines).
- **Config:** YAML loaded in `config/config.go` into `Options EnvConfig`; `IDrive` is a
  nested struct. Sample files: `cema/cfg/options-sample.yml`, `ccswm/cfg/options-sample.yml`.

---

## Design Decisions

- **LRU on every serve, not just fresh downloads.** `GetSermon` upserts an access record
  on both fresh download *and* local cache hit, bumping `last_accessed_at`. This keeps
  hot files resident; only genuinely idle files get evicted. (Matches the old TODO intent.)
- **Raw `database/sql`** (via `db.Db()`) instead of a SQLBoiler model, so adding the table
  does **not** require regenerating the ORM.
- **Conservative cloud-existence contract.** New `s3ops.ObjectExists(key)`:
  - `(true, nil)` → present
  - `(false, nil)` → definitive 404 / NotFound / NoSuchKey only
  - `(false, err)` → indeterminate (network/auth); caller must **keep** the local copy.
- **Eviction precondition is hard:** never delete a local file unless the IDrive copy is
  positively confirmed. If IDrive copy is missing, keep local (possibly only copy) + log.
- **Best-effort tracking:** access-tracking failures are logged, never propagated to the
  serving path; tracking runs in a goroutine.
- **Config-driven tuning with safe fallbacks:** interval/TTL are Go duration strings in
  `config.IDrive`; empty → default, malformed → default + logged error. Resolved once at
  startup into package vars (no per-pass re-parsing).

---

## Changes Made

### New: `db/migrate/20260626120000_CreateSermonCacheAccessTable.sql`
Goose migration creating `sermon_cache_access`:
- `id bigserial PK`, `created_at`, `last_accessed_at` (not null),
  `rel_file_spec text` (IDrive key, **unique**), `local_file_spec text`.
- Unique index on `rel_file_spec` (drives the upsert); index on `last_accessed_at`
  (drives the cleanup scan).

### New: `core/idrive/sermon_cache.go`
- `TrackSermonAccess(relFileSpec, localFileSpec)` — upsert via
  `INSERT ... ON CONFLICT (rel_file_spec) DO UPDATE SET last_accessed_at = ...`.
- `StartCacheCleanup()` — resolves durations from config, logs config, launches a
  panic-recovered goroutine: initial pass ~1 min after startup, then `time.Ticker`.
- `runCacheCleanupPass()` — selects rows idle past cutoff; per row: `ObjectExists`
  check → `os.Remove` local file → `deleteCachedSermonRow`. Skips (keeps file) on any
  uncertainty.
- Helpers: `selectIdleCachedSermons`, `deleteCachedSermonRow`, `resolveDuration`.
- Defaults: `defaultCacheCleanupInterval = 1h`, `defaultCacheIdleTTL = 4h`.

### Edited: `core/s3ops/s3ops.go`
- Added imports: `errors`, `aws-sdk-go-v2/service/s3/types`, `aws/smithy-go`.
- Added `ObjectExists(key)` using `HeadObject`, handling both `*types.NotFound` and
  generic smithy `APIError` codes (`NotFound`/`NoSuchKey`/`404`) for IDrive e2 compat.

### Edited: `core/idrive/client.go`
- `GetSermon` now calls `go TrackSermonAccess(...)` on both the fresh-download and the
  local-cache-hit branches.

### Edited: `router_rweb.go`
- After `idrive.InitClient()`, added (gated on `config.Options.IDrive.Enabled`):
  `idrive.StartCacheCleanup()`.

### Edited: `config/config.go`
- Added to the `IDrive` struct:
  - `CacheCleanupInterval string` (`yaml:"cache_cleanup_interval"`)
  - `CacheIdleTTL string` (`yaml:"cache_idle_ttl"`)

### Edited: `cema/cfg/options-sample.yml`, `ccswm/cfg/options-sample.yml`
- Documented both keys as commented lines showing defaults (`1h` / `4h`).
  (Live `cema/cfg/options.yml` left untouched — defaults match prior behavior.)

---

## Verification

- `go mod tidy` (promoted `smithy-go` to a direct dependency).
- `go build ./...` → clean.
- `go vet ./core/idrive/... ./core/s3ops/... ./config/... .` → clean.

---

## Deployment Notes / Follow-ups

- **Run the migration before deploying:**
  ```bash
  cd db/migrate
  goose postgres "user=devuser password=<REDACTED> dbname=church_development sslmode=disable" up
  ```
  Until the table exists, tracking upserts just log an error; serving is unaffected.
- **`cema` / `ccswm` consume the *published* `church` module** (no local `replace`
  directive), so they pick up these changes only after the `church` module is
  tagged/published.
- Optional future work (offered, not done): evict empty `year/` directories after file
  removal; add explicit interval/TTL values to the live `cema/cfg/options.yml`.

---

## Override Example

```yaml
idrive:
    cache_cleanup_interval: 5m   # faster scan, e.g. for local testing
    cache_idle_ttl: 15m
```
