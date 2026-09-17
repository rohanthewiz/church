// Package dbbackup implements consistent database snapshots to S3-compatible
// object storage: the SNAPSHOT tier, RPO = the trigger cadence (the k8s
// CronJob runs hourly).
//
// Continuous WAL shipping (db/replicate.go, RPO ~5s) is the primary tier and
// does not replace this one. Full snapshots are the independent check on a
// WAL-chain bug — they are produced by different code from a different
// mechanism (Engine.BackupTo, not log byte ranges) —
// and latest/ remains how a migrated Postgres database is delivered to a new
// site.
//
// Key layout in the bucket, per site (config Backup.Prefix):
//
//	<prefix>/<UTC timestamp>/church.db   immutable history, pruned to Retain
//	<prefix>/latest/church.db            rolling pointer; the cold-start
//	                                     restore's fallback when no WAL
//	                                     generation exists yet
//	<prefix>/wal/gen/...                 the other tier's keys; prune below
//	                                     cannot match them, by shape
//
// The snapshot is uploaded to its timestamped key, then uploaded again to
// latest/. S3 PUT is atomic per key, so latest/ is always a complete
// database, never a partial write. (A server-side copy would save the second
// upload, but the client below has no copy call, and a church database is a
// few MB.)
//
// Storage goes through db.BackupStore: the same stdlib-only replicate/s3
// client, bucket and credentials as WAL shipping. The snapshot tier's
// independence from WAL shipping comes from its mechanism (Engine.BackupTo,
// a full consistent copy) rather than from a second S3 client. It does not
// share core/s3ops's client: that one is bound to the IDrive media bucket,
// and backups deliberately use separate credentials (media creds must not
// read database contents).
package dbbackup

import (
	"bytes"
	"context"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/rohanthewiz/bytdb/replicate"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const (
	dbFileName = "church.db"
	latestDir  = "latest"
	// tsFormat sorts lexicographically == chronologically, which is what
	// lets pruning sort keys as plain strings. Always UTC — sites in
	// different timezones share buckets, and DST would reorder local times.
	tsFormat      = "20060102-150405Z"
	defaultRetain = 72 // three days of hourly snapshots
	// runTimeout bounds one run's object-store calls. The CronJob fires
	// hourly, so a hung upload must give up well before the next one.
	runTimeout = 10 * time.Minute
)

// Result is what a successful run reports back through the API — enough for
// the CronJob log line to be a meaningful audit record on its own.
type Result struct {
	Key       string `json:"key"`        // timestamped object written
	LatestKey string `json:"latest_key"` // rolling pointer updated
	Bytes     int64  `json:"bytes"`      // snapshot size
	Pruned    int    `json:"pruned"`     // old snapshots deleted this run
	DurMillis int64  `json:"dur_millis"`
}

// Configured reports whether the destination is fully specified. The token
// is checked separately by the API layer (it gates the endpoint, not the
// storage destination).
func Configured() bool {
	if config.Options == nil { // config not loaded (early boot, tests)
		return false
	}
	b := config.Options.Backup
	return b.Endpoint != "" && b.Bucket != "" && b.AccessKey != "" && b.SecretKey != ""
}

// Run takes one snapshot: stream from the engine, upload, roll latest/,
// prune history. Returns a Result for the API response.
//
// The store is built per run rather than cached: backups run hourly, so setup
// cost is irrelevant, and a fresh client picks up rotated credentials without
// a pod restart.
func Run() (res Result, err error) {
	started := time.Now()

	// The whole database rides through memory — church DBs are MBs, and an
	// in-memory buffer keeps the pod filesystem out of the picture (the only
	// writable volume is the live DB's own). If a site's DB ever grows to
	// where this matters, switch to a spooled temp file + multipart upload.
	var buf bytes.Buffer
	n, err := db.BytDBBackupTo(&buf)
	if err != nil {
		return res, serr.Wrap(err, "error snapshotting database")
	}

	store, prefix, err := db.BackupStore()
	if err != nil {
		return res, serr.Wrap(err, "error building backup store")
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	res, err = upload(ctx, store, prefix, config.Options.Backup.Retain, buf.Bytes(), started)
	if err != nil {
		return res, err
	}
	res.Bytes = n
	res.DurMillis = time.Since(started).Milliseconds()
	return res, nil
}

// upload writes one snapshot and its latest/ pointer, then prunes. Split from
// Run so the key layout and pruning are testable against an in-memory store.
func upload(ctx context.Context, store replicate.Storage, prefix string, retain int, data []byte,
	now time.Time) (res Result, err error) {
	// path (not filepath): S3 keys always use forward slashes.
	res.Key = path.Join(prefix, now.UTC().Format(tsFormat), dbFileName)
	res.LatestKey = path.Join(prefix, latestDir, dbFileName)
	res.Bytes = int64(len(data))

	if err = store.Put(ctx, res.Key, data); err != nil {
		return res, serr.Wrap(err, "error uploading backup", "key", res.Key)
	}
	// Written only after the timestamped copy succeeded, so latest/ never
	// points past the history.
	if err = store.Put(ctx, res.LatestKey, data); err != nil {
		return res, serr.Wrap(err, "error updating latest backup pointer", "key", res.LatestKey)
	}

	// Prune failures are logged, not returned: the snapshot itself succeeded,
	// and failing the run would make the CronJob retry a full backup just to
	// re-attempt deletes. Over-retention is the safe failure direction.
	pruned, pruneErr := prune(ctx, store, prefix, retain)
	if pruneErr != nil {
		logger.LogErr(pruneErr, "backup succeeded but pruning old snapshots failed")
	}
	res.Pruned = pruned
	return res, nil
}

// prune deletes timestamped snapshots beyond the retention count, oldest
// first. latest/ and the WAL tier's wal/... keys never qualify: only
// <prefix>/<timestamp>/church.db matches.
func prune(ctx context.Context, store replicate.Storage, prefix string, retain int) (deleted int, err error) {
	if retain <= 0 {
		retain = defaultRetain
	}

	listPrefix := prefix
	if listPrefix != "" && !strings.HasSuffix(listPrefix, "/") {
		listPrefix += "/"
	}

	// The store pages internally and returns every key under the prefix
	keys, err := store.List(ctx, listPrefix)
	if err != nil {
		return 0, serr.Wrap(err, "error listing backups for pruning")
	}
	var tsKeys []string
	for _, key := range keys {
		// Expect <prefix>/<timestamp>/church.db; skip anything else
		// (latest/, wal/, foreign objects sharing the prefix).
		rel := strings.TrimPrefix(key, listPrefix)
		parts := strings.Split(rel, "/")
		if len(parts) != 2 || parts[1] != dbFileName {
			continue
		}
		if _, tErr := time.Parse(tsFormat, parts[0]); tErr != nil {
			continue
		}
		tsKeys = append(tsKeys, key)
	}

	if len(tsKeys) <= retain {
		return 0, nil
	}
	sort.Strings(tsKeys) // timestamp format sorts chronologically
	for _, key := range tsKeys[:len(tsKeys)-retain] {
		if delErr := store.Delete(ctx, key); delErr != nil {
			// Report partial progress; the next run retries the remainder.
			return deleted, serr.Wrap(delErr, "error deleting old backup", "key", key)
		}
		deleted++
	}
	return deleted, nil
}
