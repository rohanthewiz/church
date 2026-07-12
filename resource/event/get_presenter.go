package event

import (
	"fmt"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/resource/content"
	. "github.com/rohanthewiz/logger"
)

type Presenter struct {
	content.Content
	EventDate             string
	EventDateDisplayLong  string
	EventDateDisplayShort string
	EventTime             string
	Location              string
	ContactPerson         string
	ContactPhone          string
	ContactEmail          string
	ContactURL            string
	// Recurrence rule as it travels through the admin form (string form
	// values; parsed and validated in UpsertEvent). Empty RecurFreq means a
	// one-time event. Populated from event_recurrences by LoadRecurrence —
	// deliberately not in presenterFromModel, which runs per row in list
	// views and would N+1 the recurrence table.
	RecurFreq    string // "", "weekly", "monthly"
	RecurWeekday string // "0" (Sunday) .. "6" (Saturday)
	RecurWeek    string // monthly: "1".."4" or "-1" (last)
	RecurUntil   string // YYYY-MM-DD or "" for open-ended
}

// LoadRecurrence fills the presenter's recurrence fields from the DB.
// Call after presenterFromModel when editing a single event.
func (p *Presenter) LoadRecurrence(exec db.Executor, eventID int64) error {
	rec, found, err := GetRecurrence(exec, eventID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	p.RecurFreq = rec.Freq
	p.RecurWeekday = fmt.Sprintf("%d", int(rec.Weekday))
	if rec.Freq == RecurMonthly {
		p.RecurWeek = fmt.Sprintf("%d", rec.Week)
	}
	if !rec.Until.IsZero() {
		p.RecurUntil = rec.Until.Format("2006-01-02")
	}
	return nil
}

type PresenterParams struct {
	TimeNormalFormat string
	DateLongFormat   string
	DateTimeFormat   string
}

// Fix up Presenter for Web
func presenterFromModel(evt *models.Event, params ...PresenterParams) Presenter {
	timeDisplayFormat := config.DisplayTimeFormat
	dateDisplayLongFormat := config.DisplayDateFormatLong
	dateTimeDisplayFormat := config.DisplayDateTimeFormat
	if len(params) > 0 {
		if params[0].TimeNormalFormat != "" {
			timeDisplayFormat = params[0].TimeNormalFormat
		}
		if params[0].DateLongFormat != "" {
			dateDisplayLongFormat = params[0].DateLongFormat
		}
		if params[0].DateTimeFormat != "" {
			dateTimeDisplayFormat = params[0].DateTimeFormat
		}
	}

	pres := Presenter{}
	if evt.CreatedAt.Valid {
		pres.CreatedAt = evt.CreatedAt.Time.Format(dateTimeDisplayFormat)
	}
	if evt.UpdatedAt.Valid {
		pres.UpdatedAt = evt.UpdatedAt.Time.Format(dateTimeDisplayFormat)
	}

	// Generic Content
	pres.Title = evt.Title
	pres.Slug = evt.Slug

	cats := []string{}
	for _, cat := range evt.Categories {
		cats = append(cats, cat)
	}
	pres.Categories = cats
	pres.Id = fmt.Sprintf("%d", evt.ID)
	pres.Summary = evt.Summary.String
	pres.Body = evt.Body.String
	pres.Published = evt.Published
	pres.UpdatedBy = evt.UpdatedBy

	// Presenter specific content
	earliest_date, err := time.Parse("2006-01-02", "2010-01-01")
	if err == nil && evt.EventDate.After(earliest_date) {
		loc, err := time.LoadLocation("Local")
		if err != nil {
			Log("Error", "Failed to load location 'Local'")
		} else {
			localEventDate := evt.EventDate.In(loc)
			// fmt.Println("[Debug] localEventDate.Format(config.IncomingDateTimeFormat):",
			//		localEventDate.Format(config.IncomingDateTimeFormat))                     // debug
			pres.EventDate = localEventDate.Format(config.PresenterDateFormat)                // Admin form requires this format //
			pres.EventTime = localEventDate.Format(timeDisplayFormat)                         // Admin form requires this format
			pres.EventDateDisplayLong = localEventDate.Format(dateDisplayLongFormat)          // For non-admin
			pres.EventDateDisplayShort = localEventDate.Format(config.DisplayShortDateFormat) // For non-admin
		}
	}
	pres.Location = evt.EventLocation.String
	pres.ContactPerson = evt.ContactPerson.String
	pres.ContactPhone = evt.ContactPhone.String
	pres.ContactEmail = evt.ContactEmail.String
	pres.ContactURL = evt.ContactURL.String
	return pres
}
