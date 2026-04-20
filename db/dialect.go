package db

import (
	"strconv"
	"strings"
)

// Dialect identifies the SQL dialect we're emitting placeholders for.
// The DAO layer writes all SQL with `?` placeholders (DuckDB-native style)
// and calls Rebind at the edge. When the backend is swapped from Postgres
// to DuckDB only the dialect constant flips; no DAO SQL changes.
type Dialect int

const (
	DialectPostgres Dialect = iota
	DialectDuckDB
)

// CurrentDialect returns the dialect matching the open driver.
// Tied to DBOpts.DBType so the DAO layer can stay driver-agnostic.
func CurrentDialect() Dialect {
	// Default to Postgres for now; DuckDB support flips this on driver swap.
	if dbOpts != nil && dbOpts.DBType == DBTypes.DuckDB {
		return DialectDuckDB
	}
	return DialectPostgres
}

// Rebind rewrites `?` placeholders to the form required by the current dialect.
// - Postgres (lib/pq) requires `$1, $2, ...`.
// - DuckDB (go-duckdb) accepts `?` natively, so pass-through.
//
// Design notes:
//   - Single quotes toggle a "string literal" mode; `?` inside literals is left alone.
//   - The SQL '' escape (two consecutive single quotes) is preserved without toggling.
//   - Standard SQL does not use backslash escapes inside single-quoted strings, so
//     we intentionally do not try to honor `\'`.
//   - No attempt is made to parse comments; DAO SQL in this codebase does not contain them.
func Rebind(dialect Dialect, query string) string {
	if dialect != DialectPostgres {
		return query
	}
	if !strings.ContainsRune(query, '?') {
		return query
	}

	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 0
	inString := false
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch c {
		case '\'':
			// Preserve the SQL '' escape without toggling our in-string flag.
			if inString && i+1 < len(query) && query[i+1] == '\'' {
				b.WriteByte(c)
				b.WriteByte(c)
				i++
				continue
			}
			inString = !inString
			b.WriteByte(c)
		case '?':
			if inString {
				b.WriteByte(c)
				continue
			}
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}