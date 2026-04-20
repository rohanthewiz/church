// Package model holds the hand-written data types and DAOs that replace
// the SQLBoiler-generated `models/` package. Goals:
//   - Thin wrappers over database/sql — no reflection, no query builder DSL.
//   - Stdlib-first types (sql.NullString/Time) plus the local StringSlice
//     wrapper (model/types.go) for text[] / VARCHAR[] and json.RawMessage
//     for jsonb / JSON. Types are driver-neutral — the dialect branch lives
//     inside StringSlice's Scanner/Valuer, not in the struct shapes.
//   - All SQL written with `?` placeholders and rebound by db.Rebind at call
//     time — same source for Postgres and DuckDB.
package model

import (
	"database/sql"
)

// Article mirrors the `articles` table. Field names stay CamelCase matching
// Go convention; column order below tracks the SELECT/INSERT lists in the DAO.
type Article struct {
	ID         int64
	CreatedAt  sql.NullTime
	UpdatedAt  sql.NullTime
	UpdatedBy  string
	Title      string
	Slug       string
	Summary    string
	Body       sql.NullString
	Published  bool
	Categories StringSlice
}

// scannable lets the same scan helper consume *sql.Row and *sql.Rows.
// Keeping a single Scan path per table means adding a column is a one-line
// change in the SELECT list plus one line here.
type scannable interface {
	Scan(dest ...any) error
}

// articleColumns is the canonical SELECT list. Callers that build ad-hoc
// queries (QueryArticles with user-supplied WHERE/ORDER) reuse this so the
// column order never drifts from scanArticle.
const articleColumns = `id, created_at, updated_at, updated_by, title, slug, summary, body, published, categories`

func scanArticle(s scannable) (*Article, error) {
	a := &Article{}
	err := s.Scan(
		&a.ID,
		&a.CreatedAt,
		&a.UpdatedAt,
		&a.UpdatedBy,
		&a.Title,
		&a.Slug,
		&a.Summary,
		&a.Body,
		&a.Published,
		&a.Categories,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}