package model

import (
	"fmt"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

func EventByID(id int64) (*Event, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + eventColumns + ` FROM events WHERE id = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), id)
	e, err := scanEvent(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving event by id", "id", fmt.Sprintf("%d", id))
	}
	return e, nil
}

// QueryEvents: same trust model as QueryArticles — condition/order are
// trusted fragments built from internal module config, not user input.
func QueryEvents(condition, order string, limit, offset int64) ([]*Event, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	q := `SELECT ` + eventColumns + ` FROM events`
	if condition != "" {
		q += ` WHERE ` + condition
	}
	if order != "" {
		q += ` ORDER BY ` + order
	}
	q += ` LIMIT ? OFFSET ?`

	rows, err := dbH.Query(db.Rebind(db.CurrentDialect(), q), limit, offset)
	if err != nil {
		return nil, serr.Wrap(err, "Error querying events")
	}
	defer rows.Close()

	var out []*Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, serr.Wrap(err, "Error scanning event row")
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "Error iterating event rows")
	}
	return out, nil
}

func InsertEvent(e *Event) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		INSERT INTO events
			(created_at, updated_at, updated_by, published, title, slug,
			 summary, body, event_date, event_time, event_location,
			 contact_person, contact_phone, contact_email, contact_url, categories)
		VALUES
			(NOW(), NOW(), ?, ?, ?, ?,
			 ?, ?, ?, ?, ?,
			 ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		e.UpdatedBy, e.Published, e.Title, e.Slug,
		e.Summary, e.Body, e.EventDate, e.EventTime, e.EventLocation,
		e.ContactPerson, e.ContactPhone, e.ContactEmail, e.ContactURL, e.Categories,
	)
	if err := row.Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error inserting event")
	}
	return nil
}

func UpdateEvent(e *Event) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		UPDATE events SET
			updated_at      = NOW(),
			updated_by      = ?,
			published       = ?,
			title           = ?,
			slug            = ?,
			summary         = ?,
			body            = ?,
			event_date      = ?,
			event_time      = ?,
			event_location  = ?,
			contact_person  = ?,
			contact_phone   = ?,
			contact_email   = ?,
			contact_url     = ?,
			categories      = ?
		WHERE id = ?
		RETURNING updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		e.UpdatedBy, e.Published, e.Title, e.Slug,
		e.Summary, e.Body, e.EventDate, e.EventTime, e.EventLocation,
		e.ContactPerson, e.ContactPhone, e.ContactEmail, e.ContactURL, e.Categories,
		e.ID,
	)
	if err := row.Scan(&e.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error updating event", "id", fmt.Sprintf("%d", e.ID))
	}
	return nil
}

func DeleteEvent(id int64) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `DELETE FROM events WHERE id = ?`
	if _, err := dbH.Exec(db.Rebind(db.CurrentDialect(), q), id); err != nil {
		return serr.Wrap(err, "Error deleting event", "id", fmt.Sprintf("%d", id))
	}
	return nil
}
