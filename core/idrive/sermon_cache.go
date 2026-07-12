package idrive

// sermon_cache.go implements an LRU-style eviction strategy for sermon files that
// have been downloaded from IDrive e2 and cached on the local disk.
//
// Lifecycle:
//  1. GetSermon serves a sermon (either a fresh download or a local cache hit) and
//     calls TrackSermonAccess, which upserts a row in `sermon_cache_access` and
//     bumps last_accessed_at to now.
//  2. StartCacheCleanup launches a background ticker (hourly). Each pass selects
//     rows whose last_accessed_at is older than the idle window (4h) and, for each,
//     verifies the object still exists on IDrive e2 before deleting the local copy.
//
// Design notes:
//   - We use raw database/sql via db.Db() rather than a SQLBoiler model so that
//     adding this table does not require regenerating the ORM models.
//   - The cloud-existence check is a hard precondition for deletion: if we cannot
//     confirm the object exists on IDrive e2 (missing, or an indeterminate error),
//     we keep the local copy. The local file may be the only surviving copy.

import (
	"os"
	"strconv"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/core/s3ops"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const (
	// defaultCacheCleanupInterval is how often the background eviction pass runs
	// when config.IDrive.CacheCleanupInterval is unset/invalid.
	defaultCacheCleanupInterval = time.Hour

	// defaultCacheIdleTTL is the idle window before a cached sermon (not accessed
	// within it) becomes eligible for eviction, when config.IDrive.CacheIdleTTL
	// is unset/invalid.
	defaultCacheIdleTTL = 4 * time.Hour
)

// cleanupInterval and cacheIdleTTL hold the effective, resolved durations. They
// are populated once at StartCacheCleanup from config (with the const fallbacks)
// so the cleanup pass does not re-parse config strings on every run.
var (
	cleanupInterval time.Duration
	cacheIdleTTL    time.Duration
)

// resolveDuration parses a Go duration string from config, falling back to the
// given default for empty or malformed values (logging the latter so a typo in
// the YAML is visible rather than silently ignored).
func resolveDuration(raw string, def time.Duration, field string) time.Duration {
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		logger.LogErr(serr.Wrap(err, "cache-cleanup: invalid duration in config; using default",
			"field", field, "value", raw, "default", def.String()))
		return def
	}
	return d
}

// TrackSermonAccess records (or refreshes) the access time of a locally-cached
// sermon. It is safe to call on every serve; an upsert keeps a single row per
// object and pushes last_accessed_at forward so hot files are never evicted.
//
// Errors are logged rather than returned: access tracking is best-effort and must
// never interfere with serving the sermon to the user. Callers typically invoke
// this in a goroutine.
func TrackSermonAccess(relFileSpec, localFileSpec string) {
	dbh, err := db.Db()
	if err != nil {
		logger.LogErr(serr.Wrap(err, "cache-track: could not obtain DB handle"))
		return
	}

	now := time.Now()
	// ON CONFLICT on the unique rel_file_spec turns this into an upsert:
	// insert on first download, refresh last_accessed_at on subsequent accesses.
	const q = `
		INSERT INTO sermon_cache_access (created_at, last_accessed_at, rel_file_spec, local_file_spec)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (rel_file_spec)
		DO UPDATE SET last_accessed_at = EXCLUDED.last_accessed_at,
		              local_file_spec  = EXCLUDED.local_file_spec`
	if _, err = dbh.Exec(q, now, now, relFileSpec, localFileSpec); err != nil {
		logger.LogErr(serr.Wrap(err, "cache-track: failed to upsert sermon access record",
			"relFileSpec", relFileSpec))
	}
}

// StartCacheCleanup launches the background eviction loop in its own goroutine.
// Call once at server startup (after the S3 client is configured). The loop runs
// for the life of the process.
func StartCacheCleanup() {
	// Resolve effective durations from config once, with the const defaults as fallback.
	cleanupInterval = resolveDuration(config.Options.IDrive.CacheCleanupInterval, defaultCacheCleanupInterval, "idrive.cache_cleanup_interval")
	cacheIdleTTL = resolveDuration(config.Options.IDrive.CacheIdleTTL, defaultCacheIdleTTL, "idrive.cache_idle_ttl")

	logger.Info("Starting sermon cache cleanup",
		"interval", cleanupInterval.String(), "idleTTL", cacheIdleTTL.String())

	go func() {
		// Recover so a panic in a cleanup pass can never take down the process.
		defer func() {
			if r := recover(); r != nil {
				logger.Info("Recovered from panic in sermon cache cleanup loop",
					"panic", r)
			}
		}()

		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		// Run an initial pass shortly after startup rather than waiting a full
		// interval, so a process that restarts often still gets a chance to clean up.
		time.Sleep(time.Minute)
		runCacheCleanupPass()

		for range ticker.C {
			runCacheCleanupPass()
		}
	}()
}

// cachedSermon is an in-memory view of a sermon_cache_access row needed for eviction.
type cachedSermon struct {
	id            int64
	relFileSpec   string
	localFileSpec string
}

// runCacheCleanupPass performs a single eviction sweep: find idle cached sermons,
// confirm each exists on IDrive e2, then delete the local copy and its tracking row.
func runCacheCleanupPass() {
	dbh, err := db.Db()
	if err != nil {
		logger.LogErr(serr.Wrap(err, "cache-cleanup: could not obtain DB handle"))
		return
	}

	cutoff := time.Now().Add(-cacheIdleTTL)

	candidates, err := selectIdleCachedSermons(dbh, cutoff)
	if err != nil {
		logger.LogErr(serr.Wrap(err, "cache-cleanup: failed to query idle cached sermons"))
		return
	}
	if len(candidates) == 0 {
		return
	}

	logger.Info("Sermon cache cleanup pass", "candidates", len(candidates),
		"idleSince", cutoff.Format(time.RFC3339))

	var evicted int
	for _, c := range candidates {
		// Hard precondition: never delete the local copy unless the cloud copy is
		// confirmed present. ObjectExists returns (false, nil) only for a definitive
		// "not found"; any indeterminate error comes back as (false, err) and we skip.
		exists, err := s3ops.ObjectExists(c.relFileSpec)
		if err != nil {
			logger.LogErr(serr.Wrap(err, "cache-cleanup: could not verify IDrive copy; keeping local file",
				"relFileSpec", c.relFileSpec))
			continue
		}
		if !exists {
			logger.Info("cache-cleanup: IDrive copy missing; keeping local file (possibly only copy)",
				"relFileSpec", c.relFileSpec)
			continue
		}

		// Remove the local cached file. A not-exist error is benign (file already
		// gone) and we still want to clear the stale tracking row below.
		if err = os.Remove(c.localFileSpec); err != nil && !os.IsNotExist(err) {
			logger.LogErr(serr.Wrap(err, "cache-cleanup: failed to delete local sermon copy",
				"localFileSpec", c.localFileSpec))
			continue
		}

		// Drop the tracking row; if the sermon is requested again it will be
		// re-downloaded and re-tracked from scratch.
		if err = deleteCachedSermonRow(dbh, c.id); err != nil {
			logger.LogErr(serr.Wrap(err, "cache-cleanup: deleted local file but failed to remove tracking row",
				"id", strconv.FormatInt(c.id, 10), "relFileSpec", c.relFileSpec))
			continue
		}

		evicted++
		logger.Info("cache-cleanup: evicted local sermon copy", "relFileSpec", c.relFileSpec,
			"localFileSpec", c.localFileSpec)
	}

	logger.Info("Sermon cache cleanup pass complete", "candidates", len(candidates), "evicted", evicted)
}

// selectIdleCachedSermons returns cached sermons not accessed since the cutoff.
func selectIdleCachedSermons(exec db.Executor, cutoff time.Time) (sermons []cachedSermon, err error) {
	const q = `
		SELECT id, rel_file_spec, local_file_spec
		FROM sermon_cache_access
		WHERE last_accessed_at < $1
		ORDER BY last_accessed_at ASC`

	rows, err := exec.Query(q, cutoff)
	if err != nil {
		return nil, serr.Wrap(err)
	}
	defer rows.Close()

	for rows.Next() {
		var c cachedSermon
		if err = rows.Scan(&c.id, &c.relFileSpec, &c.localFileSpec); err != nil {
			return nil, serr.Wrap(err)
		}
		sermons = append(sermons, c)
	}
	if err = rows.Err(); err != nil {
		return nil, serr.Wrap(err)
	}
	return sermons, nil
}

// deleteCachedSermonRow removes a single tracking row by primary key.
func deleteCachedSermonRow(exec db.Executor, id int64) error {
	_, err := exec.Exec(`DELETE FROM sermon_cache_access WHERE id = $1`, id)
	if err != nil {
		return serr.Wrap(err)
	}
	return nil
}

// DeleteCacheRowByRelSpec removes the tracking row for a single cached object by
// its rel_file_spec (IDrive key). Used when a local copy is deleted outside the
// background sweep (e.g. the admin Sermon Cleanup tool). A no-match is not an error.
func DeleteCacheRowByRelSpec(exec db.Executor, relFileSpec string) error {
	if _, err := exec.Exec(`DELETE FROM sermon_cache_access WHERE rel_file_spec = $1`, relFileSpec); err != nil {
		return serr.Wrap(err, "failed to delete cache row", "relFileSpec", relFileSpec)
	}
	return nil
}

// LastAccessedByRelSpec returns a map of rel_file_spec -> last_accessed_at for
// every tracked cached sermon. The table holds at most one row per cached file and
// is small (only currently-cached files), so loading it whole is cheaper than
// issuing a per-file query while building the admin cleanup listing.
func LastAccessedByRelSpec(exec db.Executor) (map[string]time.Time, error) {
	rows, err := exec.Query(`SELECT rel_file_spec, last_accessed_at FROM sermon_cache_access`)
	if err != nil {
		return nil, serr.Wrap(err)
	}
	defer rows.Close()

	out := make(map[string]time.Time)
	for rows.Next() {
		var spec string
		var at time.Time
		if err = rows.Scan(&spec, &at); err != nil {
			return nil, serr.Wrap(err)
		}
		out[spec] = at
	}
	if err = rows.Err(); err != nil {
		return nil, serr.Wrap(err)
	}
	return out, nil
}
