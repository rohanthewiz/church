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
	condition := "event_date >= '" + startDate + "' AND event_date <= '" + endDate + "'"
	evts, err := models.Events(dbH, qm.Where(condition), qm.OrderBy("event_date ASC"), qm.Limit(100)).All()
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