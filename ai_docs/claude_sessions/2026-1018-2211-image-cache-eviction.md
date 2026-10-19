# Image cache eviction (article images on IDrive e2, sermon-style LRU)

Session: `1d3607ba-197e-4e73-90cb-d0d2229fb888`

## Ask

From the cats-todo backlog: "Image handling with idrive. This gets us ready
for k8s. We want to move images to Idrive, but Idrive cannot directly serve
images to the public. Do similar caching approach as we have for sermons."

## Finding

Most of it already existed (`44604b1`, session
`2026-0917-0259-nav-perms-drafts-shutdown-images-backup-ci`):
`resource/chimage/store.go` writes `dist/img/<name>` and PUTs `images/<name>`
to e2 in the background, and `GET /assets/img/:filename` serves the local file
or fetches from e2 and caches it. The gap compared with sermons was
**eviction**: the image cache only grew. On k8s it sits in the container's
writable layer (cema has no uploads mount), so it grew until the pod was
replaced.

## What changed (uncommitted until this wrap)

- **`core/idrive/sermon_cache.go`**: generalized into a `cacheIndex`
  (`table`, `label`) with methods `trackAccess`, `runCleanupPass`,
  `selectIdle`, `deleteRow`. `sermonCache` and `imageCache` are the two
  indexes; `StartCacheCleanup` now calls `runCacheCleanupPasses`, which sweeps
  both in sequence (sequential bounds load on the e2 endpoint). Exported API is
  unchanged (`TrackSermonAccess`, `DeleteCacheRowByRelSpec`,
  `LastAccessedByRelSpec` stay sermon-only). The unexported sermon helpers
  (`runCacheCleanupPass`, `selectIdleCachedSermons`, `deleteCachedSermonRow`)
  had no other callers and were folded into the methods; `cachedSermon` →
  `cachedFile`. New seam: `var objectExists = s3ops.ObjectExists`, because the
  s3ops client is a process-wide singleton a test can't repoint.
- **`core/idrive/image_cache.go`** (new): `imageCache` index on
  `image_cache_access`, `TrackImageAccess(relFileSpec, localFileSpec)`.
- **One table per media kind** rather than reusing `sermon_cache_access`:
  keeps the admin Sermon Cleanup listing sermon-only and lets either kind be
  dropped or retuned alone. Same shape, so SQL is shared with only the table
  name varying (names are constants, never input).
- **Schema**: `db/migrate/20261018120000_CreateImageCacheAccessTable.sql`
  (`if not exists`, same pattern as the sermon table), plus a tableDef in
  `db/bytdb_schema.go` (bytdb creates it on boot; `pg_to_bytdb` skips tables
  missing from the source; `bytdb_to_pg` expects a migrated destination).
- **`resource/chimage/store.go`**: `trackImage(name, localSpec, force)` is
  called on an upload that reached e2, on a fetch from e2 (both forced), and on
  a local hit. No-op with IDrive off (local is then the only copy).
  - Local hits are throttled per image to one write per
    `trackRefreshInterval` (5m) via a `sync.Map`, so a page view or crawler
    isn't a DB write per image request.
  - Forced tracks bypass the throttle: eviction may delete the row inside the
    window, and a file cached without a row would never be evicted.
  - A failed e2 upload is never tracked → never evicted (may be the only copy).
- **Gate**: image eviction rides the existing `idrive.auto_cleanup` flag and
  `cache_cleanup_interval` / `cache_idle_ttl`. Comments updated in
  `config/config.go` and `router.go`; `deploy/k8s/README.md` "Uploaded images"
  now says to set `auto_cleanup: true` on k8s and to run the migration.

## Tests

- `core/idrive/image_cache_test.go` `TestImageCacheEviction` (testdb.Each):
  idle+on e2 → evicted; fresh → not a candidate; missing on e2 → kept;
  e2 error → kept; idle sermon row untouched by the image sweep; re-track is
  an upsert.
- `resource/chimage/store_test.go` `TestLocalHitTracksAccess` (testdb.Each):
  a local hit writes a row; a repeat hit inside the interval doesn't
  (mutation check: interval 0 → test fails); a forced track refreshes.
- `go test ./...` green with
  `CHURCH_TEST_PG_DSN=postgres://devuser:***@localhost:5432/church_development?sslmode=disable`
  (Postgres subtests ran on throwaway DBs built from all migrations, including
  the new one). gofmt clean.
- Not done: no live e2 run, and `dbc migrate up` was not run on the dev DB.

## Not changed (raised)

The background e2 PUT on save: on k8s, a pod dying in the second or two
before the upload finishes loses the image. Raised as N-063 for a decision.

## Next

Closed: None. Declined: None. Raised: N-061, N-062, N-063, N-064.
Deferred: None. Promoted: None. Moved: None.
Updated: N-019. Full list: `ai_docs/todo/next-list.md`.
