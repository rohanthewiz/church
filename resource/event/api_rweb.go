package event

import (
	"net/http"
	"strconv"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/resource/apiv1"
	"github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
	"github.com/vattle/sqlboiler/queries/qm"
)

// EventAPI is the public JSON DTO for an event — a deliberate subset of the
// model; never serialize presenters/models directly.
//
// event_date is a plain date (YYYY-MM-DD) and event_time free-form text,
// mirroring how the schema stores them — the app composes them for display
// rather than the server guessing a timezone.
type EventAPI struct {
	ID            int64    `json:"id"`
	Title         string   `json:"title"`
	Slug          string   `json:"slug"`
	Summary       string   `json:"summary"`
	EventDate     string   `json:"event_date"`
	EventTime     string   `json:"event_time"`
	EventLocation string   `json:"event_location"`
	ContactPerson string   `json:"contact_person"`
	ContactPhone  string   `json:"contact_phone"`
	ContactEmail  string   `json:"contact_email"`
	ContactURL    string   `json:"contact_url"`
	Categories    []string `json:"categories"`
	Body          string   `json:"body,omitempty"`
}

func eventToAPI(evt *models.Event, includeBody bool) EventAPI {
	e := EventAPI{
		ID:            evt.ID,
		Title:         evt.Title,
		Slug:          evt.Slug,
		Summary:       evt.Summary.String,
		EventDate:     evt.EventDate.Format(timeutil.ISO8601Date),
		EventTime:     evt.EventTime,
		EventLocation: evt.EventLocation.String,
		ContactPerson: evt.ContactPerson.String,
		ContactPhone:  evt.ContactPhone.String,
		ContactEmail:  evt.ContactEmail.String,
		ContactURL:    evt.ContactURL.String,
		Categories:    evt.Categories,
	}
	if e.Categories == nil {
		e.Categories = []string{}
	}
	if includeBody {
		e.Body = evt.Body.String
	}
	return e
}

// GET /api/v1/events?from&to&limit&offset
// Published events. With no date params, defaults to upcoming (today onward),
// soonest first — the common mobile ask. from/to (YYYY-MM-DD) widen or shift
// the window, e.g. for a month-view calendar.
func APIEventsRWeb(ctx rweb.Context) error {
	limit, offset := apiv1.ParseLimitOffset(ctx, 50, 200)

	mods := []qm.QueryMod{
		qm.Where("published = true"),
		qm.OrderBy("event_date ASC"),
		qm.Limit(limit),
		qm.Offset(offset),
	}

	from := ctx.Request().QueryParam("from")
	to := ctx.Request().QueryParam("to")
	// Validate dates by parsing, then bind — user input never enters SQL raw
	if from != "" {
		if _, err := time.Parse(timeutil.ISO8601Date, from); err != nil {
			return apiv1.Error(ctx, http.StatusBadRequest, "from must be YYYY-MM-DD")
		}
		mods = append(mods, qm.Where("event_date >= ?", from))
	}
	if to != "" {
		if _, err := time.Parse(timeutil.ISO8601Date, to); err != nil {
			return apiv1.Error(ctx, http.StatusBadRequest, "to must be YYYY-MM-DD")
		}
		mods = append(mods, qm.Where("event_date <= ?", to))
	}
	if from == "" && to == "" {
		mods = append(mods, qm.Where("event_date >= CURRENT_DATE"))
	}

	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	evts, err := models.Events(dbH, mods...).All()
	if err != nil {
		return serr.Wrap(err, "Error obtaining events")
	}

	events := make([]EventAPI, 0, len(evts))
	for _, evt := range evts {
		events = append(events, eventToAPI(evt, false))
	}

	return ctx.WriteJSON(map[string]any{
		"events": events,
		"limit":  limit,
		"offset": offset,
	})
}

// GET /api/v1/events/:id — single event including body.
func APIEventRWeb(ctx rweb.Context) error {
	id, err := strconv.ParseInt(ctx.Request().Param("id"), 10, 64)
	if err != nil {
		return apiv1.Error(ctx, http.StatusBadRequest, "event id must be an integer")
	}

	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	// Drafts 404 identically to nonexistent ids — no oracle for unpublished content
	evt, err := models.Events(dbH, qm.Where("id = ? AND published = true", id)).One()
	if err != nil {
		logger.LogErr(err, "event not found for API", "id", ctx.Request().Param("id"))
		return apiv1.Error(ctx, http.StatusNotFound, "Event not found")
	}

	return ctx.WriteJSON(eventToAPI(evt, true))
}

// UpcomingEventsAPI returns the next published events from today onward.
// Exported for the /api/v1/feed aggregator.
func UpcomingEventsAPI(limit int) ([]EventAPI, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	evts, err := models.Events(dbH, qm.Where("published = true AND event_date >= CURRENT_DATE"),
		qm.OrderBy("event_date ASC"), qm.Limit(limit)).All()
	if err != nil {
		return nil, serr.Wrap(err, "Error obtaining upcoming events")
	}
	out := make([]EventAPI, 0, len(evts))
	for _, evt := range evts {
		out = append(out, eventToAPI(evt, false))
	}
	return out, nil
}
