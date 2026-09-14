// bytdb_to_pg copies a site's bytdb data file into a Postgres database — the
// reverse of test_scripts/pg_to_bytdb, for moving a site back onto (or onto)
// the Postgres backend.
//
//	go run ./test_scripts/bytdb_to_pg \
//	  -src data/church.db \
//	  -pg "host=localhost user=devuser password=secret dbname=church_restored sslmode=disable"
//
// Prerequisites on the Postgres side: the database exists and the goose
// migrations have been applied (dbc migrate up from the church directory).
// Unlike the bytdb path, Postgres has no in-code schema bootstrap, and this
// tool deliberately does not create one — goose is the Postgres schema's
// source of truth, and a second DDL copy here could only drift from it.
//
// Flow:
//
//	-src church.db ──copy──▶ temp snapshot ──db.InitDB (bytdb, loopback pgwire)──┐
//	                                                                             │ SELECT *
//	Postgres ◀── one transaction: INSERT per table (FK order) ◀──────────────────┘
//	         ◀── setval() every serial/identity sequence past MAX(col)
//	         ◀── per-table count check ── COMMIT (or ROLLBACK on any failure)
//
// Design choices:
//   - The source is opened from a temp COPY, never in place. Opening a bytdb
//     file runs the schema bootstrap (which may add tables to an older file)
//     and can compact it; a migration tool must not mutate its input. The copy
//     also means a snapshot pulled from object storage (latest/church.db) or a
//     file from a stopped site can be used as-is. Do not point -src at the live
//     data file of a running site — the copy would race its writer.
//   - The source is read through the production path (db.InitDB → pgwire →
//     lib/pq), so values arrive exactly as the app sees them.
//   - Everything happens in ONE Postgres transaction. Postgres (unlike bytdb)
//     makes that cheap, and it upgrades pg_to_bytdb's per-table atomicity to
//     all-or-nothing: a failed run leaves the destination exactly as it was,
//     so a retry needs no cleanup.
//   - Destination tables must be empty. Refusing (rather than truncating)
//     keeps an accidental run against a live Postgres site from destroying
//     its data; wipe or recreate the database deliberately if that is intended.
//   - Sequences ARE fixed up, unlike pg_to_bytdb. Postgres does not advance a
//     bigserial sequence on explicit-id inserts, so without setval the next
//     app INSERT would collide with a copied id (duplicate key on users_pkey).
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

func main() {
	src := flag.String("src", "", "source bytdb data file (e.g. data/church.db or a downloaded latest/church.db snapshot)")
	pgDSN := flag.String("pg", os.Getenv("PG_DSN"),
		"destination Postgres DSN (or env PG_DSN), e.g. \"host=localhost user=devuser password=secret dbname=church_restored sslmode=disable\"")
	flag.Parse()

	if strings.TrimSpace(*src) == "" || strings.TrimSpace(*pgDSN) == "" {
		fmt.Println("Both are required: -src <bytdb file> and -pg <dsn> (or env PG_DSN)")
		os.Exit(2)
	}
	// A missing source must be caught here: bytdb.Open would happily create an
	// empty file, and the "migration" would then copy zero rows successfully.
	if fi, err := os.Stat(*src); err != nil || fi.IsDir() {
		fmt.Println("Source bytdb file not found (or is a directory):", *src)
		os.Exit(2)
	}

	err := run(*src, *pgDSN)
	db.CloseDB() // idempotent; releases the engine before the temp dir is removed
	if err != nil {
		fmt.Printf("MIGRATION FAILED (destination rolled back, unchanged): %+v\n", err)
		os.Exit(1)
	}
	fmt.Println("\nRESULT: migration complete")
	fmt.Println("Next: boot a site with DB_TYPE=postgres (plus PG_* settings) against the destination.")
}

func run(srcFile, pgDSN string) error {
	// Destination first: its checks are cheap and catch the common mistakes
	// (migrations not run, wrong/non-empty database) before copying the source.
	dst, err := sql.Open("postgres", pgDSN)
	if err != nil {
		return serr.Wrap(err, "could not open destination Postgres")
	}
	defer dst.Close()
	if err = dst.Ping(); err != nil {
		return serr.Wrap(err, "destination Postgres did not answer ping")
	}
	tables := db.BytDBTableNames()
	if err = checkDestinationReady(dst, tables); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "bytdb_to_pg")
	if err != nil {
		return serr.Wrap(err, "could not create temp dir for source snapshot")
	}
	defer os.RemoveAll(tmpDir)
	snapshot := filepath.Join(tmpDir, "church.db")
	if err = copyFile(srcFile, snapshot); err != nil {
		return err
	}

	// Production init path on the snapshot: engine + schema bootstrap +
	// loopback wire. Replication/restore stay inert because no app config is
	// loaded in this process (replicationConfigured() is false).
	err = db.InitDB(db.DBOpts{DBType: db.DBTypes.BytDB, File: snapshot, Listen: "127.0.0.1:0"})
	if err != nil {
		return serr.Wrap(err, "could not open source bytdb snapshot", "src", srcFile)
	}
	srcDB, err := db.Db()
	if err != nil {
		return serr.Wrap(err, "could not get source handle")
	}

	tx, err := dst.Begin()
	if err != nil {
		return serr.Wrap(err, "error starting destination transaction")
	}
	defer tx.Rollback() // no-op after a successful Commit

	var totalRows int64
	for _, tbl := range tables {
		n, err := copyTable(srcDB, tx, tbl)
		if err != nil {
			return serr.Wrap(err, "error copying table", "table", tbl)
		}
		fmt.Printf("copy  %-20s %6d rows\n", tbl, n)
		totalRows += n
	}

	fmt.Printf("\n%d rows across all tables; resetting sequences…\n", totalRows)
	for _, tbl := range tables {
		if err = resetSequences(tx, tbl); err != nil {
			return serr.Wrap(err, "error resetting sequences", "table", tbl)
		}
	}

	// Counted inside the transaction (which sees its own inserts), so a
	// mismatch still rolls everything back rather than leaving a committed,
	// known-bad destination.
	fmt.Println("\nverifying counts…")
	for _, tbl := range tables {
		if err = verifyCount(srcDB, tx, tbl); err != nil {
			return err
		}
	}
	fmt.Println("verify OK: source and destination row counts match on every table")

	if err = tx.Commit(); err != nil {
		return serr.Wrap(err, "error committing migration")
	}
	return nil
}

// checkDestinationReady requires every bootstrap table to exist in the
// destination and be empty. Both are checked for ALL tables before any write,
// so the failure message lists the whole problem at once instead of one table
// per attempt.
func checkDestinationReady(dst *sql.DB, tables []string) error {
	var missing, nonEmpty []string
	for _, tbl := range tables {
		var exists bool
		err := dst.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables
			 WHERE table_schema = 'public' AND table_name = $1)`, tbl).Scan(&exists)
		if err != nil {
			return serr.Wrap(err, "error checking destination table existence", "table", tbl)
		}
		if !exists {
			missing = append(missing, tbl)
			continue
		}
		// EXISTS stops at the first row — cheap even if the table is large.
		var hasRows bool
		if err = dst.QueryRow(`SELECT EXISTS (SELECT 1 FROM ` + tbl + `)`).Scan(&hasRows); err != nil {
			return serr.Wrap(err, "error checking destination table contents", "table", tbl)
		}
		if hasRows {
			nonEmpty = append(nonEmpty, tbl)
		}
	}
	if len(missing) > 0 {
		return serr.New("destination is missing tables — apply the goose migrations first "+
			"(dbc migrate up from the church directory)", "tables", strings.Join(missing, ","))
	}
	if len(nonEmpty) > 0 {
		return serr.New("destination tables already contain rows — refusing to merge into existing data; "+
			"use a fresh database", "tables", strings.Join(nonEmpty, ","))
	}
	return nil
}

// copyFile snapshots the source so the migration never opens (and so never
// bootstraps or compacts) the caller's file.
func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return serr.Wrap(err, "could not open source file", "src", from)
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return serr.Wrap(err, "could not create source snapshot", "path", to)
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return serr.Wrap(err, "could not copy source to snapshot", "src", from)
	}
	if err = out.Close(); err != nil {
		return serr.Wrap(err, "could not finalize source snapshot", "path", to)
	}
	return nil
}

// copyTable streams every source row into the destination transaction,
// preserving ids. Every table in the bytdb bootstrap exists in the source by
// construction (InitDB just ensured it), so there is no "absent table" case.
func copyTable(src *sql.DB, tx *sql.Tx, table string) (n int64, err error) {
	// ORDER BY the first column (the primary key on every bootstrap table) —
	// deterministic insert order makes two runs diffable against each other.
	rows, err := src.Query(`SELECT * FROM ` + table + ` ORDER BY 1`)
	if err != nil {
		return 0, serr.Wrap(err, "error reading source rows")
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return 0, serr.Wrap(err, "error reading source columns")
	}
	// A `date` scanned as time.Time would be re-sent as a full RFC3339
	// timestamp. Postgres would accept that, but the text form carries a time
	// and zone the column never had — send the plain YYYY-MM-DD instead so the
	// value cannot shift a day under any server TimeZone setting. (Mirrors the
	// same guard in pg_to_bytdb, where bytdb rejects the timestamp form.)
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return 0, serr.Wrap(err, "error reading source column types")
	}
	isDate := make([]bool, len(colTypes))
	for i, ct := range colTypes {
		isDate[i] = strings.EqualFold(ct.DatabaseTypeName(), "DATE")
	}
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	// Explicit column list from the SOURCE: a column the Postgres schema lacks
	// (migration not yet written or not applied) makes the prepare fail, so
	// schema drift surfaces now rather than as silently dropped data.
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return 0, serr.Wrap(err, "error preparing insert (schema drift between bytdb bootstrap and goose migrations?)",
			"sql", insertSQL)
	}
	defer stmt.Close()

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err = rows.Scan(ptrs...); err != nil {
			return n, serr.Wrap(err, "error scanning source row")
		}
		args := make([]any, len(cols))
		for i, v := range vals {
			// lib/pq scans text, text[] and jsonb as []byte. Re-sending them as
			// strings lets Postgres coerce by the destination column type —
			// "{a,b}" array literals and JSON documents parse in their text
			// form. (As []byte, lib/pq would send bytea-escaped binary, which
			// Postgres would reject for text[]/jsonb.) Scalars (int64, bool,
			// float64, time.Time) and NULL (nil) pass through unchanged.
			if b, ok := v.([]byte); ok {
				args[i] = string(b)
			} else if t, ok := v.(time.Time); ok && isDate[i] {
				args[i] = t.Format("2006-01-02")
			} else {
				args[i] = v
			}
		}
		if _, err = stmt.Exec(args...); err != nil {
			return n, serr.Wrap(err, "error inserting row", "row", fmt.Sprint(n+1))
		}
		n++
	}
	if err = rows.Err(); err != nil {
		return n, serr.Wrap(err, "error iterating source rows")
	}
	return n, nil
}

// resetSequences moves each serial/identity sequence on the table past the
// copied ids. Columns are discovered from the catalog instead of assuming
// "id": event_locations and event_recurrences key on event_id (no sequence),
// and a future table may name its serial column differently.
//
// setval semantics used here:
//
//	MAX(col) = m > 0  → setval(seq, m, true)   next nextval() returns m+1
//	empty table       → setval(seq, 1, false)  next nextval() returns 1
func resetSequences(tx *sql.Tx, table string) error {
	colRows, err := tx.Query(
		`SELECT column_name FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = $1
		   AND (column_default LIKE 'nextval(%' OR is_identity = 'YES')`, table)
	if err != nil {
		return serr.Wrap(err, "error listing sequence-backed columns")
	}
	var seqCols []string
	for colRows.Next() {
		var c string
		if err = colRows.Scan(&c); err != nil {
			colRows.Close()
			return serr.Wrap(err, "error scanning sequence-backed column")
		}
		seqCols = append(seqCols, c)
	}
	colRows.Close() // must close before the next statement on this tx connection
	if err = colRows.Err(); err != nil {
		return serr.Wrap(err, "error iterating sequence-backed columns")
	}

	for _, col := range seqCols {
		var next int64
		// Column names come from the catalog, not user input; quoting guards
		// against any that would need it.
		q := fmt.Sprintf(
			`SELECT setval(pg_get_serial_sequence($1, $2),
			               GREATEST(COALESCE(MAX(%[1]s), 0), 1),
			               COALESCE(MAX(%[1]s), 0) > 0) + CASE WHEN COALESCE(MAX(%[1]s), 0) > 0 THEN 1 ELSE 0 END
			 FROM %[2]s`, `"`+col+`"`, table)
		if err = tx.QueryRow(q, table, col).Scan(&next); err != nil {
			return serr.Wrap(err, "error setting sequence", "column", col)
		}
		fmt.Printf("seq   %-20s %-12s next=%d\n", table, col, next)
	}
	return nil
}

// verifyCount re-counts both sides — an independent check on copyTable's own
// bookkeeping.
func verifyCount(src *sql.DB, tx *sql.Tx, table string) error {
	var srcN, dstN int64
	if err := src.QueryRow(`SELECT count(*) FROM ` + table).Scan(&srcN); err != nil {
		return serr.Wrap(err, "error counting source rows", "table", table)
	}
	if err := tx.QueryRow(`SELECT count(*) FROM ` + table).Scan(&dstN); err != nil {
		return serr.Wrap(err, "error counting destination rows", "table", table)
	}
	if srcN != dstN {
		return serr.New("row count mismatch", "table", table,
			"source", fmt.Sprint(srcN), "destination", fmt.Sprint(dstN))
	}
	return nil
}
