package idrive

// Media cache eviction against real databases, on both backends
// (internal/testdb), with IDrive e2 stood in for by objectExists. Covers the
// image index end to end (track, sweep, evict) and that the per-kind tables
// keep a sweep of one kind away from the other's rows.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/internal/testdb"
)

func TestImageCacheEviction(t *testing.T) {
	testdb.Each(t, func(t *testing.T) {
		dbH, err := db.Db()
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()

		// Four cached images, each standing for one branch of the sweep:
		//   idle    idle, on e2           -> evicted
		//   fresh   recently used, on e2  -> kept (not a candidate)
		//   orphan  idle, missing on e2   -> kept (may be the only copy)
		//   flaky   idle, e2 check errors -> kept (indeterminate)
		names := []string{"idle.png", "fresh.png", "orphan.png", "flaky.png"}
		local := map[string]string{}
		for _, n := range names {
			local[n] = filepath.Join(dir, n)
			if err := os.WriteFile(local[n], []byte("img"), 0644); err != nil {
				t.Fatal(err)
			}
			TrackImageAccess("images/"+n, local[n])
		}
		// Re-tracking is an upsert, not a second row.
		TrackImageAccess("images/fresh.png", local["fresh.png"])

		long := time.Now().Add(-5 * time.Hour)
		for _, n := range []string{"idle.png", "orphan.png", "flaky.png"} {
			if _, err := dbH.Exec(`UPDATE image_cache_access SET last_accessed_at = $1 WHERE rel_file_spec = $2`,
				long, "images/"+n); err != nil {
				t.Fatal(err)
			}
		}
		// An idle sermon row: the image sweep must leave it alone.
		if _, err := dbH.Exec(`INSERT INTO sermon_cache_access (created_at, last_accessed_at, rel_file_spec, local_file_spec)
			VALUES ($1, $1, $2, $3)`, long, "2024/old.mp3", filepath.Join(dir, "old.mp3")); err != nil {
			t.Fatal(err)
		}

		prevExists, prevTTL := objectExists, cacheIdleTTL
		t.Cleanup(func() { objectExists, cacheIdleTTL = prevExists, prevTTL })
		cacheIdleTTL = 4 * time.Hour
		var checked []string
		objectExists = func(key string) (bool, error) {
			checked = append(checked, key)
			switch key {
			case "images/orphan.png":
				return false, nil
			case "images/flaky.png":
				return false, errors.New("e2 unreachable")
			}
			return true, nil
		}

		imageCache.runCleanupPass()

		for _, key := range checked {
			if key == "images/fresh.png" || key == "2024/old.mp3" {
				t.Errorf("sweep checked %s, which is not an idle image", key)
			}
		}
		for n, wantFile := range map[string]bool{
			"idle.png": false, "fresh.png": true, "orphan.png": true, "flaky.png": true,
		} {
			_, statErr := os.Stat(local[n])
			if gotFile := statErr == nil; gotFile != wantFile {
				t.Errorf("%s: local file present = %v, want %v", n, gotFile, wantFile)
			}
			var rows int
			if err := dbH.QueryRow(`SELECT count(*) FROM image_cache_access WHERE rel_file_spec = $1`,
				"images/"+n).Scan(&rows); err != nil {
				t.Fatal(err)
			}
			if wantRows := map[bool]int{true: 1, false: 0}[wantFile]; rows != wantRows {
				t.Errorf("%s: tracking rows = %d, want %d", n, rows, wantRows)
			}
		}

		var sermonRows int
		if err := dbH.QueryRow(`SELECT count(*) FROM sermon_cache_access`).Scan(&sermonRows); err != nil {
			t.Fatal(err)
		}
		if sermonRows != 1 {
			t.Errorf("sermon_cache_access rows = %d after an image sweep, want 1", sermonRows)
		}
	})
}
