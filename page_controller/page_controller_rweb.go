package page_controller

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/church/util/stringops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

func HomePageRWeb(ctx rweb.Context) error {
	return app.RedirectRWeb(ctx, "/pages/home", "")
	// pg, err := page.Home()
	// if err != nil {
	//	return err
	// }
	// buf := new(bytes.Buffer)
	// // Main module can receive an id param (probably should be an array of ids)
	// template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{pg.MainModuleSlug(): {"id": ctx.Request().PathParam("id")}})
	// return ctx.WriteHTML(buf.String())
}

// Non-Admin dynamic pages (the majority of the pages)
func PageHandlerRWeb(ctx rweb.Context) error {
	pg, err := page.PageFromSlug(strings.ToLower(ctx.Request().PathParam("slug")))
	if err != nil {
		return serr.Wrap(err)
	}
	return ctx.WriteHTML(string(base.RenderPageSingleRWeb(pg, ctx)))
}

// Admin Pages

func NewPageRWeb(ctx rweb.Context) error {
	pg, err := page.PageForm()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func AdminShowPageRWeb(ctx rweb.Context) error {
	pg, err := page.PageFromId(ctx.Request().PathParam("id"))
	if err != nil {
		logger.LogErr(serr.Wrap(err))
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func AdminListPagesRWeb(ctx rweb.Context) error {
	pg, err := page.PagesList()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		pg.MainModuleSlug(): {"offset": ctx.Request().QueryParam("offset"), "limit": ctx.Request().QueryParam("limit")}}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func EditPageRWeb(ctx rweb.Context) error {
	pg, err := page.PageForm()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{pg.MainModuleSlug(): {"id": ctx.Request().PathParam("id")}},
		app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func UpsertPageRWeb(ctx rweb.Context) error {
	if !app.VerifyFormToken(ctx.Request().FormValue("csrf")) { // Check that this token is present and valid in Redis
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		return err
	}
	pg := page.Presenter{}
	pg.Id = strings.TrimSpace(ctx.Request().FormValue("page_id"))
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
	if formJson == "" {
		err := errors.New("No modules received for page")
		return serr.Wrap(err)
	}
	pg.Modules = page.ModulePresentersFromJson(formJson)
	
	// Get username from session
	sess, err := cctx.GetSessionFromRWeb(ctx)
	if err == nil && sess != nil {
		pg.UpdatedBy = sess.Username
	}
	
	logger.LogAsync("Debug", "Page Presenter from form", "page", fmt.Sprintf("%#v", pg))
	pgUrl, err := page.UpsertPage(pg)
	if err != nil {
		logger.LogErr(err, "Error in event upsert", "page presenter", fmt.Sprintf("%#v", pg))
		return err
	}

	msg := "Created"
	if pg.Id != "0" && pg.Id != "" {
		msg = "Updated"
	}
	return app.RedirectRWeb(ctx, "/admin/pages", "Page "+msg+" - Page URL -> "+pgUrl)
}

func DeletePageRWeb(ctx rweb.Context) error {
	err := page.DeletePageById(ctx.Request().PathParam("id"))
	msg := "Page with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete page with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, "when", "deleting page")
	}
	return app.RedirectRWeb(ctx, "/admin/pages", msg)
}