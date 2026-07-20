package db

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/rohanthewiz/bytdb"
	bsql "github.com/rohanthewiz/bytdb/sql"
)

// The property that matters for disaster recovery is not "BackupTo returned
// bytes" but "the bytes open as a working engine with the data intact" — a
// backup pipeline that produces unrestorable snapshots fails silently for
// months and is discovered on the worst possible day. So this test round-trips:
// write → snapshot via the exported BytDBBackupTo → open the snapshot as a
// fresh engine → read the rows back.
func TestBytDBBackupToRestorable(t *testing.T) {
	// Backend-not-bytdb guard first, while the package global is untouched.
	if _, err := BytDBBackupTo(&bytes.Buffer{}); err == nil {
		t.Fatal("BytDBBackupTo must error when no bytdb engine is active")
	}

	dir := t.TempDir()
	src, err := bytdb.Open(filepath.Join(dir, "src.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	sdb := bsql.New(src)
	if _, err = sdb.Exec(`CREATE TABLE snap_check (id bigint PRIMARY KEY, note text)`); err != nil {
		t.Fatal(err)
	}
	if _, err = sdb.Exec(`INSERT INTO snap_check (id, note) VALUES (1, 'alpha'), (2, 'beta')`); err != nil {
		t.Fatal(err)
	}

	// Point the package global at the scratch engine so the test exercises
	// the exported accessor exactly as resource/dbbackup calls it.
	prevEngine := bytdbEngine
	bytdbEngine = src
	defer func() { bytdbEngine = prevEngine }()

	var buf bytes.Buffer
	n, err := BytDBBackupTo(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(buf.Len()) {
		t.Fatalf("reported %d bytes but wrote %d", n, buf.Len())
	}

	snapPath := filepath.Join(dir, "snap.db")
	if err = os.WriteFile(snapPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	restored, err := bytdb.Open(snapPath)
	if err != nil {
		t.Fatalf("snapshot did not open as a valid engine: %v", err)
	}
	defer restored.Close()

	res, err := bsql.New(restored).Exec(`SELECT id, note FROM snap_check ORDER BY id`)
	if err != nil {
		t.Fatalf("query against restored snapshot failed: %v", err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("want 2 rows from restored snapshot, got %d", len(res.Rows))
	}
	if note, ok := res.Rows[1][1].(string); !ok || note != "beta" {
		t.Fatalf("restored data mismatch: row 2 note = %v", res.Rows[1][1])
	}
}
