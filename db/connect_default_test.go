package db

import "testing"

// TestInitDBDefaultsToPostgres guards the default-backend contract: an empty
// DBType must resolve to Postgres and must never bring up the embedded bytdb
// engine. No server is needed — lib/pq's sql.Open is lazy and does not dial,
// so the test observes only the selection, not a live connection.
func TestInitDBDefaultsToPostgres(t *testing.T) {
	prevHandle, prevOpts := dbHandle, dbOpts
	prevEngine, prevServer, prevAddr := bytdbEngine, bytdbServer, bytdbAddr
	t.Cleanup(func() {
		if dbHandle != nil && dbHandle != prevHandle {
			dbHandle.Close()
		}
		dbHandle, dbOpts = prevHandle, prevOpts
		bytdbEngine, bytdbServer, bytdbAddr = prevEngine, prevServer, prevAddr
	})
	dbHandle, dbOpts = nil, nil
	bytdbEngine, bytdbServer, bytdbAddr = nil, nil, ""

	if err := InitDB(DBOpts{Host: "127.0.0.1", Port: "5432", Database: "unused"}); err != nil {
		t.Fatalf("InitDB with empty DBType: %v", err)
	}
	if dbOpts.DBType != DBTypes.Postgres {
		t.Fatalf("empty DBType resolved to %q, want %q", dbOpts.DBType, DBTypes.Postgres)
	}
	if bytdbEngine != nil || BytDBWireAddr() != "" {
		t.Fatalf("empty DBType started the embedded bytdb engine (addr %q)", BytDBWireAddr())
	}
}
