package model

import (
	"database/sql"
	"time"
)

// Sermon mirrors the `sermons` table.
// Slug is nullable in the schema (column has no NOT NULL — only the unique
// index enforces distinctness), so it is kept as sql.NullString even though
// in practice every new row gets a slug assigned by the presenter.
// DateTaught is a plain `timestamp` (no tz) in the schema and stored as a
// time.Time — the presenter layer handles timezone coercion for display.
type Sermon struct {
	ID            int64
	CreatedAt     sql.NullTime
	UpdatedAt     sql.NullTime
	UpdatedBy     string
	Title         string
	Slug          sql.NullString
	Published     bool
	Summary       sql.NullString
	Body          sql.NullString
	AudioLink     sql.NullString
	DateTaught    time.Time
	PlaceTaught   sql.NullString
	Teacher       string
	ScriptureRefs StringSlice
	Categories    StringSlice
}

const sermonColumns = `id, created_at, updated_at, updated_by, title, slug, published, summary, body, audio_link, date_taught, place_taught, teacher, scripture_refs, categories`

func scanSermon(s scannable) (*Sermon, error) {
	m := &Sermon{}
	err := s.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.UpdatedBy,
		&m.Title,
		&m.Slug,
		&m.Published,
		&m.Summary,
		&m.Body,
		&m.AudioLink,
		&m.DateTaught,
		&m.PlaceTaught,
		&m.Teacher,
		&m.ScriptureRefs,
		&m.Categories,
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}
