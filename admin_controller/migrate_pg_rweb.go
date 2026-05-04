package admin_controller

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/rohanthewiz/church/admin"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/scripts"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

// pgDSNPlaceholder is the literal token in the embedded script that
// gets swapped for the live DSN. Kept as a named constant so any
// drift between the SQL file and the handler shows up as a missing
// substitution at runtime (returns an explicit error) rather than
// silently passing the placeholder to DuckDB.
const pgDSNPlaceholder = "{{PG_DSN}}"

// migrationName is the row key recorded in migration_state on success.
// The handler refuses to re-run while this row exists. Mirrors the
// final INSERT statement in pg_to_duckdb.sql — change them together.
const migrationName = "pg_to_duckdb"

// MigratePgToDuckDBRWeb performs the one-shot legacy Postgres → DuckDB
// data load.
//
// Authorisation
//
//	Gated on the same `SuperToken` the superadmin bootstrap uses.
//	That token only exists when no superadmin is present, which is
//	exactly the state we are in immediately before a fresh cut-over,
//	so the gate is naturally available at the moment we need it and
//	gone afterwards.
//
// Idempotency / one-shot guarantee
//
//	Before doing anything we look up `migration_state` for the marker
//	row. If it exists we abort with a clear error. The script itself
//	inserts that marker as its very last statement, so any failure
//	along the way leaves the marker absent and a retry is allowed —
//	the TRUNCATEs at the top of the script ensure retries start from
//	a clean slate.
//
// Query params
//
//	token   — must equal admin.SuperToken
//	pg_dsn  — full Postgres URI, e.g.
//	          postgresql://devuser:secret@localhost/church_development
//
// Note: pg_dsn is read from the query string, which means it (and the
// embedded password) ends up in the rweb verbose access log. That is
// acceptable for a one-shot local cut-over but should not be invoked
// over an untrusted network without TLS + log scrubbing.
func MigratePgToDuckDBRWeb(ctx rweb.Context) error {
	// Token gate — same shape as SetupSuperAdminRWeb.
	if admin.SuperToken == "" || ctx.Request().QueryParam("token") != admin.SuperToken {
		return errors.New("ye shalt not pass")
	}

	pgDSN := strings.TrimSpace(ctx.Request().QueryParam("pg_dsn"))
	if pgDSN == "" {
		return errors.New("pg_dsn query parameter is required")
	}

	dbHandle, err := db.Db()
	if err != nil {
		return serr.Wrap(err, "failed to obtain DuckDB handle")
	}

	// Marker check first. We probe DuckDB itself so the "already done"
	// signal survives process restarts and is inseparable from the
	// data it gates.
	already, err := migrationAlreadyDone(dbHandle, migrationName)
	if err != nil {
		return serr.Wrap(err, "failed checking migration_state")
	}
	if already {
		return errors.New("pg_to_duckdb migration has already run; refusing to repeat. Delete the row from migration_state to force a re-run.")
	}

	// Require the placeholder so a stale embedded script (placeholder
	// accidentally removed during a refactor) cannot run with an
	// unintended DSN baked in.
	if !strings.Contains(scripts.PgToDuckDB, pgDSNPlaceholder) {
		return errors.New("embedded migration script is missing the " + pgDSNPlaceholder + " placeholder")
	}
	script := strings.ReplaceAll(scripts.PgToDuckDB, pgDSNPlaceholder, pgDSN)

	logger.Log("info", "starting pg_to_duckdb migration")
	if err := db.ExecScript(dbHandle, script); err != nil {
		return serr.Wrap(err, "pg_to_duckdb migration failed")
	}
	logger.Log("info", "pg_to_duckdb migration completed")

	return ctx.WriteString("Migration complete.")
}

// migrationAlreadyDone reports whether `migration_state` already
// contains a row keyed by name. Looking it up by primary key keeps
// this to a single-row index probe regardless of how many migration
// markers the table accumulates over time.
func migrationAlreadyDone(dbHandle *sql.DB, name string) (bool, error) {
	var exists bool
	err := dbHandle.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM migration_state WHERE name = ?)",
		name,
	).Scan(&exists)
	if err != nil {
		return false, serr.Wrap(err, "marker lookup failed")
	}
	return exists, nil
}
