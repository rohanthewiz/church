package event

import (
	"net/http"
	"sort"
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
//
// Recurring events appear as multiple entries in list responses — one per
// occurrence in the requested window — all sharing the base event's id (an
// occurrence is not a row; the detail endpoint always returns the base
// event). recurring/recurrence_desc let clients badge them.
type EventAPI struct {
	ID             int64    `json:"id"`
	Title          string   `json:"title"`
	Slug           string   `json:"slug"`
	Summary        string   `json:"summary"`
	EventDate      string   `json:"event_date"`
	EventTime      string   `json:"event_time"`
	EventLocation  string   `json:"event_location"`
	ContactPerson  string   `json:"contact_person"`
	ContactPhone   string   `json:"contact_phone"`
	ContactEmail   string   `json:"contact_email"`
	ContactURL     string   `json:"contact_url"`
	Categories     []string `json:"categories"`
	Recurring      bool     `json:"recurring"`
	RecurrenceDesc string   `json:"recurrence_desc,omitempty"`
	Body           string   `json:"body,omitempty"`

	// Detail endpoint only: the structured rule, for edit UIs / calendar export
	Recurrence *RecurrenceAPI `json:"recurrence,omitempty"`
}

// RecurrenceAPI is the wire form of a Recurrence rule.
type RecurrenceAPI struct {
	Freq    string `json:"freq"`              // "weekly" | "monthly"
	Weekday int    `json:"weekday"`           // 0=Sunday .. 6=Saturday
	Week    int    `json:"week,omitempty"`    // monthly: 1..4, -1=last
	Until   string `json:"until,omitempty"`   // YYYY-MM-DD, empty = open-ended
	Desc    string `json:"desc"`              // human-readable, e.g. "Second Saturday of each month"
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

// DefaultWindowDays bounds event listing when the caller gives no explicit
// range. Recurrence expansion requires a finite horizon — an open-ended
// "everything upcoming" is meaningless once weekly events exist (a weekly
// series alone would be infinite). One quarter is plenty for mobile scrolling;
// month views pass explicit from/to.
const DefaultWindowDays = 92

// baseEventsCap bounds the pre-expansion window query; hitting it is logged
// (never truncate silently)
const baseEventsCap = 500

// WindowedEvents returns published events with event_date in [from, to],
// with recurring events expanded into one entry per occurrence, sorted by
// date. Shared by the JSON API, the home feed, and the website's FullCalendar
// endpoint so all three always agree on what happens when.
func WindowedEvents(from, to time.Time) ([]EventAPI, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}

	evts, err := models.Events(dbH,
		qm.Where("published = true AND event_date >= ? AND event_date <= ?",
			from.Format(timeutil.ISO8601Date), to.Format(timeutil.ISO8601Date)),
		qm.OrderBy("event_date ASC"), qm.Limit(baseEventsCap)).All()
	if err != nil {
		return nil, serr.Wrap(err, "Error obtaining events")
	}
	if len(evts) == baseEventsCap {
		logger.Log("Warn", "event window query hit its cap; results may be truncated",
			"cap", strconv.Itoa(baseEventsCap))
	}

	recs, err := allRecurrences()
	if err != nil {
		return nil, err
	}
	recByEventID := make(map[int64]Recurrence, len(recs))
	for _, rec := range recs {
		recByEventID[rec.EventID] = rec
	}

	events := make([]EventAPI, 0, len(evts))
	for _, evt := range evts {
		dto := eventToAPI(evt, false)
		if rec, ok := recByEventID[evt.ID]; ok {
			dto.Recurring = true
			dto.RecurrenceDesc = rec.Describe()
		}
		events = append(events, dto)
	}

	// Expand each rule into occurrence entries. Base events are fetched by id
	// (not the window query) because a series anchored before the window must
	// still produce occurrences inside it.
	for _, rec := range recs {
		baseEvt, err := models.Events(dbH,
			qm.Where("id = ? AND published = true", rec.EventID)).One()
		if err != nil {
			continue // unpublished or deleted base — series is off
		}
		occurrences := rec.Occurrences(baseEvt.EventDate, from, to)
		if len(occurrences) == 0 {
			continue
		}
		dto := eventToAPI(baseEvt, false)
		dto.Recurring = true
		dto.RecurrenceDesc = rec.Describe()
		for _, occ := range occurrences {
			occDto := dto // copy; slices inside (Categories) are shared read-only
			occDto.EventDate = occ.Format(timeutil.ISO8601Date)
			events = append(events, occDto)
		}
	}

	// EventDate is YYYY-MM-DD so lexical order is date order; title breaks ties
	// for a stable listing
	sort.Slice(events, func(i, j int) bool {
		if events[i].EventDate != events[j].EventDate {
			return events[i].EventDate < events[j].EventDate
		}
		return events[i].Title < events[j].Title
	})
	return events, nil
}

// GET /api/v1/events?from&to&limit&offset
// Published events including recurring occurrences, soonest first. With no
// date params, defaults to the upcoming quarter; from/to (YYYY-MM-DD) shift
// the window, e.g. for a month-view calendar.
func APIEventsRWeb(ctx rweb.Context) error {
	limit, offset := apiv1.ParseLimitOffset(ctx, 50, 200)

	// Validate then parse the window — user input never reaches SQL raw
	fromStr := ctx.Request().QueryParam("from")
	toStr := ctx.Request().QueryParam("to")
	from := time.Now()
	if fromStr != "" {
		var err error
		if from, err = time.Parse(timeutil.ISO8601Date, fromStr); err != nil {
			return apiv1.Error(ctx, http.StatusBadRequest, "from must be YYYY-MM-DD")
		}
	}
	to := from.AddDate(0, 0, DefaultWindowDays)
	if toStr != "" {
		var err error
		if to, err = time.Parse(timeutil.ISO8601Date, toStr); err != nil {
			return apiv1.Error(ctx, http.StatusBadRequest, "to must be YYYY-MM-DD")
		}
	}

	events, err := WindowedEvents(from, to)
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not load events")
	}

	// Paging happens after expansion/merge — SQL-level offsets would count base
	// rows, not occurrences, and skip or double events at page boundaries
	pageEnd := min(offset+limit, len(events))
	page := []EventAPI{}
	if offset < len(events) {
		page = events[offset:pageEnd]
	}

	return ctx.WriteJSON(map[string]any{
		"events": page,
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
		return apiv1.ServerError(ctx, err, "Could not load event")
	}
	// Drafts 404 identically to nonexistent ids — no oracle for unpublished content
	evt, err := models.Events(dbH, qm.Where("id = ? AND published = true", id)).One()
	if err != nil {
		logger.LogErr(err, "event not found for API", "id", ctx.Request().Param("id"))
		return apiv1.Error(ctx, http.StatusNotFound, "Event not found")
	}

	dto := eventToAPI(evt, true)
	if rec, found, err := GetRecurrence(evt.ID); err != nil {
		logger.LogErr(err, "could not load recurrence for event detail", "id", ctx.Request().Param("id"))
	} else if found {
		dto.Recurring = true
		dto.RecurrenceDesc = rec.Describe()
		recAPI := RecurrenceAPI{
			Freq:    rec.Freq,
			Weekday: int(rec.Weekday),
			Week:    rec.Week,
			Desc:    rec.Describe(),
		}
		if !rec.Until.IsZero() {
			recAPI.Until = rec.Until.Format(timeutil.ISO8601Date)
		}
		dto.Recurrence = &recAPI
	}

	return ctx.WriteJSON(dto)
}

// UpcomingEventsAPI returns the next published events from today onward
// (recurring occurrences included) within the default window.
// Exported for the /api/v1/feed aggregator.
func UpcomingEventsAPI(limit int) ([]EventAPI, error) {
	now := time.Now()
	events, err := WindowedEvents(now, now.AddDate(0, 0, DefaultWindowDays))
	if err != nil {
		return nil, err
	}
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}
