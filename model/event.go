package model

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
)

// Event mirrors the `events` table. EventDate is stored as a non-null
// timestamptz (schema contract), while the optional-content columns use
// sql.NullString so callers can distinguish missing from empty.
type Event struct {
	ID            int64
	CreatedAt     sql.NullTime
	UpdatedAt     sql.NullTime
	UpdatedBy     string
	Published     bool
	Title         string
	Slug          string
	Summary       sql.NullString
	Body          sql.NullString
	EventDate     time.Time
	EventTime     string
	EventLocation sql.NullString
	ContactPerson sql.NullString
	ContactPhone  sql.NullString
	ContactEmail  sql.NullString
	ContactURL    sql.NullString
	Categories    pq.StringArray
}

const eventColumns = `id, created_at, updated_at, updated_by, published, title, slug, ` +
	`summary, body, event_date, event_time, event_location, ` +
	`contact_person, contact_phone, contact_email, contact_url, categories`

func scanEvent(s scannable) (*Event, error) {
	e := &Event{}
	err := s.Scan(
		&e.ID,
		&e.CreatedAt,
		&e.UpdatedAt,
		&e.UpdatedBy,
		&e.Published,
		&e.Title,
		&e.Slug,
		&e.Summary,
		&e.Body,
		&e.EventDate,
		&e.EventTime,
		&e.EventLocation,
		&e.ContactPerson,
		&e.ContactPhone,
		&e.ContactEmail,
		&e.ContactURL,
		&e.Categories,
	)
	if err != nil {
		return nil, err
	}
	return e, nil
}
