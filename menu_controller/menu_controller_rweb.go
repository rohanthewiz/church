package menu_controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/menu"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

// Admin Pages

func NewMenuRWeb(ctx rweb.Context) error {
	pg, err := page.MenuForm()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

// func AdminShowMenuRWeb(ctx rweb.Context) error {
//	pg, err := menu.MenuFromId(ctx.Request().PathParam("id"))
//	if err != nil {
//		logger.LogErr(err, "Error in AdminShowMenu", "location", logger.FunctionLoc())
//		return err
//	}
//	buf := new(bytes.Buffer)
//	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{})
//	return ctx.WriteHTML(buf.String())
// }

func AdminListMenusRWeb(ctx rweb.Context) error {
	pg, err := page.MenusList()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		pg.MainModuleSlug(): {"offset": ctx.Request().QueryParam("offset"), "limit": ctx.Request().QueryParam("limit")}}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func EditMenuRWeb(ctx rweb.Context) error {
	pg, err := page.MenuForm()
	fmt.Println("*|* (In menu_controller) MenuForm - mainModuleSlug:", pg.MainModuleSlug())
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{pg.MainModuleSlug(): {"id": ctx.Request().PathParam("id")}},
		app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func UpsertMenuRWeb(ctx rweb.Context) error {
	if !app.VerifyFormToken(ctx.Request().FormValue("csrf")) { // Check that this token is present and valid in Redis
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		return err
	}
	mnu := menu.MenuDef{}
	mnu.Id = strings.TrimSpace(ctx.Request().FormValue("menu_id"))
	mnu.Title = strings.TrimSpace(ctx.Request().FormValue("menu_title"))
	// slugs are updated on the backend only //mnu.Slug = strings.TrimSpace(ctx.Request().FormValue("menu_slug"))
	if ctx.Request().FormValue("published") == "on" {
		mnu.Published = true
	}
	if ctx.Request().FormValue("is_admin") == "on" {
		mnu.IsAdmin = true
	}

	// The entire form data is serialized into the "items" field (behavior of the js serializer)
	// We are only interested in the Items portions of that though
	formJson := strings.TrimSpace(ctx.Request().FormValue("items"))
	logger.Debug("Form data", "json", formJson)
	if formJson == "" {
		err := errors.New("No items received for menu")
		return serr.Wrap(err)
	}
	form := menu.FormMenuObject{}
	err := json.Unmarshal([]byte(formJson), &form)
	if err != nil {
		return serr.Wrap(err, "error unmarshaling menu items")
	}
	for _, item := range form.Items {
		menuItemDef := menu.MenuItemDef{
			Label:       strings.TrimSpace(item.Label),
			Url:         strings.TrimSpace(item.Url),
			SubMenuSlug: item.SubMenuSlug,
		}
		mnu.Items = append(mnu.Items, menuItemDef)
	}

	// Get username from session
	sess, err := cctx.GetSessionFromRWeb(ctx)
	if err == nil && sess != nil {
		mnu.UpdatedBy = sess.Username
	}
	
	fmt.Printf("*|* menu: %#v\n", mnu)

	err = menu.UpsertMenu(mnu)
	if err != nil {
		logger.LogErr(serr.Wrap(err, "Error in event upsert"))
		return serr.Wrap(err)
	}
	msg := "Created"
	if mnu.Id != "0" && mnu.Id != "" {
		msg = "Updated"
	}
	return app.RedirectRWeb(ctx, "/admin/menus", "Menu "+msg)
}

func DeleteMenuRWeb(ctx rweb.Context) error {
	err := menu.DeleteMenuById(ctx.Request().PathParam("id"))
	msg := "Menu with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete menu with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, "when", "deleting menu")
	}
	return app.RedirectRWeb(ctx, "/admin/menus", msg)
}