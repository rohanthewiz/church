package chimage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rohanthewiz/church/config"
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
	s.Get("/assets/img/:filename", ServeImageRWeb)

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
