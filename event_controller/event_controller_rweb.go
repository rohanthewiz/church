package event_controller

import (
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/authz"
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

// UpsertEventRWeb saves the admin event form.
//
// Failures come back to the admin as a flash, never a bare 500. Where they land
// depends on what is known about the DB state:
//
//	expired token / refused input ──► the form again   (nothing was written)
//	server fault                  ──► the events list  (a partial write is possible)
//
// Refused input is safe to send back to the form because event.Validate runs
// before the first write. A server fault may arrive after the events row was
// inserted (recurrence and location rows follow it, untransacted), so returning
// to the "new" form would invite a duplicate; the list shows what exists.
// The redirect re-renders the form from the DB, so typed-but-unsaved values are
// not carried back.
func UpsertEventRWeb(ctx rweb.Context) error {
	id := strings.TrimSpace(ctx.Request().FormValue("event_id"))
	formURL := "/admin/events/new"
	if id != "" && id != "0" {
		formURL = "/admin/events/edit/" + id
	}

	csrf := ctx.Request().FormValue("csrf")
	// At the action func (example UpsertEvent), check that this token is present and valid in the in-process kvstore
	if !app.VerifyFormToken(csrf) {
		return app.RedirectRWebWarn(ctx, formURL,
			"Your form has expired and was not saved. Please refresh the form and try again.")
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
		// Recurrence rule fields; parsed and validated in UpsertEvent
		RecurFreq:    strings.TrimSpace(ctx.Request().FormValue("recur_freq")),
		RecurWeekday: strings.TrimSpace(ctx.Request().FormValue("recur_weekday")),
		RecurWeek:    strings.TrimSpace(ctx.Request().FormValue("recur_week")),
		RecurUntil:   strings.TrimSpace(ctx.Request().FormValue("recur_until")),
		// The event's own point; parsed and validated in UpsertEvent, which is
		// also where "one of the two is filled in" is refused.
		Latitude:  strings.TrimSpace(ctx.Request().FormValue("event_latitude")),
		Longitude: strings.TrimSpace(ctx.Request().FormValue("event_longitude")),
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

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return app.RedirectRWebError(ctx, formURL, "The event could not be saved: the database is unavailable.")
	}

	// Publishing is its own permission, resolved field by field rather than by
	// refusing the save: someone who may edit but not publish can still save
	// their edits, and the flag keeps its stored value (false on create). See
	// authz.ResolveFlag.
	actor, ok := authz.ActorFrom(ctx)
	if !ok {
		// Admin routes always run behind AdminGuardRWeb, so no actor means a
		// wiring bug. Fail closed rather than publish unchecked.
		return app.RedirectRWebError(ctx, formURL, "Your permissions could not be confirmed. The event was not saved.")
	}
	submittedFlag := efs.Published
	efs.Published, err = authz.ResolveFlag(dbH, actor, authz.EventsPublish, authz.FlagEventPublished, efs.Id, submittedFlag)
	if err != nil {
		logger.LogErr(err, "Error resolving event published flag", "event_id", efs.Id)
		return app.RedirectRWebError(ctx, formURL, "Error saving the event. It was not saved.")
	}
	// The form renders the switch disabled (with the stored value in a hidden
	// field) for anyone lacking the permission, so a difference here means a
	// stale form or a hand-built post. Say what happened instead of silently
	// ignoring the box.
	flagNote := ""
	if efs.Published != submittedFlag {
		flagNote = " Publishing needs the events.publish permission, so the published setting was left as it was."
	}
	err = efs.UpsertEvent(dbH)
	if err != nil {
		if msg, isInput := event.UserMessage(err); isInput {
			// The admin's mistake, not ours: no error log, just the reason
			return app.RedirectRWebError(ctx, formURL, msg+". The event was not saved.")
		}
		logger.LogErr(err, "Error in event upsert", "event_presenter", fmt.Sprintf("%#v", efs))
		return app.RedirectRWebError(ctx, "/admin/events",
			"Error saving the event. It may be partly saved; check it below before trying again.")
	}
	msg := "Created"
	if efs.Id != "0" && efs.Id != "" {
		msg = "Updated"
	}

	redirectTo := "/admin/events"
	if sess != nil && sess.FormReferrer != "" {
		redirectTo = sess.FormReferrer // return to the form caller
	}
	return app.RedirectRWeb(ctx, redirectTo, "Event "+msg+flagNote)
}

func DeleteEventRWeb(ctx rweb.Context) error {
	// POST + token: the route rejects GET, and the token ties the request to a
	// page we actually rendered (see grid CSRFToken / app.VerifyFormTokenRWeb).
	if ok, err := app.VerifyFormTokenRWeb(ctx, "/admin/events"); !ok {
		return err
	}
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return app.RedirectRWeb(ctx, "/admin/events", "Error deleting event")
	}
	err = event.DeleteEventById(dbH, ctx.Request().PathParam("id"))
	msg := "Event with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete event with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, "when", "deleting event")
	}
	return app.RedirectRWeb(ctx, "/admin/events", msg)
}