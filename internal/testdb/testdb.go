// Package testdb runs a test against each database backend church supports,
// so a check written once proves both. Each backend leaves the db package
// initialized (db.InitDB, as a site binary does) on an empty, fully migrated
// schema, and registers the cleanup that tears it down.
//
// bytdb builds its schema in code on first open (db/bytdb_schema.go), so its
// opener is one call. Postgres is goose-managed, so its opener builds a
// throwaway database and applies every migration's Up section, the way
// `dbc migrate up` would on a new site:
//
//	CHURCH_TEST_PG_DSN (any database on the server, e.g. postgres or
//	        │           church_development; only used to CREATE/DROP)
//	        ▼
//	CREATE DATABASE church_smoke_<nanos> ──► db/migrate/*.sql Up sections,
//	        │                                in filename (= version) order
//	        ▼
//	db.InitDB(Postgres) ──► test body ──► CloseDB ──► DROP DATABASE
//
// Postgres runs only when CHURCH_TEST_PG_DSN is set; CI has no server, so
// there the Postgres subtest skips and bytdb alone runs. A throwaway database
// rather than a shared one because tests insert fixed names and assert on
// counts, and so that no run can touch data anyone cares about. The DSN's
// role needs CREATEDB. The charges migration runs `ALTER TABLE ... OWNER TO
// devuser`, so the role must also be devuser or a member of it, as it is on
// the dev machines this is meant for:
//
//	CHURCH_TEST_PG_DSN="postgres://devuser:secret@localhost:5432/church_development?sslmode=disable" \
//	    go test ./...
//
// The db package holds one global handle, so tests using this must not run
// in parallel with each other.
package testdb

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/rohanthewiz/church/db"
)

// EnvPostgresDSN names the variable that enables the Postgres backend.
const EnvPostgresDSN = "CHURCH_TEST_PG_DSN"

// Each runs body once per backend, as subtests named "bytdb" and "postgres",
// with the db package opened on that backend.
func Each(t *testing.T, body func(t *testing.T)) {
	t.Run("bytdb", func(t *testing.T) {
		OpenBytDB(t)
		body(t)
	})
	t.Run("postgres", func(t *testing.T) {
		OpenPostgres(t)
		body(t)
	})
}

// OpenBytDB initializes the db package on an embedded bytdb in a temp dir.
func OpenBytDB(t testing.TB) {
	t.Helper()
	if err := db.InitDB(db.DBOpts{DBType: db.DBTypes.BytDB, File: filepath.Join(t.TempDir(), "church.db")}); err != nil {
		t.Fatalf("InitDB bytdb: %v", err)
	}
	t.Cleanup(db.CloseDB)
}

// OpenPostgres initializes the db package on a throwaway, migrated Postgres
// database, or skips the test when EnvPostgresDSN is unset.
func OpenPostgres(t testing.TB) {
	t.Helper()
	dsn := os.Getenv(EnvPostgresDSN)
	if dsn == "" {
		t.Skip(EnvPostgresDSN + " not set")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "postgres" && u.Scheme != "postgresql" {
		t.Fatalf("%s must be a postgres:// URL (parse error: %v)", EnvPostgresDSN, err)
	}

	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	t.Cleanup(func() { admin.Close() })

	// The name is generated here, never taken from input, so quoting it into
	// DDL (which takes no bind parameters) is safe.
	name := fmt.Sprintf("church_smoke_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("create throwaway database: %v", err)
	}
	// Registered before anything else can fail, so a failed migration still
	// drops the database. Cleanups run last-in first-out: db.CloseDB (below)
	// releases the site's connections before this DROP runs, and FORCE ends
	// any a failed test left open.
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`); err != nil {
			t.Errorf("drop throwaway database %s: %v", name, err)
		}
	})

	target := *u
	target.Path = "/" + name
	applyMigrations(t, target.String())

	// db.InitDB takes the connection in parts (it builds a key=value string
	// with sslmode=disable, as the site config does).
	pass, _ := u.User.Password()
	port := u.Port()
	if port == "" {
		port = "5432"
	}
	if err := db.InitDB(db.DBOpts{DBType: db.DBTypes.Postgres, Host: u.Hostname(), Port: port,
		User: u.User.Username(), Word: pass, Database: name}); err != nil {
		t.Fatalf("InitDB postgres: %v", err)
	}
	t.Cleanup(db.CloseDB)
	t.Logf("postgres test database: %s", name)
}

// migrationsDir locates db/migrate from this file's own path, so tests in
// any package (each runs with its own directory as the working directory)
// find the same migrations.
func migrationsDir(t testing.TB) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate testdb source to find db/migrate")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrate")
}

// applyMigrations runs the goose Up section of every db/migrate file, in
// filename order (goose's version order, since names start with a
// timestamp). Each section goes to Exec whole: with no bind parameters lib/pq
// uses the simple query protocol, which accepts several statements at once,
// including goose StatementBegin/End blocks, so no statement splitting is
// needed.
func applyMigrations(t testing.TB, dsn string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(migrationsDir(t), "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found under db/migrate (err %v)", err)
	}
	sort.Strings(files)

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open throwaway database: %v", err)
	}
	defer conn.Close()

	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		up := string(raw)
		if i := strings.Index(up, "-- +goose Up"); i >= 0 {
			up = up[i+len("-- +goose Up"):]
		}
		if i := strings.Index(up, "-- +goose Down"); i >= 0 {
			up = up[:i]
		}
		if _, err := conn.Exec(up); err != nil {
			t.Fatalf("migration %s: %v", filepath.Base(f), err)
		}
	}
}
