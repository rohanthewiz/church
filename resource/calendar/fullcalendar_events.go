package calendar

import (
	"github.com/rohanthewiz/church/models"
	"github.com/vattle/sqlboiler/queries/qm"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
	tu "github.com/rohanthewiz/church/util/timeutil"
	"github.com/labstack/echo"
	"net/http"
	"strconv"
)

// API Response
type FCResponse struct {
	Events []FullcalendarEvent `json:"events"`
}

type FullcalendarEvent struct {
	Title string `json:"title"`
	Start string `json:"start"`
	End string `json:"end, omitempty"`
	AllDay bool `json:"allDay"`
	Url string `json:"url"`
}

// Return events between the given dates as FullCalendar events
func GetFullCalendarEvents(c echo.Context) (err error) {
	var startDate, endDate string
	startDate = c.QueryParam("start")
	endDate = c.QueryParam("end")
	var fEvents []FullcalendarEvent
	db, err := db.Db()
	if err != nil { return err }
	condition := "event_date >= '" + startDate + "' AND event_date <= '" + endDate + "'"
	evts, err := models.Events(db, qm.Where(condition), qm.OrderBy("event_date ASC"), qm.Limit(100)).All()
	if err != nil {
		return serr.Wrap(err, "Error obtaining events")
	}

	for _, evt := range evts {
		fe := FullcalendarEvent{ AllDay: true } // We have no end date
		fe.Title = evt.Title
		fe.Start = evt.EventDate.Format(tu.ISO8601DateTime)
		fe.Url = "/events/" + strconv.FormatInt(evt.ID, 10)
		fEvents = append(fEvents, fe)
	}

	return c.JSON(http.StatusOK, &fEvents)
}
