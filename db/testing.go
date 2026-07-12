package db

import "database/sql"

// SetHandleForTesting swaps the cached handle for a test double (e.g. a
// go-sqlmock DB) so query code and HTTP handlers can be exercised without a
// live Postgres.
//
// Why this exists: query functions reach for the package-global handle via
// Db() rather than accepting an executor parameter, so a test's only seam is
// the global itself. (If queries are ever refactored to take a
// boil.ContextExecutor, this hook — and the global — can go away.)
//
// Db() pings the handle before returning it; sqlmock answers pings
// successfully unless ping-monitoring is enabled, so no expectation is needed
// for that. Not safe for parallel tests within a package — the handle is
// process-global state.
func SetHandleForTesting(h *sql.DB) {
	dbHandle = h
}
