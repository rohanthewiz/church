package model

import (
	"fmt"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

func SermonByID(id int64) (*Sermon, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + sermonColumns + ` FROM sermons WHERE id = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), id)
	m, err := scanSermon(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving sermon by id", "id", fmt.Sprintf("%d", id))
	}
	return m, nil
}

func SermonBySlug(slug string) (*Sermon, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + sermonColumns + ` FROM sermons WHERE slug = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), slug)
	m, err := scanSermon(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving sermon by slug", "slug", slug)
	}
	return m, nil
}

// QuerySermons accepts already-formed condition and order fragments — same
// trust model as the rest of the module: fragments come from internal module
// config, not user input.
func QuerySermons(condition, order string, limit, offset int64) ([]*Sermon, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}

	q := `SELECT ` + sermonColumns + ` FROM sermons`
	if condition != "" {
		q += ` WHERE ` + condition
	}
	if order != "" {
		q += ` ORDER BY ` + order
	}
	q += ` LIMIT ? OFFSET ?`

	rows, err := dbH.Query(db.Rebind(db.CurrentDialect(), q), limit, offset)
	if err != nil {
		return nil, serr.Wrap(err, "Error querying sermons")
	}
	defer rows.Close()

	var out []*Sermon
	for rows.Next() {
		m, err := scanSermon(rows)
		if err != nil {
			return nil, serr.Wrap(err, "Error scanning sermon row")
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "Error iterating sermon rows")
	}
	return out, nil
}

func InsertSermon(m *Sermon) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		INSERT INTO sermons
			(created_at, updated_at, updated_by, title, slug, published, summary, body,
			 audio_link, date_taught, place_taught, teacher, scripture_refs, categories)
		VALUES
			(NOW(), NOW(), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		m.UpdatedBy, m.Title, m.Slug, m.Published, m.Summary, m.Body,
		m.AudioLink, m.DateTaught, m.PlaceTaught, m.Teacher, m.ScriptureRefs, m.Categories,
	)
	if err := row.Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error inserting sermon")
	}
	return nil
}

func UpdateSermon(m *Sermon) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		UPDATE sermons SET
			updated_at     = NOW(),
			updated_by     = ?,
			title          = ?,
			slug           = ?,
			published      = ?,
			summary        = ?,
			body           = ?,
			audio_link     = ?,
			date_taught    = ?,
			place_taught   = ?,
			teacher        = ?,
			scripture_refs = ?,
			categories     = ?
		WHERE id = ?
		RETURNING updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		m.UpdatedBy, m.Title, m.Slug, m.Published, m.Summary, m.Body,
		m.AudioLink, m.DateTaught, m.PlaceTaught, m.Teacher, m.ScriptureRefs, m.Categories, m.ID,
	)
	if err := row.Scan(&m.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error updating sermon", "id", fmt.Sprintf("%d", m.ID))
	}
	return nil
}

func DeleteSermon(id int64) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `DELETE FROM sermons WHERE id = ?`
	if _, err := dbH.Exec(db.Rebind(db.CurrentDialect(), q), id); err != nil {
		return serr.Wrap(err, "Error deleting sermon", "id", fmt.Sprintf("%d", id))
	}
	return nil
}
