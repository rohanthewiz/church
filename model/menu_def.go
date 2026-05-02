package model

import (
	"database/sql"
)

// MenuDef mirrors the `menu_defs` table. Items is jsonb in the schema —
// carried as a raw byte slice so JSON marshal/unmarshal stays in the caller
// (presenter layer) and this package has no knowledge of the payload shape.
type MenuDef struct {
	ID        int64
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
	UpdatedBy string
	Title     string
	Slug      string
	Published bool
	IsAdmin   bool
	Items     []byte // jsonb — nil when NULL
}

// items is selected as VARCHAR so the row scans cleanly into []byte
// under both backends. DuckDB's JSON reader returns []any when the
// stored value is a JSON array (it parses on the way out); casting
// to VARCHAR forces the textual representation, which is what the Go
// model carries. Postgres jsonb→text round-trips without loss.
const menuDefColumns = `id, created_at, updated_at, updated_by, title, slug, published, is_admin, CAST(items AS VARCHAR) AS items`

func scanMenuDef(s scannable) (*MenuDef, error) {
	m := &MenuDef{}
	err := s.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.UpdatedBy,
		&m.Title,
		&m.Slug,
		&m.Published,
		&m.IsAdmin,
		&m.Items,
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}
