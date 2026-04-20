package model

import (
	"database/sql/driver"
	"fmt"

	"github.com/lib/pq"

	"github.com/rohanthewiz/church/db"
)

// StringSlice is the driver-neutral replacement for pq.StringArray.
//
// Why a wrapper instead of the driver's own type?
//
//	pq.StringArray speaks the Postgres `text[]` text-format wire protocol
//	(e.g. `{a,b,"c d"}`). DuckDB's driver delivers and accepts list values
//	as native Go slices (`[]any`, `[]string`). The two shapes are not
//	interchangeable — feeding DuckDB's `[]any` to pq.StringArray.Scan
//	silently produces the wrong result (parse error or empty slice), and
//	returning pq.StringArray's text format as a query parameter to DuckDB
//	binds a single VARCHAR instead of a VARCHAR[]. Dispatching on the
//	active dialect is what keeps a single struct definition working under
//	both backends.
//
// Storage: StringSlice is `[]string` underneath, so any field that used
// to be pq.StringArray keeps its old value-assignment ergonomics —
// `model.Article{Categories: []string{"news"}}` compiles unchanged.
type StringSlice []string

// Scan populates the slice from whatever shape the driver hands us.
//
//	DuckDB    — go-duckdb decodes VARCHAR[] as []any (each element a
//	            string). Some list paths may short-circuit to []string.
//	Postgres  — lib/pq passes the raw text-array bytes ({a,b,"c d"}) as
//	            []byte or string; we delegate to pq.StringArray.Scan so
//	            the canonical parser stays in one place.
//	nil       — treat as empty slice, not an error. Some columns are
//	            NOT NULL at the schema level but code still expects a
//	            benign zero value in tests.
func (s *StringSlice) Scan(src any) error {
	if src == nil {
		*s = nil
		return nil
	}

	switch v := src.(type) {
	case []any:
		// DuckDB's default list decoding path.
		out := make([]string, len(v))
		for i, item := range v {
			if item == nil {
				out[i] = ""
				continue
			}
			str, ok := item.(string)
			if !ok {
				return fmt.Errorf("model: StringSlice scan: expected string element, got %T", item)
			}
			out[i] = str
		}
		*s = out
		return nil

	case []string:
		// Defensive branch: some driver versions surface list-of-varchar
		// already typed. Cheaper than re-converting through []any.
		out := make([]string, len(v))
		copy(out, v)
		*s = out
		return nil

	case []byte, string:
		// Postgres wire format. pq.StringArray understands both []byte
		// and string inputs and handles quoting/escaping correctly.
		return (*pq.StringArray)(s).Scan(src)
	}

	return fmt.Errorf("model: StringSlice scan: unsupported source type %T", src)
}

// Value serializes the slice for the active driver.
//
// Postgres path: build the `{a,b,"c d"}` text-array literal via
// pq.StringArray so lib/pq recognises it as a text[] binding.
//
// DuckDB path: go-duckdb's NamedValueChecker accepts arbitrary Go
// slices as list parameters, so we return the underlying []string
// directly. Wrapping it in pq's text format here would bind a single
// VARCHAR literal that DuckDB would then refuse (type mismatch) — or
// worse, coerce to a stringified list.
//
// A nil StringSlice returns nil so the database stores SQL NULL where
// the column permits it. Callers that need an explicit empty array
// should assign `StringSlice{}` rather than leaving the field nil.
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	if db.CurrentDialect() == db.DialectDuckDB {
		// Return a fresh []string copy. go-duckdb reads the slice during
		// bind; aliasing the caller's backing array would be safe today
		// but introduces a lifetime coupling that is easy to forget.
		out := make([]string, len(s))
		copy(out, s)
		return out, nil
	}
	return pq.StringArray(s).Value()
}
