package calendar

import (
	"strconv"

	"github.com/rohanthewiz/church/model"
	tu "github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

// Return events between the given dates as FullCalendar events
func GetFullCalendarEventsRWeb(ctx rweb.Context) error {
	startDate := ctx.Request().QueryParam("start")
	endDate := ctx.Request().QueryParam("end")
	var fEvents []FullcalendarEvent

	// Trust boundary same as the echo handler: start/end are interpolated
	// into the SQL condition, so they must remain admin-controlled input.
	// DuckDB (unlike Postgres) rejects empty-string timestamp literals, so
	// only include each bound when the caller actually supplied it. A bare
	// hit on /calendar with no params now returns up to `limit` events
	// rather than failing with a Conversion Error.
	var condition string
	switch {
	case startDate != "" && endDate != "":
		condition = "event_date >= '" + startDate + "' AND event_date <= '" + endDate + "'"
	case startDate != "":
		condition = "event_date >= '" + startDate + "'"
	case endDate != "":
		condition = "event_date <= '" + endDate + "'"
	}
	evts, err := model.QueryEvents(condition, "event_date ASC", 100, 0)
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
