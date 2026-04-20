package model

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

func MenuDefByID(id int64) (*MenuDef, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + menuDefColumns + ` FROM menu_defs WHERE id = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), id)
	m, err := scanMenuDef(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving menu def by id", "id", fmt.Sprintf("%d", id))
	}
	return m, nil
}

func MenuDefBySlug(slug string) (*MenuDef, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + menuDefColumns + ` FROM menu_defs WHERE slug = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), slug)
	m, err := scanMenuDef(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving menu def by slug", "slug", slug)
	}
	return m, nil
}

// ExistsMenuDefBySlug is the idempotency check for bootstrap routines that
// need to create a menu by known slug without clobbering an existing one.
func ExistsMenuDefBySlug(slug string) (bool, error) {
	dbH, err := db.Db()
	if err != nil {
		return false, serr.Wrap(err)
	}
	const q = `SELECT 1 FROM menu_defs WHERE slug = ? LIMIT 1`
	var one int
	err = dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), slug).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, serr.Wrap(err, "Error checking menu_def existence", "slug", slug)
	}
	return true, nil
}

func QueryMenuDefs(condition, order string, limit, offset int64) ([]*MenuDef, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	q := `SELECT ` + menuDefColumns + ` FROM menu_defs`
	if condition != "" {
		q += ` WHERE ` + condition
	}
	if order != "" {
		q += ` ORDER BY ` + order
	}
	q += ` LIMIT ? OFFSET ?`

	rows, err := dbH.Query(db.Rebind(db.CurrentDialect(), q), limit, offset)
	if err != nil {
		return nil, serr.Wrap(err, "Error querying menu defs")
	}
	defer rows.Close()

	var out []*MenuDef
	for rows.Next() {
		m, err := scanMenuDef(rows)
		if err != nil {
			return nil, serr.Wrap(err, "Error scanning menu def row")
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "Error iterating menu def rows")
	}
	return out, nil
}

func InsertMenuDef(m *MenuDef) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		INSERT INTO menu_defs
			(created_at, updated_at, updated_by, title, slug, published, is_admin, items)
		VALUES
			(NOW(), NOW(), ?, ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		m.UpdatedBy, m.Title, m.Slug, m.Published, m.IsAdmin, m.Items,
	)
	if err := row.Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error inserting menu def")
	}
	return nil
}

func UpdateMenuDef(m *MenuDef) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		UPDATE menu_defs SET
			updated_at = NOW(),
			updated_by = ?,
			title      = ?,
			slug       = ?,
			published  = ?,
			is_admin   = ?,
			items      = ?
		WHERE id = ?
		RETURNING updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		m.UpdatedBy, m.Title, m.Slug, m.Published, m.IsAdmin, m.Items, m.ID,
	)
	if err := row.Scan(&m.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error updating menu def", "id", fmt.Sprintf("%d", m.ID))
	}
	return nil
}

func DeleteMenuDef(id int64) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `DELETE FROM menu_defs WHERE id = ?`
	if _, err := dbH.Exec(db.Rebind(db.CurrentDialect(), q), id); err != nil {
		return serr.Wrap(err, "Error deleting menu def", "id", fmt.Sprintf("%d", id))
	}
	return nil
}
