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

// bootstrapDuckDB executes schema_duckdb.sql statement-by-statement.
// Why split manually instead of passing the whole string to Exec?
//   - database/sql.Exec on go-duckdb runs exactly one statement per call;
//     multi-statement strings get only the first statement executed.
//   - A simple split on ';' is safe here because the schema file contains
//     no string literals with embedded semicolons and no stored-proc bodies.
//     Line comments (`-- ...`) are parsed by DuckDB itself, so they can
//     ride along with the following statement without issue.
func bootstrapDuckDB(db_ *sql.DB) error {
	for _, raw := range strings.Split(duckdbSchema, ";") {
		stmt := strings.TrimSpace(raw)
		if stmt == "" {
			continue
		}
		if _, err := db_.Exec(stmt); err != nil {
			return serr.Wrap(err, "Failed executing DuckDB schema statement", "stmt", stmt)
		}
	}
	return nil
}
