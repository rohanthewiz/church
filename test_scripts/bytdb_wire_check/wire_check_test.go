package main

import (
	"testing"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/internal/testdb"
)

// TestWireChecks runs the script's checks on each backend. On Postgres they
// prove the same hand-written SQL (ON CONFLICT upserts, text[] binding, the
// sermon search, FK cascades) that bytdb has to emulate.
func TestWireChecks(t *testing.T) {
	testdb.Each(t, func(t *testing.T) {
		dbH, err := db.Db()
		if err != nil {
			t.Fatal(err)
		}
		failures = 0
		runChecks(dbH)
		if failures > 0 {
			t.Errorf("%d wire check(s) failed; see the FAIL lines above", failures)
		}
	})
}
