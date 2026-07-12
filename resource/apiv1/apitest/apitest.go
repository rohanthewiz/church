// Package apitest is shared plumbing for /api/v1 contract tests.
//
// The tests exist to freeze the JSON contract consumed by the church_mobile
// Flutter app (see church_mobile/lib/src/api/api_client.dart and
// lib/src/models/*): the app hard-casts ids and iterates arrays without null
// checks, so any silent drift in key names, envelope shape, or the
// {"error": ...} failure shape becomes a runtime crash on phones. Handlers
// are exercised through a real rweb router via Server.Request (in-process,
// no listener), with the database stubbed by go-sqlmock through
// db.SetHandleForTesting — the query layer reaches for a package-global
// handle, so the global is the only seam available until queries accept an
// executor parameter.
//
// This package deliberately imports no resource packages, so any resource's
// tests may use it without an import cycle.
package apitest

import (
	"encoding/json"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/rweb"
)

// NewServer returns a routable in-process server. Tests register the same
// handlers the production router does (router_rweb.go) and drive them with
// Server.Request — no port, no goroutine.
func NewServer() *rweb.Server {
	return rweb.NewServer(rweb.ServerOptions{})
}

// MockDB swaps the process-global DB handle for a sqlmock and returns the
// mock for setting expectations. The swap is process-global state, so tests
// using it must not run in parallel within a package (Go runs same-package
// tests sequentially by default).
func MockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	dbH, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("could not create sqlmock: %v", err)
	}
	db.SetHandleForTesting(dbH)
	t.Cleanup(func() { dbH.Close() })
	return mock
}

// GetJSON performs a synthetic GET and decodes the body, failing the test if
// the response is not JSON — the core /api/v1 guarantee is that every
// response, success or failure, parses as JSON.
func GetJSON(t *testing.T, s *rweb.Server, url string) (status int, doc map[string]any) {
	t.Helper()
	resp := s.Request("GET", url, nil, nil)
	if ct := resp.Header("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("GET %s: Content-Type = %q, want application/json", url, ct)
	}
	doc = map[string]any{}
	if err := json.Unmarshal(resp.Body(), &doc); err != nil {
		t.Fatalf("GET %s: response is not JSON (status %d): %s", url, resp.Status(), resp.Body())
	}
	return resp.Status(), doc
}

// WantKeys asserts every listed key is present in the object — key names are
// the contract; the Dart models read exactly these strings.
func WantKeys(t *testing.T, obj map[string]any, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if _, ok := obj[k]; !ok {
			t.Errorf("missing contract key %q in %v", k, obj)
		}
	}
}

// WantError asserts the uniform failure shape {"error": <msg>} — the only
// error body the mobile client knows how to surface.
func WantError(t *testing.T, status, wantStatus int, doc map[string]any) {
	t.Helper()
	if status != wantStatus {
		t.Errorf("status = %d, want %d", status, wantStatus)
	}
	msg, ok := doc["error"].(string)
	if !ok || msg == "" {
		t.Errorf(`error responses must be {"error": "<msg>"}, got %v`, doc)
	}
}
