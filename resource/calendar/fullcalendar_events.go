package calendar

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/model"
	tu "github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/serr"
)

// API Response
type FCResponse struct {
	Events []FullcalendarEvent `json:"events"`
}

type FullcalendarEvent struct {
	Title  string `json:"title"`
	Start  string `json:"start"`
	End    string `json:"end, omitempty"`
	AllDay bool   `json:"allDay"`
	Url    string `json:"url"`
}

// GetFullCalendarEvents returns events between start/end query params as
// FullCalendar-shaped JSON. Trust boundary: start/end are interpolated
// directly into the SQL condition — the legacy behavior — so they must
// remain admin-controlled date strings. If that ever changes, parameterise.
func GetFullCalendarEvents(c echo.Context) (err error) {
	startDate := c.QueryParam("start")
	endDate := c.QueryParam("end")
	var fEvents []FullcalendarEvent

	condition := "event_date >= '" + startDate + "' AND event_date <= '" + endDate + "'"
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

	return c.JSON(http.StatusOK, &fEvents)
}
