package event_controller

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/event"
	"github.com/rohanthewiz/church/util/stringops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

func NewEventRWeb(ctx rweb.Context) error {
	pg, err := page.EventForm()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageNewRWeb(pg, ctx)))
}

// Show a particular event - for given by id
func ShowEventRWeb(ctx rweb.Context) error {
	pg, err := page.EventWithUpcomingEvents() // page.EventShow()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageSingleRWeb(pg, ctx)))
}

func ListEventsRWeb(ctx rweb.Context) error {
	pg, err := page.EventsList()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageListRWeb(pg, ctx)))
}

func AdminListEventsRWeb(ctx rweb.Context) error {
	pg, err := page.AdminEventsList()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageListRWeb(pg, ctx)))
}

func EditEventRWeb(ctx rweb.Context) error {
	pg, err := page.EventForm()
	if err != nil {
		return err
	}
	cctx.SetFormReferrerRWeb(ctx) // save the referrer calling for edit
	return ctx.WriteHTML(string(base.RenderPageSingleRWeb(pg, ctx)))
}

func UpsertEventRWeb(ctx rweb.Context) error {
	csrf := ctx.Request().FormValue("csrf")
	// At the action func (example UpsertEvent), check that this token is present and valid in Redis
	if !app.VerifyFormToken(csrf) {
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		return err
	}
	// apparently embedded fields cannot be set immediately in a literal struct
	// we'll set those after efs is created
	efs := event.Presenter{
		EventDate:     strings.TrimSpace(ctx.Request().FormValue("event_date")),
		EventTime:     strings.TrimSpace(ctx.Request().FormValue("event_time")),
		Location:      strings.TrimSpace(ctx.Request().FormValue("event_location")),
		ContactPerson: strings.TrimSpace(ctx.Request().FormValue("contact_person")),
		ContactPhone:  strings.TrimSpace(ctx.Request().FormValue("contact_phone")),
		ContactEmail:  strings.TrimSpace(ctx.Request().FormValue("contact_email")),
		ContactURL:    strings.TrimSpace(ctx.Request().FormValue("contact_url")),
	}
	// set embedded fields etc
	efs.Id = ctx.Request().FormValue("event_id")
	efs.Title = strings.TrimSpace(ctx.Request().FormValue("event_title"))
	efs.Summary = ctx.Request().FormValue("event_summary")
	efs.Body = ctx.Request().FormValue("event_body")
	efs.Categories = stringops.StringSplitAndTrim(ctx.Request().FormValue("categories"), ",")
	
	// Get username from session
	sess, err := cctx.GetSessionFromRWeb(ctx)
	if err == nil && sess != nil {
		efs.UpdatedBy = sess.Username
	}
	
	if ctx.Request().FormValue("published") == "on" {
		efs.Published = true
	}

	err = efs.UpsertEvent()
	if err != nil {
		logger.LogErr(err, "Error in event upsert", "event_presenter", fmt.Sprintf("%#v", efs))
		return err
	}
	msg := "Created"
	if efs.Id != "0" && efs.Id != "" {
		msg = "Updated"
	}

	redirectTo := "/admin/events"
	if sess != nil && sess.FormReferrer != "" {
		redirectTo = sess.FormReferrer // return to the form caller
	}
	return app.RedirectRWeb(ctx, redirectTo, "Event "+msg)
}

func DeleteEventRWeb(ctx rweb.Context) error {
	err := event.DeleteEventById(ctx.Request().PathParam("id"))
	msg := "Event with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete event with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, "when", "deleting event")
	}
	return app.RedirectRWeb(ctx, "/admin/events", msg)
}