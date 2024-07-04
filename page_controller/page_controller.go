package page_controller

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	ctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/church/util/stringops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

func HomePage(c echo.Context) (err error) {
	app.Redirect(c, "/pages/home", "")
	// pg, err := page.Home()
	// if err != nil {
	//	c.Error(err)
	//	return err
	// }
	// buf := new(bytes.Buffer)
	// // Main module can receive an id param (probably should be an array of ids)
	// template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{pg.MainModuleSlug(): {"id": c.Param("id")}})
	// c.HTMLBlob(200, buf.Bytes())
	return err
}

// Non-Admin dynamic pages (the majority of the pages)
func PageHandler(c echo.Context) error {
	pg, err := page.PageFromSlug(strings.ToLower(c.Param("slug")))
	if err != nil {
		c.Error(err)
		return serr.Wrap(err)
	}
	c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return nil
}

// Admin Pages

func NewPage(c echo.Context) error {
	pg, err := page.PageForm()
	if err != nil {
		c.Error(err)
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{}, app.IsLoggedIn(c))
	c.HTMLBlob(200, buf.Bytes())
	return nil
}

func AdminShowPage(c echo.Context) error {
	pg, err := page.PageFromId(c.Param("id"))
	if err != nil {
		logger.LogErr(serr.Wrap(err))
		c.Error(err)
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{}, app.IsLoggedIn(c))
	c.HTMLBlob(200, buf.Bytes())
	return nil
}

func AdminListPages(c echo.Context) error {
	pg, err := page.PagesList()
	if err != nil {
		c.Error(err)
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{
		pg.MainModuleSlug(): {"offset": c.QueryParam("offset"), "limit": c.QueryParam("limit")}}, app.IsLoggedIn(c))
	c.HTMLBlob(200, buf.Bytes())
	return nil
}

func EditPage(c echo.Context) error {
	pg, err := page.PageForm()
	if err != nil {
		c.Error(err)
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{pg.MainModuleSlug(): {"id": c.Param("id")}},
		app.IsLoggedIn(c))
	c.HTMLBlob(200, buf.Bytes())
	return nil
}

func UpsertPage(c echo.Context) error {
	if !app.VerifyFormToken(c.FormValue("csrf")) { // Check that this token is present and valid in Redis
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		c.Error(err)
		return err
	}
	pg := page.Presenter{}
	pg.Id = strings.TrimSpace(c.FormValue("page_id"))
	pg.Title = strings.TrimSpace(c.FormValue("page_title"))
	pg.Slug = strings.TrimSpace(c.FormValue("page_slug"))
	pg.AvailablePositions = stringops.StringSplitAndTrim(c.FormValue("available_positions"), ",")
	if c.FormValue("published") == "on" {
		pg.Published = true
	}
	if c.FormValue("is_home") == "on" {
		pg.IsHome = true
	}
	pg.IsAdmin = false // admin pages shall be all hardwired
	// if c.FormValue("is_admin") == "on" {
	//	pg.IsAdmin = true
	// }

	// The entire form data is serialized into the "modules" field (behavior of the js serializer)
	// We are only interested in the Modules portions of that though
	formJson := strings.TrimSpace(c.FormValue("modules"))
	logger.Log("Debug", "Data from form", "json", formJson)
	if formJson == "" {
		err := errors.New("No modules received for page")
		c.Error(err)
		return serr.Wrap(err)
	}
	pg.Modules = page.ModulePresentersFromJson(formJson)
	pg.UpdatedBy = c.(*ctx.CustomContext).Session.Username
	logger.LogAsync("Debug", "Page Presenter from form", "page", fmt.Sprintf("%#v", pg))
	pgUrl, err := page.UpsertPage(pg)
	if err != nil {
		logger.LogErr(err, "Error in event upsert", "page presenter", fmt.Sprintf("%#v", pg))
		c.Error(err)
		return err
	}

	msg := "Created"
	if pg.Id != "0" && pg.Id != "" {
		msg = "Updated"
	}
	app.Redirect(c, "/admin/pages", "Page "+msg+" - Page URL -> "+pgUrl)
	return nil
}

func DeletePage(c echo.Context) error {
	err := page.DeletePageById(c.Param("id"))
	msg := "Page with id: " + c.Param("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete page with id: " + c.Param("id")
		logger.LogErr(err, "when", "deleting page")
	}
	app.Redirect(c, "/admin/pages", msg)
	return nil
}
