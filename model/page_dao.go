package model

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

func PageByID(id int64) (*Page, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + pageColumns + ` FROM pages WHERE id = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), id)
	p, err := scanPage(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving page by id", "id", fmt.Sprintf("%d", id))
	}
	return p, nil
}

func PageBySlug(slug string) (*Page, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + pageColumns + ` FROM pages WHERE slug = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), slug)
	p, err := scanPage(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving page by slug", "slug", slug)
	}
	return p, nil
}

// ExistsPageBySlug is used by bootstrap routines to avoid clobbering an
// already-present page. Ignores sql.ErrNoRows — that is the expected "not
// found" case, not an error worth surfacing.
func ExistsPageBySlug(slug string) (bool, error) {
	dbH, err := db.Db()
	if err != nil {
		return false, serr.Wrap(err)
	}
	const q = `SELECT 1 FROM pages WHERE slug = ? LIMIT 1`
	var one int
	err = dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), slug).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, serr.Wrap(err, "Error checking page existence", "slug", slug)
	}
	return true, nil
}

func QueryPages(condition, order string, limit, offset int64) ([]*Page, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	q := `SELECT ` + pageColumns + ` FROM pages`
	if condition != "" {
		q += ` WHERE ` + condition
	}
	if order != "" {
		q += ` ORDER BY ` + order
	}
	q += ` LIMIT ? OFFSET ?`

	rows, err := dbH.Query(db.Rebind(db.CurrentDialect(), q), limit, offset)
	if err != nil {
		return nil, serr.Wrap(err, "Error querying pages")
	}
	defer rows.Close()

	var out []*Page
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, serr.Wrap(err, "Error scanning page row")
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "Error iterating page rows")
	}
	return out, nil
}

func InsertPage(p *Page) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		INSERT INTO pages
			(created_at, updated_at, updated_by, title, slug, published, is_home, is_admin, available_positions, data)
		VALUES
			(NOW(), NOW(), ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		p.UpdatedBy, p.Title, p.Slug, p.Published, p.IsHome, p.IsAdmin, p.AvailablePositions, p.Data,
	)
	if err := row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error inserting page")
	}
	return nil
}

func UpdatePage(p *Page) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		UPDATE pages SET
			updated_at          = NOW(),
			updated_by          = ?,
			title               = ?,
			slug                = ?,
			published           = ?,
			is_home             = ?,
			is_admin            = ?,
			available_positions = ?,
			data                = ?
		WHERE id = ?
		RETURNING updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		p.UpdatedBy, p.Title, p.Slug, p.Published, p.IsHome, p.IsAdmin, p.AvailablePositions, p.Data, p.ID,
	)
	if err := row.Scan(&p.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error updating page", "id", fmt.Sprintf("%d", p.ID))
	}
	return nil
}

func DeletePage(id int64) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `DELETE FROM pages WHERE id = ?`
	if _, err := dbH.Exec(db.Rebind(db.CurrentDialect(), q), id); err != nil {
		return serr.Wrap(err, "Error deleting page", "id", fmt.Sprintf("%d", id))
	}
	return nil
}
