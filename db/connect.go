package db

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2" // registers "duckdb" database/sql driver
	_ "github.com/lib/pq"              // registers "postgres" database/sql driver
	"github.com/rohanthewiz/serr"
)

// Cache DB handle and options
var dbHandle *sql.DB
var dbOpts *DBOpts

var DBTypes = dbTypes{"postgres", "mysql", "duckdb"}

type dbTypes struct {
	Postgres, MySQL, DuckDB string
}

// DBOpts carries every field any supported driver could want. Fields not
// relevant to the active DBType are ignored — e.g. Host/Port are unused
// for DuckDB, DuckDBPath is unused for Postgres.
type DBOpts struct {
	DBType   string
	Host     string
	Port     string
	User     string
	Word     string
	Database string

	// DuckDBPath is the on-disk file path for the DuckDB backend.
	// An empty string selects an in-memory instance (data is lost on close),
	// which is useful for tests but never for production.
	DuckDBPath string
}

// schema_duckdb.sql is embedded so it ships inside the binary — the
// caller doesn't need to know where the file lives on disk. It is
// replayed on every DuckDB open; every statement uses IF NOT EXISTS,
// so existing objects are untouched and new ones get picked up.
//
//go:embed schema_duckdb.sql
var duckdbSchema string

func InitDB(opts DBOpts) error {
	dbOpts = &opts

	err := openDB()
	if err != nil {
		return serr.Wrap(err, "Error initializing database")
	}
	return nil
}

func CloseDB() {
	if dbHandle != nil {
		dbHandle.Close()
	}
}

// Get a valid DB handle
func Db() (*sql.DB, error) {
	if dbHandle != nil {
		if dbHandle.Ping() == nil { // pings without error
			return dbHandle, nil
		}
	}
	err := openDB()
	return dbHandle, err
}

// openDB picks a driver and DSN shape based on DBOpts.DBType. Each
// branch is responsible for producing the string sql.Open expects for
// its driver — there is no single "DSN" format shared across drivers.
func openDB() error {
	if dbOpts == nil {
		return serr.Wrap(errors.New("Please call InitDB before using database"))
	}

	var (
		driverName string
		dsn        string
	)
	switch dbOpts.DBType {
	case DBTypes.DuckDB:
		// go-duckdb's DSN is simply the DB file path; "" = in-memory.
		driverName = "duckdb"
		dsn = dbOpts.DuckDBPath
	default:
		// Postgres (and historically MySQL under a pq-shaped DSN) land here.
		driverName = dbOpts.DBType
		dsn = fmt.Sprintf("host=%s dbname=%s user=%s password=%s port=%s sslmode=disable",
			dbOpts.Host, dbOpts.Database, dbOpts.User, dbOpts.Word, dbOpts.Port,
		)
	}

	db_, err := sql.Open(driverName, dsn)
	if err != nil {
		return serr.Wrap(err, "Failed to open database")
	}
	dbHandle = db_

	// DuckDB is file-backed and does not inherit schema from a separate
	// migration tool, so we replay the embedded schema on every open.
	if dbOpts.DBType == DBTypes.DuckDB {
		if err := bootstrapDuckDB(db_); err != nil {
			return serr.Wrap(err, "Failed to bootstrap DuckDB schema")
		}
	}
	return nil
}

// bootstrapDuckDB replays the embedded schema on every DuckDB open.
// The heavy lifting (split / strip / exec) lives in ExecScript so other
// callers — notably the one-shot Postgres→DuckDB migration endpoint —
// can reuse the exact same execution semantics.
func bootstrapDuckDB(db_ *sql.DB) error {
	if err := ExecScript(db_, duckdbSchema); err != nil {
		return serr.Wrap(err, "Failed bootstrapping DuckDB schema")
	}
	return nil
}

// ExecScript runs a multi-statement SQL string against db_ one statement
// at a time. Why split manually instead of handing the whole string to
// Exec?
//   - database/sql.Exec on go-duckdb runs exactly one statement per
//     call; multi-statement strings silently execute only the first.
//   - We split on ';' to feed one statement at a time. To keep that
//     split safe we first strip `-- ...` line comments, because a `;`
//     *inside* a comment (e.g. "DuckDB has no SERIAL; we emit one
//     sequence per table.") would otherwise create a chunk that is
//     comment-only and trips DuckDB's "empty query" error.
//   - Our scripts contain no string literals with embedded semicolons
//     and no stored-proc bodies, so once line comments are gone a plain
//     ';' split is sufficient.
//
// The function is fail-fast: the first failing statement aborts the
// run and is wrapped into the returned error so the caller can see
// exactly which statement broke.
func ExecScript(db_ *sql.DB, src string) error {
	for raw := range strings.SplitSeq(StripLineComments(src), ";") {
		stmt := strings.TrimSpace(raw)
		if stmt == "" {
			continue
		}
		if _, err := db_.Exec(stmt); err != nil {
			return serr.Wrap(err, "Failed executing SQL statement", "stmt", stmt)
		}
	}
	return nil
}

// StripLineComments removes `-- ...` to end-of-line on each line of src.
// We intentionally do NOT try to be clever about `--` appearing inside a
// string literal — none of our scripts contain such literals, and
// adding quote tracking would buy us nothing while complicating the
// code.
func StripLineComments(src string) string {
	var out strings.Builder
	out.Grow(len(src))
	for line := range strings.SplitSeq(src, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}
