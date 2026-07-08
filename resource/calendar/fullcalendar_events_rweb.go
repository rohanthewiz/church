package calendar

import (
	"strconv"
	"time"

	"github.com/rohanthewiz/church/resource/event"
	tu "github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/rweb"
)

// FullcalendarEvent is the JSON shape the FullCalendar JS widget expects for
// each event (https://fullcalendar.io/docs/event-object).
type FullcalendarEvent struct {
	Title  string `json:"title"`
	Start  string `json:"start"`
	End    string `json:"end,omitempty"`
	AllDay bool   `json:"allDay"`
	Url    string `json:"url"`
}

// Return events between the given dates as FullCalendar events.
// Delegates to event.WindowedEvents so the website calendar shows recurring
// occurrences ("every Sunday", "second Saturday") exactly like the mobile API
// — one source of truth for expansion.
func GetFullCalendarEventsRWeb(ctx rweb.Context) error {
	// FullCalendar sends start/end as ISO dates; parse (never concatenate into
	// SQL — this endpoint was an injection vector once, commit cb80039) and
	// fall back to the default upcoming window on anything malformed.
	from, ok := parseFCDate(ctx.Request().QueryParam("start"))
	if !ok {
		from = time.Now()
	}
	to, ok := parseFCDate(ctx.Request().QueryParam("end"))
	if !ok {
		to = from.AddDate(0, 0, event.DefaultWindowDays)
	}

	events, err := event.WindowedEvents(from, to)
	if err != nil {
		return err
	}

	fEvents := make([]FullcalendarEvent, 0, len(events))
	for _, evt := range events {
		fEvents = append(fEvents, FullcalendarEvent{
			Title:  evt.Title,
			Start:  evt.EventDate, // date-only is valid for allDay events
			AllDay: true,          // we have no end date
			Url:    "/events/" + strconv.FormatInt(evt.ID, 10),
		})
	}

	return ctx.WriteJSON(&fEvents)
}

// parseFCDate accepts the shapes FullCalendar emits: plain dates and full
// ISO8601 timestamps (of which the leading 10 chars are the date).
func parseFCDate(s string) (time.Time, bool) {
	if len(s) > 10 {
		s = s[:10]
	}
	t, err := time.Parse(tu.ISO8601Date, s)
	return t, err == nil
}
