package model

import (
	"database/sql"
)

// Page mirrors the `pages` table. Data is the jsonb (Postgres) / JSON
// (DuckDB) module list — carried as a raw byte slice so the JSON shape
// stays in the page/ presenter layer and this package stays unaware of
// the module schema. AvailablePositions uses StringSlice which handles
// both Postgres text[] and DuckDB VARCHAR[] in model/types.go.
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
	AvailablePositions StringSlice
	Data               []byte // jsonb / JSON — nil when NULL
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
