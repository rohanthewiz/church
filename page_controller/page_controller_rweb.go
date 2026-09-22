package page_controller

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/church/util/stringops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

// HomePageRWeb serves the home page. It first attempts to load the page with
// slug "home" from the database. If that fails (e.g. no home page has been
// created yet), it falls back to a hardwired home page so the site remains
// functional even without DB-seeded content.
func HomePageRWeb(ctx rweb.Context) error {
	pg, err := loadPageBySlug("home")
	if err != nil {
		logger.Log("Info", "Home page not found in DB, using hardwired fallback", "err", err.Error())
		pg, err = page.Home()
		if err != nil {
			return serr.Wrap(err, "failed to load hardwired home page")
		}
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		pg.MainModuleSlug(): {"id": ctx.Request().PathParam("id")},
		// Only username: the nav resolves a signed-in viewer's permissions
		// from it to filter the Admin submenu (see menu.RenderNav)
		"_global": {"username": cctx.GetUsernameFromRWeb(ctx)},
	}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

// Non-Admin dynamic pages (the majority of the pages)
func PageHandlerRWeb(ctx rweb.Context) error {
	slug := strings.ToLower(ctx.Request().PathParam("slug"))
	pg, err := loadPageBySlug(slug)
	if err != nil {
		// No row for the slug is the visitor's problem (a stale link, a
		// typo), not a server fault: answer 404 with a page in the site
		// layout. Both backends reach the finder through database/sql, whose
		// Row.Scan reports a missing row as sql.ErrNoRows, and serr.Wrap
		// keeps it reachable via Unwrap. Any other error is a real failure
		// and stays a 500.
		if errors.Is(err, sql.ErrNoRows) {
			logger.Debug("Page not found", "slug", slug)
			ctx.Status(http.StatusNotFound)
			return ctx.WriteHTML(string(base.RenderPageSingleRWeb(page.NotFound(), ctx)))
		}
		return serr.Wrap(err)
	}
	return ctx.WriteHTML(string(base.RenderPageSingleRWeb(pg, ctx)))
}

// loadPageBySlug is the controller-side boundary where the DB handle is
// fetched and handed to the page query layer (see db/executor.go convention).
func loadPageBySlug(slug string) (*page.Page, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Could not obtain DB handle")
	}
	return page.PageFromSlug(dbH, slug)
}

// Admin Pages

func NewPageRWeb(ctx rweb.Context) error {
	pg, err := page.PageForm()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		"_global": {"user_agent": ctx.UserAgent(), "username": cctx.GetUsernameFromRWeb(ctx),
			// What the viewer may do, so admin modules offer only permitted actions
			authz.ParamKey: authz.ParamValue(ctx)},
	}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func AdminShowPageRWeb(ctx rweb.Context) error {
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return err
	}
	pg, err := page.PageFromId(dbH, ctx.Request().PathParam("id"))
	if err != nil {
		logger.LogErr(serr.Wrap(err))
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		"_global": {"user_agent": ctx.UserAgent(), "username": cctx.GetUsernameFromRWeb(ctx),
			// What the viewer may do, so admin modules offer only permitted actions
			authz.ParamKey: authz.ParamValue(ctx)},
	}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func AdminListPagesRWeb(ctx rweb.Context) error {
	pg, err := page.PagesList()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		pg.MainModuleSlug(): {"offset": ctx.Request().QueryParam("offset"), "limit": ctx.Request().QueryParam("limit")},
		"_global": {"user_agent": ctx.UserAgent(), "username": cctx.GetUsernameFromRWeb(ctx),
			// What the viewer may do, so admin modules offer only permitted actions
			authz.ParamKey: authz.ParamValue(ctx)},
	}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func EditPageRWeb(ctx rweb.Context) error {
	pg, err := page.PageForm()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		pg.MainModuleSlug(): {"id": ctx.Request().PathParam("id")},
		"_global": {"user_agent": ctx.UserAgent(), "username": cctx.GetUsernameFromRWeb(ctx),
			// What the viewer may do, so admin modules offer only permitted actions
			authz.ParamKey: authz.ParamValue(ctx)},
	}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

// UpsertPageRWeb saves the admin page form. Refusals before the write (expired
// token, missing modules JSON) flash back to the form; a failed write flashes
// on the pages list. Neither is a bare 500 page.
func UpsertPageRWeb(ctx rweb.Context) error {
	pg := page.Presenter{}
	pg.Id = strings.TrimSpace(ctx.Request().FormValue("page_id"))
	formURL := "/admin/pages/new"
	if pg.Id != "" && pg.Id != "0" {
		formURL = "/admin/pages/edit/" + pg.Id
	}

	if !app.VerifyFormToken(ctx.Request().FormValue("csrf")) { // Check that this token is present and valid in the in-process kvstore
		return app.RedirectRWebWarn(ctx, formURL,
			"Your form has expired and was not saved. Please refresh the form and try again.")
	}
	pg.Title = strings.TrimSpace(ctx.Request().FormValue("page_title"))
	pg.Slug = strings.TrimSpace(ctx.Request().FormValue("page_slug"))
	pg.AvailablePositions = stringops.StringSplitAndTrim(ctx.Request().FormValue("available_positions"), ",")
	if ctx.Request().FormValue("published") == "on" {
		pg.Published = true
	}
	if ctx.Request().FormValue("is_home") == "on" {
		pg.IsHome = true
	}
	pg.IsAdmin = false // admin pages shall be all hardwired
	// if ctx.Request().FormValue("is_admin") == "on" {
	//	pg.IsAdmin = true
	// }

	// The entire form data is serialized into the "modules" field (behavior of the js serializer)
	// We are only interested in the Modules portions of that though
	formJson := strings.TrimSpace(ctx.Request().FormValue("modules"))
	logger.Log("Debug", "Data from form", "json", formJson)
	// "modules" is filled by the form's preSubmit() script; empty means the
	// script did not run, not that the admin wants a page with no modules.
	if formJson == "" {
		logger.LogErr(serr.New("No modules received for page"), "page_id", pg.Id)
		return app.RedirectRWebError(ctx, formURL,
			"No page modules were received, so the page was not saved. Please reload the form and try again.")
	}
	pg.Modules = page.ModulePresentersFromJson(formJson)

	// Get username from session
	sess, err := cctx.GetSessionFromRWeb(ctx)
	if err == nil && sess != nil {
		pg.UpdatedBy = sess.Username
	}

	logger.LogAsync("Debug", "Page Presenter from form", "page", fmt.Sprintf("%#v", pg))
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return app.RedirectRWebError(ctx, formURL, "The page could not be saved: the database is unavailable.")
	}

	// Publishing is its own permission, resolved field by field rather than by
	// refusing the save: someone who may edit but not publish can still save
	// their edits, and the flag keeps its stored value (false on create). See
	// authz.ResolveFlag.
	actor, ok := authz.ActorFrom(ctx)
	if !ok {
		// Admin routes always run behind AdminGuardRWeb, so no actor means a
		// wiring bug. Fail closed rather than publish unchecked.
		return app.RedirectRWebError(ctx, formURL, "Your permissions could not be confirmed. The page was not saved.")
	}
	submittedFlag := pg.Published
	pg.Published, err = authz.ResolveFlag(dbH, actor, authz.PagesPublish, authz.FlagPagePublished, pg.Id, submittedFlag)
	if err != nil {
		logger.LogErr(err, "Error resolving page published flag", "page_id", pg.Id)
		return app.RedirectRWebError(ctx, formURL, "Error saving the page. It was not saved.")
	}
	// The form renders the switch disabled (with the stored value in a hidden
	// field) for anyone lacking the permission, so a difference here means a
	// stale form or a hand-built post. Say what happened instead of silently
	// ignoring the box.
	flagNote := ""
	if pg.Published != submittedFlag {
		flagNote = " Publishing needs the pages.publish permission, so the published setting was left as it was."
	}
	pgUrl, err := page.UpsertPage(dbH, pg)
	if err != nil {
		logger.LogErr(err, "Error in page upsert", "page presenter", fmt.Sprintf("%#v", pg))
		return app.RedirectRWebError(ctx, "/admin/pages", "Error saving the page. Please check it below and try again.")
	}

	msg := "Created"
	if pg.Id != "0" && pg.Id != "" {
		msg = "Updated"
	}
	return app.RedirectRWeb(ctx, "/admin/pages", "Page "+msg+" - Page URL -> "+pgUrl+flagNote)
}

func DeletePageRWeb(ctx rweb.Context) error {
	// POST + token: the route rejects GET, and the token ties the request to a
	// page we actually rendered (see grid CSRFToken / app.VerifyFormTokenRWeb).
	if ok, err := app.VerifyFormTokenRWeb(ctx, "/admin/pages"); !ok {
		return err
	}
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return app.RedirectRWeb(ctx, "/admin/pages", "Error deleting page")
	}
	err = page.DeletePageById(dbH, ctx.Request().PathParam("id"))
	msg := "Page with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete page with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, "when", "deleting page")
	}
	return app.RedirectRWeb(ctx, "/admin/pages", msg)
}
