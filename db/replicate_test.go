package db

// Cold-start restore is the one piece of the replication story that only runs
// on the worst day — a lost volume — so it gets the strongest tests in the
// package. The property under test throughout is not "Restore returned no
// error" but "the file it produced opens as an engine with the right rows in
// it": a restore path that silently produces an empty or stale database looks
// identical to a healthy one until someone goes looking for their data.
//
// The precedence matrix these pin (see restoreIfMissing):
//
//	dest file exists      → untouched, no store access at all
//	WAL generation shipped→ WAL wins, even with a snapshot also present
//	only latest/ snapshot → snapshot fallback
//	neither               → fresh site, no file, no error (bootstrap runs)
//	store unreachable     → error, no file (startup must abort)

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rohanthewiz/bytdb"
	"github.com/rohanthewiz/bytdb/replicate"
	bsql "github.com/rohanthewiz/bytdb/sql"
	"github.com/rohanthewiz/church/config"
)

const testPrefix = "testsite"

// memStore is an in-memory replicate.Storage. Object stores are the one
// dependency these tests genuinely need, and faking the four-method interface
// is cheaper and more deterministic than standing up MinIO — the real client
// is exercised by the deploy runbook instead.
type memStore struct {
	mu   sync.Mutex
	objs map[string][]byte
	// failAll, when set, makes every operation fail. Models an unreachable
	// store / bad credentials, which must abort startup rather than fall
	// through to an empty-schema bootstrap.
	failAll error
}

func newMemStore() *memStore { return &memStore{objs: map[string][]byte{}} }

func (m *memStore) Put(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failAll != nil {
		return m.failAll
	}
	// Copy: the replicator reuses one staging buffer across chunks, so
	// retaining its slice would alias later writes.
	m.objs[key] = append([]byte(nil), data...)
	return nil
}

func (m *memStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failAll != nil {
		return nil, m.failAll
	}
	data, ok := m.objs[key]
	if !ok {
		return nil, errors.New("memstore: no such key: " + key)
	}
	return append([]byte(nil), data...), nil
}

func (m *memStore) List(_ context.Context, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failAll != nil {
		return nil, m.failAll
	}
	var keys []string
	for k := range m.objs {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys) // the lexicographic order S3 List guarantees
	return keys, nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failAll != nil {
		return m.failAll
	}
	delete(m.objs, key) // deleting a missing key is not an error
	return nil
}

// seedEngine builds a scratch database carrying one identifying note value,
// so a restored file can be traced back to which source produced it.
func seedEngine(t *testing.T, file, note string) *bytdb.Engine {
	t.Helper()
	eng, err := bytdb.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	sdb := bsql.New(eng)
	if _, err = sdb.Exec(`CREATE TABLE restore_check (id bigint PRIMARY KEY, note text)`); err != nil {
		t.Fatal(err)
	}
	if _, err = sdb.Exec(`INSERT INTO restore_check (id, note) VALUES (1, '` + note + `')`); err != nil {
		t.Fatal(err)
	}
	return eng
}

// noteIn opens file as an engine and returns the seeded note — the assertion
// that the restore produced a working database, not just bytes on disk.
func noteIn(t *testing.T, file string) string {
	t.Helper()
	eng, err := bytdb.Open(file)
	if err != nil {
		t.Fatalf("restored file did not open as an engine: %v", err)
	}
	defer eng.Close()

	res, err := bsql.New(eng).Exec(`SELECT note FROM restore_check WHERE id = 1`)
	if err != nil {
		t.Fatalf("query against restored file failed: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("want 1 row in restored file, got %d", len(res.Rows))
	}
	note, ok := res.Rows[0][0].(string)
	if !ok {
		t.Fatalf("unexpected note type %T", res.Rows[0][0])
	}
	return note
}

// shipWAL seeds an engine and ships one full generation into store, leaving a
// manifested (certified-complete) generation — what a live site produces
// within a tick of starting.
func shipWAL(t *testing.T, store replicate.Storage, note string) {
	t.Helper()
	eng := seedEngine(t, filepath.Join(t.TempDir(), "src.db"), note)
	defer eng.Close()

	rep := replicate.New(eng, store, replicate.Options{
		Prefix: path.Join(testPrefix, walPrefix),
		Logf:   func(string, ...any) {}, // silence: ship progress is not test output
	})
	if err := rep.ShipNow(context.Background()); err != nil {
		t.Fatalf("shipping a generation failed: %v", err)
	}
}

// putSnapshot writes a dbbackup-shaped latest/church.db into store.
func putSnapshot(t *testing.T, store replicate.Storage, note string) {
	t.Helper()
	eng := seedEngine(t, filepath.Join(t.TempDir(), "snapsrc.db"), note)
	defer eng.Close()

	var buf bytes.Buffer
	if _, err := eng.BackupTo(&buf); err != nil {
		t.Fatal(err)
	}
	key := path.Join(testPrefix, snapshotLatestKey)
	if err := store.Put(context.Background(), key, buf.Bytes()); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreLeavesExistingFileAlone(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "church.db")
	if err := os.WriteFile(dest, []byte("live database, do not touch"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A store that fails everything: if restoreIfMissing touches it at all,
	// the test fails. The existence check must short-circuit before any I/O —
	// a running site must never be at the mercy of the object store.
	store := newMemStore()
	store.failAll = errors.New("store must not be contacted")

	what, err := restoreIfMissing(context.Background(), store, testPrefix, dest)
	if err != nil {
		t.Fatalf("restore over an existing file must be a silent no-op, got %v", err)
	}
	if what != "" {
		t.Fatalf("nothing should have been restored, got %q", what)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "live database, do not touch" {
		t.Fatalf("existing database file was modified: %q (%v)", data, err)
	}
}

// The reason WAL-first exists: a shipped generation is seconds stale while
// latest/ can be an hour old. With both present the newer one must win, and
// the note values make "which source did we take" directly observable.
func TestRestorePrefersWALOverSnapshot(t *testing.T) {
	store := newMemStore()
	putSnapshot(t, store, "stale-snapshot")
	shipWAL(t, store, "fresh-wal")

	dest := filepath.Join(t.TempDir(), "church.db")
	what, err := restoreIfMissing(context.Background(), store, testPrefix, dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(what, "WAL generation") {
		t.Fatalf("want a WAL restore, got %q", what)
	}
	if note := noteIn(t, dest); note != "fresh-wal" {
		t.Fatalf("restored the wrong source: note = %q, want fresh-wal", note)
	}
}

// A site deployed before the WAL rollout — or one migrated from Postgres,
// whose imported file was uploaded to latest/ — has no generations. The
// snapshot fallback is what keeps that runbook working unchanged.
func TestRestoreFallsBackToSnapshot(t *testing.T) {
	store := newMemStore()
	putSnapshot(t, store, "only-snapshot")

	dest := filepath.Join(t.TempDir(), "church.db")
	what, err := restoreIfMissing(context.Background(), store, testPrefix, dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(what, "snapshot") {
		t.Fatalf("want a snapshot restore, got %q", what)
	}
	if note := noteIn(t, dest); note != "only-snapshot" {
		t.Fatalf("restored the wrong source: note = %q, want only-snapshot", note)
	}
}

// A genuinely new site: empty bucket, empty volume. This must NOT error —
// erroring here would make every first deploy crash-loop — and must not
// create a file, so schema bootstrap runs on a clean engine.
func TestRestoreFreshSiteIsNoOp(t *testing.T) {
	store := newMemStore()
	dest := filepath.Join(t.TempDir(), "church.db")

	what, err := restoreIfMissing(context.Background(), store, testPrefix, dest)
	if err != nil {
		t.Fatalf("a fresh site must restore nothing without erroring, got %v", err)
	}
	if what != "" {
		t.Fatalf("nothing should have been restored, got %q", what)
	}
	if _, err = os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("no file should have been created, stat gave %v", err)
	}
}

// An unreachable store is indistinguishable, from inside the process, from a
// store that holds this site's only surviving copy. Aborting is the only safe
// reading: bootstrapping an empty schema here would serve a blank site and
// then replicate that blankness over the good replica.
func TestRestoreAbortsWhenStoreUnreachable(t *testing.T) {
	store := newMemStore()
	store.failAll = errors.New("connection refused")

	dest := filepath.Join(t.TempDir(), "church.db")
	if _, err := restoreIfMissing(context.Background(), store, testPrefix, dest); err == nil {
		t.Fatal("an unreachable object store must abort startup, not fall through to bootstrap")
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a failed restore must leave no file behind, stat gave %v", err)
	}
}

// ErrIncompleteReplica means the store certified a generation complete and
// has since lost chunks. Falling back to the hour-old snapshot would be a
// silent, unannounced rollback; the operator decides that, by placing a file.
func TestRestoreAbortsOnIncompleteReplica(t *testing.T) {
	store := newMemStore()
	putSnapshot(t, store, "snapshot-that-must-not-win")
	shipWAL(t, store, "complete-generation")

	// Delete a shipped chunk while leaving the manifest that certifies it.
	keys, err := store.List(context.Background(), path.Join(testPrefix, walPrefix))
	if err != nil {
		t.Fatal(err)
	}
	deleted := false
	for _, k := range keys {
		if strings.HasSuffix(k, ".wlog") {
			if err = store.Delete(context.Background(), k); err != nil {
				t.Fatal(err)
			}
			deleted = true
			break
		}
	}
	if !deleted {
		t.Fatal("no .wlog chunk found to delete - shipping layout changed?")
	}

	dest := filepath.Join(t.TempDir(), "church.db")
	_, err = restoreIfMissing(context.Background(), store, testPrefix, dest)
	if err == nil {
		t.Fatal("a generation with missing chunks must abort, not silently fall back to the snapshot")
	}
	if !errors.Is(err, replicate.ErrIncompleteReplica) {
		t.Fatalf("want ErrIncompleteReplica, got %v", err)
	}
	if _, statErr := os.Stat(dest); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("a failed restore must leave no file behind, stat gave %v", statErr)
	}
}

// Gating: the destination alone enables cold-start restore (a site that never
// turns on WAL shipping still needs latest/ pulled onto an empty volume — the
// job the initContainer used to do). Partial credentials count as unconfigured
// so a half-filled secret can't produce confusing runtime failures.
func TestReplicationConfiguredGating(t *testing.T) {
	prev := config.Options
	t.Cleanup(func() { config.Options = prev })

	config.Options = nil
	if replicationConfigured() {
		t.Fatal("must be unconfigured when config has not loaded")
	}

	cfg := &config.EnvConfig{}
	config.Options = cfg
	if replicationConfigured() {
		t.Fatal("must be unconfigured with an empty backup block")
	}

	cfg.Backup.Endpoint = "us-east-1.example.com"
	cfg.Backup.Bucket = "church-backups"
	if replicationConfigured() {
		t.Fatal("must be unconfigured without credentials")
	}

	cfg.Backup.AccessKey = "ak"
	cfg.Backup.SecretKey = "sk"
	if !replicationConfigured() {
		t.Fatal("must be configured once endpoint, bucket and both keys are set")
	}
}

// A bad interval must cost the default RPO, never a boot failure: the value
// arrives from a k8s env var, where a typo is a deploy-time slip, not a
// reason to take a site offline.
func TestReplicateIntervalParsing(t *testing.T) {
	prev := config.Options
	t.Cleanup(func() { config.Options = prev })

	cfg := &config.EnvConfig{}
	config.Options = cfg

	cases := []struct {
		raw  string
		want time.Duration
	}{
		{"", 5 * time.Second},
		{"  ", 5 * time.Second},
		{"15s", 15 * time.Second},
		{"1m", time.Minute},
		{"nonsense", 5 * time.Second},
		{"-3s", 5 * time.Second},
		{"0s", 5 * time.Second},
	}
	for _, tc := range cases {
		cfg.Backup.ReplicateInterval = tc.raw
		if got := replicateInterval(); got != tc.want {
			t.Errorf("replicate_interval %q: got %v, want %v", tc.raw, got, tc.want)
		}
	}
}

// StartBytDBReplication is called unconditionally from every site's main, so
// its no-op paths are load-bearing: a Postgres install, a dev machine with no
// bucket, or a site that simply has not opted in must all get silence.
func TestStartReplicationNoOpsWithoutOptIn(t *testing.T) {
	prevOpts, prevEngine := config.Options, bytdbEngine
	t.Cleanup(func() {
		config.Options, bytdbEngine = prevOpts, prevEngine
		bytdbReplicator = nil
	})

	cfg := &config.EnvConfig{}
	cfg.Backup.Endpoint = "us-east-1.example.com"
	cfg.Backup.Bucket = "church-backups"
	cfg.Backup.AccessKey = "ak"
	cfg.Backup.SecretKey = "sk"
	cfg.Backup.Replicate = true
	config.Options = cfg

	// No engine: the Postgres fallback path.
	bytdbEngine = nil
	if err := StartBytDBReplication(); err != nil {
		t.Fatalf("must be a no-op without a bytdb engine, got %v", err)
	}
	if _, running := BytDBReplicationStatus(); running {
		t.Fatal("no replicator should be running without a bytdb engine")
	}

	// Engine present but the flag off: adopting a bytdb version that ships
	// the feature must not start writing to anybody's bucket by itself.
	eng := seedEngine(t, filepath.Join(t.TempDir(), "src.db"), "n/a")
	defer eng.Close()
	bytdbEngine = eng
	cfg.Backup.Replicate = false
	if err := StartBytDBReplication(); err != nil {
		t.Fatalf("must be a no-op when replicate is off, got %v", err)
	}
	if _, running := BytDBReplicationStatus(); running {
		t.Fatal("no replicator should be running with replicate off")
	}

	// Flag on but the destination unconfigured: same silence, no panic.
	cfg.Backup.Replicate = true
	cfg.Backup.Bucket = ""
	if err := StartBytDBReplication(); err != nil {
		t.Fatalf("must be a no-op with an unconfigured destination, got %v", err)
	}
	if _, running := BytDBReplicationStatus(); running {
		t.Fatal("no replicator should be running without a destination")
	}
}
