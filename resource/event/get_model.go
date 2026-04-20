package event

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/model"
	. "github.com/rohanthewiz/logger"
)

// modelFromPresenter builds the DB-shaped struct from the web Presenter,
// and reports whether this should be an INSERT (create_op=true) or UPDATE.
func modelFromPresenter(pres Presenter) (*model.Event, bool, error) {
	var create_op bool
	m := findModelByIdOrCreate(pres.Id)
	if m.ID < 1 {
		create_op = true
	}

	if title := strings.TrimSpace(pres.Title); title != "" {
		m.Title = title
	} else {
		msg := "Presenter title is a required field when creating events"
		Log("Error", msg)
		return m, create_op, errors.New(msg)
	}

	// Slug is write-once: only set on create, to preserve external references.
	if create_op {
		pres.CreateSlug()
		m.Slug = pres.Slug
	}

	// Compose a zoned timestamp string from the separate date + time form
	// fields, then parse via the incoming-format layout. We rely on the
	// server's zone abbreviation here — acceptable because the admin is
	// assumed to be in the same zone as the deployment.
	zone, _ := time.Now().Zone()
	datetimez := pres.EventDate + " " + pres.EventTime + " " + zone
	fmt.Println("[Debug] datetimez:", datetimez) // debug
	dte, err := time.Parse(config.IncomingDateTimeFormat, datetimez)
	if err != nil {
		Log("Error", "Error parsing event date", "error", err.Error())
		return m, create_op, err
	}
	m.EventDate = dte
	m.EventTime = strings.TrimSpace(pres.EventTime) // todo - could eliminate this db attrib
	m.Published = pres.Published
	m.Summary = sql.NullString{String: strings.TrimSpace(pres.Summary), Valid: true}
	m.Body = sql.NullString{String: strings.TrimSpace(pres.Body), Valid: true}
	m.EventLocation = sql.NullString{String: strings.TrimSpace(pres.Location), Valid: true}
	m.ContactPerson = sql.NullString{String: strings.TrimSpace(pres.ContactPerson), Valid: true}
	m.ContactPhone = sql.NullString{String: strings.TrimSpace(pres.ContactPhone), Valid: true}
	m.ContactEmail = sql.NullString{String: strings.TrimSpace(pres.ContactEmail), Valid: true}
	m.ContactURL = sql.NullString{String: strings.TrimSpace(pres.ContactURL), Valid: true}
	m.UpdatedBy = strings.TrimSpace(pres.UpdatedBy)

	if len(pres.Categories) > 0 {
		// Rebuild the slice rather than appending — same reason as article:
		// the db column value round-trips through the struct so additive
		// updates would accumulate old categories.
		categories := []string{}
		for _, cat := range pres.Categories {
			if trimmed := strings.TrimSpace(cat); trimmed != "" {
				categories = append(categories, trimmed)
			}
		}
		m.Categories = categories
	} else {
		m.Categories = []string{"general"}
	}
	return m, create_op, nil
}
