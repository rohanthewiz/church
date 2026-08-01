package db

// Continuous WAL shipping for the embedded bytdb backend, plus the
// cold-start restore that makes a fresh volume self-heal.
//
// Two independent tiers protect a site's data, both landing in the same
// bucket under the site's Backup.Prefix:
//
//	<prefix>/wal/gen/<gen>/<start>-<end>.wlog   this file       — RPO ~5s
//	<prefix>/<UTC ts>/church.db                 resource/dbbackup — RPO 1h
//	<prefix>/latest/church.db                   resource/dbbackup — restore fallback
//
// They cannot collide: dbbackup's prune only deletes keys shaped exactly
// <prefix>/<timestamp>/church.db, and the replicator lists, writes, and
// prunes only under <prefix>/wal/. Keeping the snapshot tier alive is
// deliberate — it is the independent check on a WAL-chain bug, and it is
// still how a Postgres migration is delivered to a new site (upload the
// imported file to latest/, boot, done).
//
// Doctrine, settled upstream: a replica is RECOVERY, not HA. Nothing here
// fails over; a lost volume is recovered by assembling the newest complete
// generation onto a new one at boot.
//
//	engine ──LogState/ReadLogRange──▶ Replicator ──PUT──▶ object store
//	  ▲                                                        │
//	  └────────── replicate.Restore (boot, empty volume) ◀──────┘

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rohanthewiz/bytdb/replicate"
	"github.com/rohanthewiz/bytdb/replicate/s3"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/serr"
)

const (
	// walPrefix namespaces WAL chunks within the site's backup prefix. The
	// replicator appends its own "gen/..." beneath this.
	walPrefix = "wal"
	// snapshotLatestKey is dbbackup's rolling pointer, relative to the site
	// prefix — the fallback when no WAL generation exists yet.
	snapshotLatestKey = "latest/church.db"
	// restoreTimeout bounds the whole cold-start restore. Generous: it runs
	// once, on an empty volume, and a slow object store is a far better
	// outcome than giving up and bootstrapping an empty site.
	restoreTimeout = 2 * time.Minute
)

// Package state for the running replicator. Guarded by the same
// single-threaded startup/shutdown discipline as bytdbEngine — started once
// from main after InitDB, closed once from CloseDB.
var (
	bytdbReplicator     *replicate.Replicator
	bytdbReplicateEvery time.Duration // effective interval, for the status endpoint
)

// ReplicationStatus is the JSON-shaped view of replicate.Status served by
// GET /api/admin/db/replication. It exists because replicate.Status carries
// a bare `error` (which marshals to `{}`) and a time.Time whose usefulness
// to an operator is the derived lag, not the absolute stamp.
type ReplicationStatus struct {
	Generation string `json:"generation"` // "" before the first ship of this process
	Epoch      uint64 `json:"epoch"`      // log epoch the generation tracks
	Watermark  int64  `json:"watermark"`  // bytes shipped in this generation
	// LastShip is RFC3339 UTC, empty before the first successful ship.
	LastShip string `json:"last_ship"`
	// LagSeconds is now − LastShip: the single number worth alerting on.
	// nil (JSON null) before the first ship, where "lag" is undefined.
	LagSeconds *int64 `json:"lag_seconds"`
	// IntervalSeconds is the configured cadence, so a caller can judge
	// "lag >> interval" without also knowing the site's config.
	IntervalSeconds float64 `json:"interval_seconds"`
	// LastError is the most recent ship failure, cleared on success. nil is
	// the healthy state; a persistent value means shipping is stalled (bucket
	// outage, rotated creds) while the site itself keeps serving.
	LastError *string `json:"last_error"`
}

// replicationConfigured reports whether the object-store destination is
// fully specified. Note this is the *destination*, independent of the
// Replicate flag: cold-start restore needs the destination only, since
// pulling latest/church.db onto an empty volume is the job the initContainer
// used to do and must keep working on sites that never enable WAL shipping.
func replicationConfigured() bool {
	if config.Options == nil { // config not loaded (early boot, tests)
		return false
	}
	b := config.Options.Backup
	return b.Endpoint != "" && b.Bucket != "" && b.AccessKey != "" && b.SecretKey != ""
}

// backupStore builds the S3 client shared by shipping and restore, and
// returns the site's key prefix alongside it.
//
// This is replicate/s3 — the stdlib-only SigV4 client that ships with the
// feature — not the aws-sdk-v2 client resource/dbbackup carries. Two clients
// against one bucket is a wart; consolidating dbbackup onto this one (and
// dropping the AWS SDK dependency entirely) is a tracked follow-up, kept out
// of this change so a replication bug and an SDK swap can't be confused.
func backupStore() (store replicate.Storage, prefix string, err error) {
	b := config.Options.Backup

	endpoint := b.Endpoint
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint // same assumption dbbackup makes
	}
	region := b.Region
	if region == "" {
		region = "us-east-1" // S3-compatibles generally accept any non-empty region
	}

	cl, err := s3.New(s3.Config{
		Endpoint:  endpoint,
		Region:    region,
		Bucket:    b.Bucket,
		AccessKey: b.AccessKey,
		SecretKey: b.SecretKey,
		// Path-style addressing (the s3.Config default) is what Linode Object
		// Storage and MinIO both prefer; left implicit rather than set here so
		// upstream's default tracks whatever it decides is right.
	})
	if err != nil {
		return nil, "", serr.Wrap(err, "error building object-store client for replication",
			"endpoint", endpoint, "bucket", b.Bucket)
	}
	return cl, b.Prefix, nil
}

// replicateInterval parses the configured cadence, falling back to the
// upstream default on empty or unparseable input. Deliberately total: a
// typo in an env var should cost the operator a log line and the default
// RPO, not a site that refuses to boot.
func replicateInterval() time.Duration {
	raw := strings.TrimSpace(config.Options.Backup.ReplicateInterval)
	if raw == "" {
		return 5 * time.Second // upstream Options default; restated for the status endpoint
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		fmt.Println("bytdb replication: ignoring invalid replicate_interval", raw, "- using 5s")
		return 5 * time.Second
	}
	return d
}

// StartBytDBReplication starts the background ship loop. It is a no-op
// (nil error) when the backend is not bytdb, when the destination is
// unconfigured, or when replicate is not enabled — callers don't gate.
//
// Callers should log a returned error and keep serving: shipping is
// insurance, and a site that refuses to start because its bucket
// credentials are wrong has turned a degraded backup into an outage. The
// hourly snapshot tier still covers such a site.
func StartBytDBReplication() error {
	if bytdbEngine == nil { // Postgres fallback, or DB never initialized
		return nil
	}
	if bytdbReplicator != nil { // already running; idempotent
		return nil
	}
	if !replicationConfigured() || !config.Options.Backup.Replicate {
		return nil
	}

	store, prefix, err := backupStore()
	if err != nil {
		return err
	}

	interval := replicateInterval()
	// The engine satisfies replicate.Source (LogState / ReadLogRange). Chunk
	// size and retained-generation count keep their upstream defaults (8 MB,
	// 3): church databases are megabytes, so one chunk covers a whole
	// generation and three generations is a few tens of MB in the bucket.
	// They stay out of config until a site exists that needs them different.
	rep := replicate.New(bytdbEngine, store, replicate.Options{
		Prefix:   path.Join(prefix, walPrefix),
		Interval: interval,
		// fmt, not logger: db is imported by packages that initialize before
		// logging is configured. Ship failures arrive here — they are retried
		// on the next tick, so they are informational, not fatal.
		Logf: func(format string, args ...any) {
			fmt.Printf("bytdb replication: "+format+"\n", args...)
		},
	})
	rep.Start()

	bytdbReplicator = rep
	bytdbReplicateEvery = interval
	fmt.Println("bytdb replication started", "prefix:", path.Join(prefix, walPrefix),
		"interval:", interval)
	return nil
}

// BytDBReplicationStatus reports shipping progress; ok is false when no
// replicator is running (not bytdb, unconfigured, or not enabled).
func BytDBReplicationStatus() (out ReplicationStatus, ok bool) {
	if bytdbReplicator == nil {
		return out, false
	}
	st := bytdbReplicator.Status()

	out = ReplicationStatus{
		Generation:      st.Generation,
		Epoch:           st.Epoch,
		Watermark:       st.Watermark,
		IntervalSeconds: bytdbReplicateEvery.Seconds(),
	}
	if !st.LastShipTime.IsZero() {
		out.LastShip = st.LastShipTime.UTC().Format(time.RFC3339)
		lag := int64(time.Since(st.LastShipTime).Seconds())
		out.LagSeconds = &lag
	}
	if st.LastError != nil {
		msg := st.LastError.Error()
		out.LastError = &msg
	}
	return out, true
}

// closeBytDBReplication stops the ship loop and waits for its final flush.
// It must run BEFORE the engine closes — the flush reads from a live
// source. CloseDB enforces that ordering.
func closeBytDBReplication() {
	if bytdbReplicator == nil {
		return
	}
	if err := bytdbReplicator.Close(); err != nil {
		fmt.Println("bytdb replication: error on shutdown flush:", err.Error())
	}
	bytdbReplicator = nil
	bytdbReplicateEvery = 0
}

// restoreIfMissing brings a database file back onto an empty volume,
// preferring the freshest source. Returns a short description of what it
// did, for the boot log; an empty description means "nothing to restore",
// which is the legitimate fresh-site case and lets schema bootstrap run.
//
// Precedence — WAL first, because a shipped generation is seconds stale
// while latest/ is up to an hour stale:
//
//  1. newest complete WAL generation under <prefix>/wal/
//  2. on ErrNoReplica → <prefix>/latest/church.db (dbbackup's snapshot)
//  3. neither present → fresh site
//
// Any other error is returned and MUST abort startup. Bootstrapping an
// empty schema over a site that should have data produces the worst
// outcome available: a site serving 200s with nothing in it, quietly, while
// the replicator promptly ships that emptiness over a good replica. A
// crash-looping pod is loud, harmless (strategy: Recreate means nothing
// else is serving), and recoverable by hand.
//
// Written against replicate.Storage rather than a concrete client so the
// precedence matrix is unit-testable against an in-memory fake.
func restoreIfMissing(ctx context.Context, store replicate.Storage, prefix, destPath string) (string, error) {
	// Existence is the only guard needed. Restore never runs over a live
	// file, and both paths below land atomically (temp + fsync + rename), so
	// a crash mid-restore leaves no plausible-looking partial database.
	if _, err := os.Stat(destPath); err == nil {
		return "", nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", serr.Wrap(err, "could not check for existing database file", "file", destPath)
	}

	info, err := replicate.Restore(ctx, store, path.Join(prefix, walPrefix), destPath)
	switch {
	case err == nil:
		return fmt.Sprintf("restored from WAL generation %s (%d bytes, %d chunks)",
			info.Generation, info.Bytes, info.Chunks), nil

	case errors.Is(err, replicate.ErrNoReplica):
		// No generation has ever been shipped here: a site predating the
		// rollout, or a brand-new one. Fall through to the snapshot tier.

	default:
		// Includes ErrIncompleteReplica — the store certified a generation
		// complete and has since lost chunks. Refusing to silently fall back
		// to an hour-old snapshot is the point: that decision belongs to an
		// operator, who makes it by placing a file (see the manual-recovery
		// note in deploy/k8s/README.md), after which this function sees an
		// existing file and stands down.
		return "", serr.Wrap(err, "WAL replica restore failed", "prefix", path.Join(prefix, walPrefix))
	}

	snapKey := path.Join(prefix, snapshotLatestKey)
	// List-before-Get to tell "no snapshot" (a fresh site) from "store
	// unreachable" (must abort). The client surfaces a 404 as a generic
	// status error with no typed sentinel, and matching on message text
	// would be a trap; a listing is unambiguous and costs one request.
	keys, err := store.List(ctx, snapKey)
	if err != nil {
		return "", serr.Wrap(err, "could not check object store for a database snapshot", "key", snapKey)
	}
	found := false
	for _, k := range keys {
		if k == snapKey { // List is a prefix scan; require the exact key
			found = true
			break
		}
	}
	if !found {
		return "", nil // legitimately fresh site — schema bootstrap takes it from here
	}

	data, err := store.Get(ctx, snapKey)
	if err != nil {
		return "", serr.Wrap(err, "could not download database snapshot", "key", snapKey)
	}
	if err = writeFileAtomic(destPath, data); err != nil {
		return "", serr.Wrap(err, "could not write restored database snapshot", "file", destPath)
	}
	return fmt.Sprintf("restored from snapshot %s (%d bytes)", snapKey, len(data)), nil
}

// restoreBytDBIfMissing is the config-reading wrapper startBytDB calls.
// No-op when the destination is unconfigured — a local dev machine or a
// Postgres install must not be asked to reach an object store on boot.
func restoreBytDBIfMissing(destPath string) error {
	if !replicationConfigured() {
		return nil
	}
	store, prefix, err := backupStore()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), restoreTimeout)
	defer cancel()

	what, err := restoreIfMissing(ctx, store, prefix, destPath)
	if err != nil {
		return err
	}
	if what != "" {
		fmt.Println("bytdb:", what, "->", destPath)
	}
	return nil
}

// writeFileAtomic materializes data at destPath so that no reader — and no
// crash — can observe a partial file: write to a sibling temp file, fsync
// it, rename over the destination, then fsync the directory so the rename
// itself survives a power loss. Mirrors what replicate.Restore does
// internally, applied to the snapshot fallback path.
func writeFileAtomic(destPath string, data []byte) error {
	dir := filepath.Dir(destPath)
	f, err := os.CreateTemp(dir, ".church-restore-*")
	if err != nil {
		return serr.Wrap(err, "could not create temp file for restore", "dir", dir)
	}
	tmp := f.Name()
	// Best-effort cleanup: a no-op once the rename below has succeeded.
	defer func() {
		f.Close()
		os.Remove(tmp)
	}()

	if _, err = f.Write(data); err != nil {
		return serr.Wrap(err, "could not write temp restore file", "file", tmp)
	}
	if err = f.Sync(); err != nil {
		return serr.Wrap(err, "could not fsync temp restore file", "file", tmp)
	}
	if err = f.Close(); err != nil {
		return serr.Wrap(err, "could not close temp restore file", "file", tmp)
	}
	if err = os.Rename(tmp, destPath); err != nil {
		return serr.Wrap(err, "could not rename restore file into place", "file", destPath)
	}
	if d, dErr := os.Open(dir); dErr == nil {
		d.Sync()
		d.Close()
	}
	return nil
}
