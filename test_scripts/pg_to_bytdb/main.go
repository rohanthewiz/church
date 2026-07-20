// pg_to_bytdb copies a site's Postgres database into a fresh bytdb data
// file — the cutover tool for migrating existing installs to the embedded
// backend.
//
//	go run ./test_scripts/pg_to_bytdb \
//	  -pg "host=localhost user=devuser password=secret dbname=church_development sslmode=disable" \
//	  -dest data/migrated_church.db
//
// Approach: the destination is brought up through the production path
// (db.InitDB → schema bootstrap → pgwire loopback), so the produced file is
// bit-for-bit what a site would create for itself — same engine version,
// same schema, same wire semantics. Rows are then copied table-by-table in
// FK-dependency order (db.BytDBTableNames), preserving ids.
//
// No sequence fix-up: bytdb identity counters self-heal — an explicit-id
// insert bumps the durable counter past that id in the same transaction
// (verified upstream on v0.6.2), so after an id-preserving copy the next
// DEFAULT insert is already correct. Postgres-style setval() is unnecessary
// (and setval on identity-column readback names like users_id_seq is not
// supported over the wire anyway).
//
// The tool is deliberately strict: a missing destination column, a type the
// wire rejects, or a count mismatch aborts with a non-zero exit — a cutover
// must fail loudly, never produce a silently partial database. The one
// tolerated skew: a source table absent in Postgres (e.g. prayer_requests
// on an install predating that feature) is skipped with a notice, since the
// bootstrap has already created it empty.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

func main() {
	pgDSN := flag.String("pg", os.Getenv("PG_DSN"),
		"source Postgres DSN (or env PG_DSN), e.g. \"host=localhost user=devuser password=secret dbname=church_development sslmode=disable\"")
	dest := flag.String("dest", "data/migrated_church.db", "destination bytdb file (must not exist)")
	flag.Parse()

	if strings.TrimSpace(*pgDSN) == "" {
		fmt.Println("A source is required: -pg <dsn> (or env PG_DSN)")
		os.Exit(2)
	}
	// Refusing an existing destination (rather than truncating) keeps a retry
	// after a partial run from silently appending into half-copied tables.
	if _, err := os.Stat(*dest); err == nil {
		fmt.Println("Destination already exists — remove it first:", *dest)
		os.Exit(2)
	}

	if err := run(*pgDSN, *dest); err != nil {
		fmt.Printf("MIGRATION FAILED: %+v\n", err)
		// The half-written destination is useless and dangerous to leave
		// around (it looks like a completed migration); remove it.
		db.CloseDB()
		os.Remove(*dest)
		os.Exit(1)
	}
	db.CloseDB() // flushes the WAL before the process exits
	fmt.Println("\nRESULT: migration complete —", *dest)
	fmt.Println("Next: run the wire proof and boot a site with DB_FILE pointed at the new file.")
}

func run(pgDSN, dest string) error {
	src, err := sql.Open("postgres", pgDSN)
	if err != nil {
		return serr.Wrap(err, "could not open source Postgres")
	}
	defer src.Close()
	if err = src.Ping(); err != nil {
		return serr.Wrap(err, "source Postgres did not answer ping")
	}

	// Production init path: engine + schema bootstrap + loopback wire.
	err = db.InitDB(db.DBOpts{DBType: db.DBTypes.BytDB, File: dest, Listen: "127.0.0.1:0"})
	if err != nil {
		return serr.Wrap(err, "could not initialize destination bytdb")
	}
	dst, err := db.Db()
	if err != nil {
		return serr.Wrap(err, "could not get destination handle")
	}

	var totalRows int64
	for _, tbl := range db.BytDBTableNames() {
		n, copied, err := copyTable(src, dst, tbl)
		if err != nil {
			return serr.Wrap(err, "error copying table", "table", tbl)
		}
		if !copied {
			fmt.Printf("skip  %-20s (not present in source)\n", tbl)
			continue
		}
		fmt.Printf("copy  %-20s %6d rows\n", tbl, n)
		totalRows += n
	}
	fmt.Printf("\n%d rows across all tables; verifying counts…\n", totalRows)

	for _, tbl := range db.BytDBTableNames() {
		if err := verifyCount(src, dst, tbl); err != nil {
			return err
		}
	}
	fmt.Println("verify OK: source and destination row counts match on every table")
	return nil
}

// copyTable streams every source row into the destination inside one
// transaction, preserving ids. copied=false means the table doesn't exist
// in the source (older install), which the caller reports and tolerates.
func copyTable(src, dst *sql.DB, table string) (n int64, copied bool, err error) {
	var exists bool
	err = src.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_name = $1)`, table).Scan(&exists)
	if err != nil {
		return 0, false, serr.Wrap(err, "error checking source table existence")
	}
	if !exists {
		return 0, false, nil
	}

	// ORDER BY the first column (id on every bootstrap table) — deterministic
	// runs make two migration attempts diffable against each other.
	rows, err := src.Query(`SELECT * FROM ` + table + ` ORDER BY 1`)
	if err != nil {
		return 0, false, serr.Wrap(err, "error reading source rows")
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return 0, false, serr.Wrap(err, "error reading source columns")
	}
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	// Explicit column list from the SOURCE: if the source carries a column
	// the destination schema lacks (or vice-versa drifted), the insert
	// errors — schema drift must surface at migration time, not after.
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	tx, err := dst.Begin()
	if err != nil {
		return 0, false, serr.Wrap(err, "error starting destination transaction")
	}
	defer tx.Rollback() // no-op after a successful Commit

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return 0, false, serr.Wrap(err, "error preparing insert", "sql", insertSQL)
	}
	defer stmt.Close()

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err = rows.Scan(ptrs...); err != nil {
			return n, false, serr.Wrap(err, "error scanning source row")
		}
		args := make([]any, len(cols))
		for i, v := range vals {
			// lib/pq scans text/varchar/arrays/jsonb as []byte. Passing them
			// on as strings lets the destination coerce by column type —
			// array literals ("{a,b}") and jsonb documents ride through in
			// their text form, which the wire proof showed bytdb accepts.
			// Scalars (int64, bool, float64, time.Time) and NULL (nil) pass
			// through untouched.
			if b, ok := v.([]byte); ok {
				args[i] = string(b)
			} else {
				args[i] = v
			}
		}
		if _, err = stmt.Exec(args...); err != nil {
			return n, false, serr.Wrap(err, "error inserting row", "row", fmt.Sprint(n+1))
		}
		n++
	}
	if err = rows.Err(); err != nil {
		return n, false, serr.Wrap(err, "error iterating source rows")
	}
	if err = tx.Commit(); err != nil {
		return n, false, serr.Wrap(err, "error committing table copy")
	}
	return n, true, nil
}

// verifyCount re-counts both sides after all copies — an independent check
// on the copy loop's own bookkeeping. Tables absent in the source verify
// against zero (bootstrap created them empty).
func verifyCount(src, dst *sql.DB, table string) error {
	var srcN int64
	err := src.QueryRow(
		`SELECT COALESCE((SELECT count(*) FROM ` + table + `), 0)`).Scan(&srcN)
	if err != nil {
		// Absent in source → expect an empty destination table.
		srcN = 0
	}
	var dstN int64
	if err = dst.QueryRow(`SELECT count(*) FROM ` + table).Scan(&dstN); err != nil {
		return serr.Wrap(err, "error counting destination rows", "table", table)
	}
	if srcN != dstN {
		return serr.New("row count mismatch", "table", table,
			"source", fmt.Sprint(srcN), "destination", fmt.Sprint(dstN))
	}
	return nil
}
