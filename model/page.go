package model

import (
	"database/sql"

	"github.com/lib/pq"
)

// Page mirrors the `pages` table. Data is the jsonb module list — carried as
// a raw byte slice so the JSON shape stays in the page/ presenter layer and
// this package stays unaware of the module schema. AvailablePositions is a
// Postgres text[]; on the DuckDB cutover this will be swapped for a
// StringSlice wrapper in model/scan_types.go.
type Page struct {
	ID                 int64
	CreatedAt          sql.NullTime
	UpdatedAt          sql.NullTime
	UpdatedBy          string
	Title              string
	Slug               string
	Published          bool
	IsHome             bool
	IsAdmin            bool
	AvailablePositions pq.StringArray
	Data               []byte // jsonb — nil when NULL
}

const pageColumns = `id, created_at, updated_at, updated_by, title, slug, published, is_home, is_admin, available_positions, data`

func scanPage(s scannable) (*Page, error) {
	p := &Page{}
	err := s.Scan(
		&p.ID,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.UpdatedBy,
		&p.Title,
		&p.Slug,
		&p.Published,
		&p.IsHome,
		&p.IsAdmin,
		&p.AvailablePositions,
		&p.Data,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}
