package chimage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/internal/testdb"
	"github.com/rohanthewiz/rweb"
)

// inTempDir runs the test from an empty directory, since the image directory
// is CWD-relative (dist/img/), and with IDrive disabled.
func inTempDir(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	prevOpts := config.Options
	config.Options = &config.EnvConfig{}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
		config.Options = prevOpts
	})
}

func TestValidName(t *testing.T) {
	for name, want := range map[string]bool{
		"photo.png.abc123.png": true,
		"my pic.jpg.1f.jpg":    true,
		"":                     false,
		".hidden.png":          false,
		"../x.png":             false,
		"a/b.png":              false,
		`a\b.png`:              false,
	} {
		if got := validName(name); got != want {
			t.Errorf("validName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestStoreAndServeLocal(t *testing.T) {
	inTempDir(t)
	png := []byte("\x89PNG\r\n\x1a\nfake-image-bytes")

	if err := storeImage("photo.png.abc.png", png); err != nil {
		t.Fatalf("storeImage: %v", err)
	}
	if err := storeImage("my pic.png.def.png", png); err != nil {
		t.Fatalf("storeImage with a space: %v", err)
	}
	if err := storeImage("../escape.png", png); err == nil {
		t.Error("storeImage accepted a path")
	}
	if leftovers, _ := filepath.Glob(filepath.Join(LocalDir, ".*")); len(leftovers) != 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}

	s := rweb.NewServer(rweb.ServerOptions{})
	s.Get("/assets/img/:filename", ServeImage)

	r := s.Request("GET", "/assets/img/photo.png.abc.png", nil, nil)
	if r.Status() != 200 || string(r.Body()) != string(png) {
		t.Errorf("stored image: status %d body %q", r.Status(), r.Body())
	}
	if ct := r.Header("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	if cc := r.Header("Cache-Control"); cc == "" {
		t.Error("no Cache-Control on an immutable image")
	}

	// Stored article HTML links names as typed; browsers percent-encode spaces
	r = s.Request("GET", "/assets/img/my%20pic.png.def.png", nil, nil)
	if r.Status() != 200 {
		t.Errorf("image with a space: status %d", r.Status())
	}

	// A non-image extension is never served as a page
	if err := storeImage("x.html.1.html", []byte("<script>alert(1)</script>")); err != nil {
		t.Fatal(err)
	}
	r = s.Request("GET", "/assets/img/x.html.1.html", nil, nil)
	if ct := r.Header("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("html-named upload served as %q", ct)
	}

	// IDrive disabled: a missing image is a plain 404
	r = s.Request("GET", "/assets/img/missing.png", nil, nil)
	if r.Status() != 404 {
		t.Errorf("missing image: status %d, want 404", r.Status())
	}
	r = s.Request("GET", "/assets/img/.hidden", nil, nil)
	if r.Status() != 404 {
		t.Errorf("hidden name: status %d, want 404", r.Status())
	}
}

// TestLocalHitTracksAccess checks the hookup into the media cache eviction:
// with IDrive enabled, serving a cached image records its access (so the sweep
// can later evict it), repeat hits inside trackRefreshInterval skip the
// database, and a forced track (a fresh write) always records. Files are put
// in place directly, since storeImage and a cache miss would talk to e2.
func TestLocalHitTracksAccess(t *testing.T) {
	testdb.Each(t, func(t *testing.T) {
		inTempDir(t)
		config.Options.IDrive.Enabled = true
		lastTracked.Clear()
		t.Cleanup(lastTracked.Clear)
		dbH, err := db.Db()
		if err != nil {
			t.Fatal(err)
		}

		const name = "pic.png.ab12.png"
		if err := writeFileAtomic(filepath.Join(LocalDir, name), []byte("img")); err != nil {
			t.Fatal(err)
		}
		// lastAccess returns the tracked time once a row exists, waiting out
		// the background upsert; ok is false when none appears in time.
		lastAccess := func() (at time.Time, ok bool) {
			t.Helper()
			for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
				err := dbH.QueryRow(`SELECT last_accessed_at FROM image_cache_access WHERE rel_file_spec = $1`,
					ObjectKey(name)).Scan(&at)
				if err == nil {
					return at, true
				}
			}
			return at, false
		}

		if _, found, err := loadImage(name); err != nil || !found {
			t.Fatalf("loadImage: found %v, err %v", found, err)
		}
		if _, ok := lastAccess(); !ok {
			t.Fatal("a local hit recorded no image_cache_access row")
		}

		// Age the row, then hit again inside the refresh interval: the row
		// must keep its aged timestamp.
		aged := time.Now().Add(-time.Hour).Truncate(time.Second)
		if _, err := dbH.Exec(`UPDATE image_cache_access SET last_accessed_at = $1 WHERE rel_file_spec = $2`,
			aged, ObjectKey(name)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := loadImage(name); err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond) // a write, if one were (wrongly) made, lands by now
		if at, _ := lastAccess(); !at.Equal(aged) {
			t.Errorf("a repeat hit inside the refresh interval rewrote last_accessed_at: %v, want %v", at, aged)
		}

		// A forced track (what a fresh upload or e2 fetch does) always writes.
		trackImage(name, filepath.Join(LocalDir, name), true)
		for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			if at, _ := lastAccess(); at.After(aged) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("a forced track did not refresh last_accessed_at")
			}
		}
	})
}
