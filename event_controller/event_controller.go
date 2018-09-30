package event_controller

import (
	"github.com/labstack/echo"
	ctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/resource/event"
	"github.com/rohanthewiz/church/app"
	"errors"
	"github.com/rohanthewiz/church/page"
	"strings"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/util/stringops"
	"fmt"
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/auth_controller"
)

func NewEvent(c echo.Context) error {
	pg, err := page.EventForm()
	if err != nil { c.Error(err); return err }
	c.HTMLBlob(200, base.RenderPageNew(pg, c))
	return  nil
}

// Show a particular event - for given by id
func ShowEvent(c echo.Context) error {
	pg, err := page.EventWithUpcomingEvents() //page.EventShow()
	if err != nil { c.Error(err); return err }
	c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return  nil
}

func ListEvents(c echo.Context) error {
	pg, err := page.EventsList()
	if err != nil {	c.Error(err); return err }
	c.HTMLBlob(200, base.RenderPageList(pg, c))
	return  nil
}

func AdminListEvents(c echo.Context) error {
	pg, err := page.AdminEventsList()
	if err != nil { c.Error(err); return err }
	c.HTMLBlob(200, base.RenderPageList(pg, c))
	return  nil
}

func EditEvent(c echo.Context) error {
	pg, err := page.EventForm()
	if err != nil { c.Error(err); return err }
	auth_controller.SetFormReferrer(c) // save the referrer calling for edit
	c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return  nil
}

func UpsertEvent(c echo.Context) error {
	csrf := c.FormValue("csrf")
	// At the action func (example UpsertEvent), check that this token is present and valid in Redis
	if !app.VerifyFormToken(csrf) {
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		c.Error(err)
		return err
	}
	// apparently embedded fields cannot be set immediately in  a literal struct
	// we'll set those after efs is created
	efs := event.Presenter{
		EventDate: strings.TrimSpace(c.FormValue("event_date")),
		EventTime: strings.TrimSpace(c.FormValue("event_time")),
		Location: strings.TrimSpace(c.FormValue("event_location")),
		ContactPerson: strings.TrimSpace(c.FormValue("contact_person")),
		ContactPhone: strings.TrimSpace(c.FormValue("contact_phone")),
		ContactEmail: strings.TrimSpace(c.FormValue("contact_email")),
		ContactURL: strings.TrimSpace(c.FormValue("contact_url")),
	}
	// set embedded fields etc
	efs.Id = c.FormValue("event_id")
	efs.Title = strings.TrimSpace(c.FormValue("event_title"))
	efs.Summary = c.FormValue("event_summary")
	efs.Body = c.FormValue("event_body")
	efs.Categories = stringops.StringSplitAndTrim(c.FormValue("categories"), ",")
	efs.UpdatedBy = c.(*ctx.CustomContext).Username
	if c.FormValue("published") == "on" {
		efs.Published = true
	}

	err := efs.UpsertEvent()
	if err != nil {
		logger.LogErr(err, "Error in event upsert", "event_presenter", fmt.Sprintf("%#v", efs))
		c.Error(err)
		return err
	}
	msg := "Created"
	if efs.Id != "0" && efs.Id != "" {
		msg = "Updated"
	}

	redirectTo := "/admin/events"
	if cc, ok := c.(*ctx.CustomContext); ok && cc.FormReferrer != "" {
		redirectTo = cc.FormReferrer // return to the form caller
	}
	app.Redirect(c, redirectTo, "Event " + msg)
	return nil
}


func DeleteEvent(c echo.Context) error {
	err := event.DeleteEventById(c.Param("id"))
	msg := "Event with id: " + c.Param("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete event with id: " + c.Param("id")
		logger.LogErrAsync(err, "when", "deleting event")
	}
	app.Redirect(c, "/admin/events", msg)
	return nil
}
