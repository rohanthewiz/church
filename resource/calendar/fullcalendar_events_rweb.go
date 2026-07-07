package calendar

import (
	"strconv"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	tu "github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
	"github.com/vattle/sqlboiler/queries/qm"
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

// Return events between the given dates as FullCalendar events
func GetFullCalendarEventsRWeb(ctx rweb.Context) error {
	var startDate, endDate string
	startDate = ctx.Request().QueryParam("start")
	endDate = ctx.Request().QueryParam("end")
	var fEvents []FullcalendarEvent
	dbH, err := db.Db()
	if err != nil {
		return err
	}
	// Bind the range bounds as placeholders — start/end arrive straight from the
	// query string, so concatenating them into the WHERE clause would be an SQL
	// injection vector. SQLBoiler rewrites `?` to the Postgres `$n` form for us.
	evts, err := models.Events(dbH,
		qm.Where("event_date >= ? AND event_date <= ?", startDate, endDate),
		qm.OrderBy("event_date ASC"), qm.Limit(100)).All()
	if err != nil {
		return serr.Wrap(err, "Error obtaining events")
	}

	for _, evt := range evts {
		fe := FullcalendarEvent{AllDay: true} // We have no end date
		fe.Title = evt.Title
		fe.Start = evt.EventDate.Format(tu.ISO8601DateTime)
		fe.Url = "/events/" + strconv.FormatInt(evt.ID, 10)
		fEvents = append(fEvents, fe)
	}

	return ctx.WriteJSON(&fEvents)
}