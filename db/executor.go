package db

import "database/sql"

// Executor is the injection seam for the query layer: the minimal query
// surface shared by *sql.DB, *sql.Tx, and test doubles like go-sqlmock.
//
// Why an interface here instead of *sql.DB everywhere: query functions that
// take a concrete *sql.DB can only ever run against the process-global
// connection, which forces tests through the SetHandleForTesting global swap
// and makes multi-statement transactions impossible to compose (a *sql.Tx is
// not a *sql.DB). Accepting this interface lets the same query function run
// against the live handle, a transaction, or a mock — the caller decides.
//
// The method set is deliberately identical to vattle/sqlboiler's
// boil.Executor, so a db.Executor value passes implicitly wherever generated
// model code (models.Sermons(exec, ...)) expects one — no adapter needed.
// Hand-written SQL packages (apitoken, idrive's sermon cache) use it without
// importing sqlboiler at all.
//
// Convention: the executor is always the FIRST parameter of a query function
// (matching SQLBoiler's generated API), and it is fetched via db.Db() at
// natural boundaries only — HTTP handlers, module data loaders, background
// services, bootstrap — then threaded down. Query functions never reach for
// the global themselves.
type Executor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}
