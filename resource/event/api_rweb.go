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

	// Location is the event's own point, when it has one. Always present and
	// never null — see EventLocationAPI.
	Location EventLocationAPI `json:"location"`

	// Detail endpoint only: the structured rule, for edit UIs / calendar export
	Recurrence *RecurrenceAPI `json:"recurrence,omitempty"`
}

// EventLocationAPI is the wire form of an event's own point: where THIS event
// is, as opposed to where the church is.
//
// # Why a boolean rather than a null object or a null pair
//
// The same three reasons AppConfig.Location gives, and this is the second
// place they apply, which is what makes them the contract's rule rather than
// one DTO's habit. Every key is present and non-null so the client maps it
// straight into a struct with no optional fields; the thing the client must
// decide — draw THIS point or fall back to the church's — cannot be read off
// the numbers, because 0,0 is the Gulf of Guinea and not an absence; and so
// the server that knows says so outright, once.
//
// Configured is false for the great majority of events, which are held at the
// church and carry no point of their own. That is not a gap in the data: the
// church's point is site configuration, and repeating it on every event row
// would be the same fact stored in two places with no mechanism keeping them
// equal.
type EventLocationAPI struct {
	// Configured is false when the event has no point of its own. The other
	// two fields are then zero and must not be used.
	Configured bool    `json:"configured"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
}

// RecurrenceAPI is the wire form of a Recurrence rule.
type RecurrenceAPI struct {
	Freq    string `json:"freq"`            // "weekly" | "monthly"
	Weekday int    `json:"weekday"`         // 0=Sunday .. 6=Saturday
	Week    int    `json:"week,omitempty"`  // monthly: 1..4, -1=last
	Until   string `json:"until,omitempty"` // YYYY-MM-DD, empty = open-ended
	Desc    string `json:"desc"`            // human-readable, e.g. "Second Saturday of each month"
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

// pointToAPI reads one event's point out of a batch, or reports that it has
// none. A miss and an unreadable table produce the same answer here, which is
// the right one: both mean "this event has no point of its own", and the
// client's fallback is the same in each case.
func pointToAPI(points map[int64]Point, eventID int64) EventLocationAPI {
	pt, ok := points[eventID]
	if !ok {
		return EventLocationAPI{}
	}
	return EventLocationAPI{Configured: true, Latitude: pt.Latitude, Longitude: pt.Longitude}
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

	recs, err := allRecurrences(dbH)
	if err != nil {
		return nil, err
	}
	recByEventID := make(map[int64]Recurrence, len(recs))
	for _, rec := range recs {
		recByEventID[rec.EventID] = rec
	}

	// Every event that could appear in this response, in one query. The ids
	// are the window's rows plus the base event of every rule — a series
	// anchored before the window still produces occurrences inside it, so its
	// base row may not be in evts at all.
	//
	// A failure here is logged and not fatal: a list of events with no maps is
	// a usable list, and the alternative is a screen that shows nothing because
	// a sidecar table was unreadable.
	pointIDs := make([]int64, 0, len(evts)+len(recs))
	for _, evt := range evts {
		pointIDs = append(pointIDs, evt.ID)
	}
	for _, rec := range recs {
		pointIDs = append(pointIDs, rec.EventID)
	}
	points, err := EventPoints(dbH, pointIDs)
	if err != nil {
		logger.LogErr(err, "could not load event locations for window; events will have none")
	}

	events := make([]EventAPI, 0, len(evts))
	for _, evt := range evts {
		dto := eventToAPI(evt, false)
		if rec, ok := recByEventID[evt.ID]; ok {
			dto.Recurring = true
			dto.RecurrenceDesc = rec.Describe()
		}
		dto.Location = pointToAPI(points, evt.ID)
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
		// Every occurrence of a series is held at the series' place: the point
		// belongs to the base event, and an occurrence is not a row.
		dto.Location = pointToAPI(points, baseEvt.ID)
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
// 200 → {"events": [...], "limit", "offset", "has_more"} — has_more only
// speaks for the requested window; a further page of *time* may exist beyond
// `to` even when it is false.
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
	// rows, not occurrences, and skip or double events at page boundaries.
	// The full window is already in memory, so has_more is a plain length
	// check — no limit+1 probe needed here (contrast sermons/articles).
	pageEnd := min(offset+limit, len(events))
	page := []EventAPI{}
	if offset < len(events) {
		page = events[offset:pageEnd]
	}

	return ctx.WriteJSON(map[string]any{
		"events":   page,
		"limit":    limit,
		"offset":   offset,
		"has_more": pageEnd < len(events),
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
	// One event, so one lookup rather than the window query's batch. Logged and
	// not fatal for the same reason: an event detail with no map is a usable
	// screen, and the client falls back to the church's own point.
	if pt, found, err := GetEventPoint(dbH, evt.ID); err != nil {
		logger.LogErr(err, "could not load location for event detail", "id", ctx.Request().Param("id"))
	} else if found {
		dto.Location = EventLocationAPI{Configured: true, Latitude: pt.Latitude, Longitude: pt.Longitude}
	}
	if rec, found, err := GetRecurrence(dbH, evt.ID); err != nil {
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
