package db

// End-to-end exercise of the replication story through the production entry
// points — InitDB, StartBytDBReplication, CloseDB — against a fake object
// store spoken to over real HTTP by the real replicate/s3 client.
//
// The unit tests in replicate_test.go drive restoreIfMissing directly against
// an in-memory Storage, which skips everything between the config block and a
// bucket: endpoint handling, SigV4 signing, path-style URLs, the ListObjectsV2
// XML shape, and the actual key strings the two tiers agree on. Those are
// precisely the parts that fail silently in production — a mis-signed request
// or an off-by-one prefix looks like "no replica found", which the restore
// path is designed to treat as a legitimately fresh site.
//
// The scenario is the disaster it exists for:
//
//	boot on an empty volume → write rows → ship → lose the volume → boot again
//
// and the assertion is that the rows come back.

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rohanthewiz/church/config"
)

// fakeS3 implements just enough of the S3 REST API, path-style, for the
// replicator and restore: PUT/GET/DELETE on an object and ListObjectsV2 on
// the bucket. Standing up MinIO would test the same surface at the cost of a
// dependency the CI box may not have.
type fakeS3 struct {
	mu     sync.Mutex
	bucket string
	objs   map[string][]byte
	// unsigned counts requests that arrived without a SigV4 signature. A fake
	// that ignored auth would happily serve an unsigned client, hiding a
	// signing regression until the first real deploy.
	unsigned int
}

func newFakeS3(bucket string) *fakeS3 {
	return &fakeS3{bucket: bucket, objs: map[string][]byte{}}
}

func (f *fakeS3) keys(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for k := range f.objs {
		if strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	return out
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256") ||
		r.Header.Get("x-amz-content-sha256") == "" {
		f.mu.Lock()
		f.unsigned++
		f.mu.Unlock()
		http.Error(w, "unsigned request", http.StatusForbidden)
		return
	}

	// Path-style addressing: /<bucket>[/<key>]
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p != f.bucket && !strings.HasPrefix(p, f.bucket+"/") {
		http.Error(w, "no such bucket", http.StatusNotFound)
		return
	}
	key := strings.TrimPrefix(strings.TrimPrefix(p, f.bucket), "/")

	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && key == "" && r.URL.Query().Get("list-type") == "2":
		f.list(w, r.URL.Query().Get("prefix"))

	case r.Method == http.MethodGet:
		data, ok := f.objs[key]
		if !ok {
			http.Error(w, "no such key", http.StatusNotFound)
			return
		}
		w.Write(data)

	case r.Method == http.MethodPut:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		f.objs[key] = data
		w.WriteHeader(http.StatusOK)

	case r.Method == http.MethodDelete:
		delete(f.objs, key) // deleting a missing key is not an error
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "unsupported", http.StatusMethodNotAllowed)
	}
}

// s3Contents / s3ListResult mirror the ListObjectsV2 response envelope the
// client decodes (it reads Contents>Key, IsTruncated, NextContinuationToken).
type s3Contents struct {
	Key string `xml:"Key"`
}

type s3ListResult struct {
	XMLName     xml.Name     `xml:"ListBucketResult"`
	IsTruncated bool         `xml:"IsTruncated"`
	Contents    []s3Contents `xml:"Contents"`
}

// list emits a ListBucketResult. Unpaginated on purpose: a church site's
// generations never approach 1000 keys, and the client's paging loop is
// upstream's to test.
func (f *fakeS3) list(w http.ResponseWriter, prefix string) {
	res := s3ListResult{}
	for k := range f.objs {
		if strings.HasPrefix(k, prefix) {
			res.Contents = append(res.Contents, s3Contents{Key: k})
		}
	}
	// S3 guarantees ascending byte order, and the restore verifies a
	// generation's chunk chain from the listing alone — an unsorted fake
	// would let a broken contiguity check pass.
	sort.Slice(res.Contents, func(i, j int) bool {
		return res.Contents[i].Key < res.Contents[j].Key
	})

	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(res)
}

func TestReplicationRoundTripThroughObjectStore(t *testing.T) {
	const (
		bucket = "church-backups"
		prefix = "itest"
	)
	store := newFakeS3(bucket)
	srv := httptest.NewServer(store)
	defer srv.Close()

	// Save every package global the production path mutates; this test drives
	// InitDB/CloseDB for real, so leaking any of it would poison sibling tests.
	prevOpts, prevHandle, prevDBOpts := config.Options, dbHandle, dbOpts
	prevEngine, prevServer, prevAddr := bytdbEngine, bytdbServer, bytdbAddr
	t.Cleanup(func() {
		CloseDB()
		config.Options, dbHandle, dbOpts = prevOpts, prevHandle, prevDBOpts
		bytdbEngine, bytdbServer, bytdbAddr = prevEngine, prevServer, prevAddr
	})

	cfg := &config.EnvConfig{}
	cfg.Backup.Endpoint = srv.URL // httptest gives a full http:// URL
	cfg.Backup.Region = "us-east-1"
	cfg.Backup.Bucket = bucket
	cfg.Backup.AccessKey = "test-access"
	cfg.Backup.SecretKey = "test-secret"
	cfg.Backup.Prefix = prefix
	cfg.Backup.Replicate = true
	// Fast cadence so the test drives the real Run loop (not ShipNow) without
	// a long wait — the loop is what production uses, so it is what is tested.
	cfg.Backup.ReplicateInterval = "100ms"
	config.Options = cfg

	dataDir := t.TempDir()
	dataFile := filepath.Join(dataDir, "church.db")
	opts := DBOpts{DBType: DBTypes.BytDB, File: dataFile, Listen: "127.0.0.1:0"}

	// ---- First boot: empty volume, empty bucket. Must bootstrap, not fail.
	if err := InitDB(opts); err != nil {
		t.Fatalf("first boot on an empty volume failed: %v", err)
	}
	handle, err := Db()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = handle.Exec(`CREATE TABLE repl_check (id bigint PRIMARY KEY, note text)`); err != nil {
		t.Fatal(err)
	}
	if _, err = handle.Exec(`INSERT INTO repl_check (id, note) VALUES (1, 'shipped')`); err != nil {
		t.Fatal(err)
	}

	if err = StartBytDBReplication(); err != nil {
		t.Fatalf("starting replication failed: %v", err)
	}

	// Wait for the ship loop to report a successful upload.
	var status ReplicationStatus
	deadline := time.Now().Add(20 * time.Second)
	for {
		var running bool
		status, running = BytDBReplicationStatus()
		if !running {
			t.Fatal("replicator stopped running")
		}
		if status.LastError != nil {
			t.Fatalf("ship failed: %s", *status.LastError)
		}
		if status.LastShip != "" && status.Watermark > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no successful ship within 20s (status %+v)", status)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if status.Generation == "" {
		t.Error("a shipped generation must have an id")
	}
	if status.IntervalSeconds != 0.1 {
		t.Errorf("status should echo the configured interval, got %v", status.IntervalSeconds)
	}
	if status.LagSeconds == nil {
		t.Error("lag must be reported once something has shipped")
	}
	if store.unsigned > 0 {
		t.Errorf("%d requests reached the store unsigned", store.unsigned)
	}

	// Keys must land under the WAL namespace only — the snapshot tier's
	// prune walks the same prefix and must never see these.
	walKeys := store.keys(prefix + "/wal/gen/")
	if len(walKeys) == 0 {
		t.Fatalf("nothing shipped under %s/wal/gen/ (bucket holds %v)", prefix, store.keys(""))
	}
	for _, k := range store.keys("") {
		if !strings.HasPrefix(k, prefix+"/wal/") {
			t.Errorf("replication wrote outside its namespace: %s", k)
		}
	}
	if !hasSuffixAny(walKeys, "manifest.json") {
		t.Error("a caught-up generation must be certified with a manifest, else restore " +
			"falls back to best-effort selection")
	}

	// ---- Lose the volume: clean shutdown, then the data file is gone.
	CloseDB()
	if bytdbReplicator != nil {
		t.Fatal("CloseDB must stop the replicator (its final flush needs a live engine)")
	}
	if err = os.Remove(dataFile); err != nil {
		t.Fatal(err)
	}

	// ---- Second boot on a fresh volume: must self-heal from the replica.
	if err = InitDB(opts); err != nil {
		t.Fatalf("boot on a fresh volume failed to restore: %v", err)
	}
	if _, err = os.Stat(dataFile); err != nil {
		t.Fatalf("restore did not produce a data file: %v", err)
	}

	handle, err = Db()
	if err != nil {
		t.Fatal(err)
	}
	var note string
	if err = handle.QueryRow(`SELECT note FROM repl_check WHERE id = 1`).Scan(&note); err != nil {
		t.Fatalf("row written before the volume was lost did not survive: %v", err)
	}
	if note != "shipped" {
		t.Fatalf("restored the wrong data: note = %q", note)
	}
}

func hasSuffixAny(keys []string, suffix string) bool {
	for _, k := range keys {
		if strings.HasSuffix(k, suffix) {
			return true
		}
	}
	return false
}
