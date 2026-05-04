// Package scripts ships SQL artifacts that ride with the binary.
//
// The files in this directory are usable both as standalone CLI inputs
// (see the comments at the top of each .sql file for the relevant
// invocation) and as embedded payloads for in-process handlers — for
// example, MigratePgToDuckDBRWeb in admin_controller. Centralising the
// embed here keeps a single source of truth: editing the .sql file is
// enough, no extra rebuild step is required to keep the embedded copy
// in sync.
package scripts

import _ "embed"

// PgToDuckDB is the contents of scripts/pg_to_duckdb.sql. It contains
// a {{PG_DSN}} placeholder that the caller is expected to substitute
// before execution; see admin_controller.MigratePgToDuckDBRWeb for the
// canonical wiring.
//
//go:embed pg_to_duckdb.sql
var PgToDuckDB string
