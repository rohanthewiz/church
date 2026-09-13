package menu_controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/authz"
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
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		"_global": {"user_agent": ctx.UserAgent(), "username": cctx.GetUsernameFromRWeb(ctx),
			// What the viewer may do, so admin modules offer only permitted actions
			authz.ParamKey: authz.ParamValue(ctx)},
	}, app.IsLoggedInRWeb(ctx))
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
		pg.MainModuleSlug(): {"offset": ctx.Request().QueryParam("offset"), "limit": ctx.Request().QueryParam("limit")},
		"_global": {"user_agent": ctx.UserAgent(), "username": cctx.GetUsernameFromRWeb(ctx),
			// What the viewer may do, so admin modules offer only permitted actions
			authz.ParamKey: authz.ParamValue(ctx)},
	}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func EditMenuRWeb(ctx rweb.Context) error {
	pg, err := page.MenuForm()
	fmt.Println("*|* (In menu_controller) MenuForm - mainModuleSlug:", pg.MainModuleSlug())
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

// UpsertMenuRWeb saves the admin menu form. Refusals before the write (expired
// token, missing or malformed items JSON) flash back to the form; a failed
// write flashes on the menus list. Neither is a bare 500 page.
func UpsertMenuRWeb(ctx rweb.Context) error {
	mnu := menu.MenuDef{}
	mnu.Id = strings.TrimSpace(ctx.Request().FormValue("menu_id"))
	formURL := "/admin/menus/new"
	if mnu.Id != "" && mnu.Id != "0" {
		formURL = "/admin/menus/edit/" + mnu.Id
	}

	if !app.VerifyFormToken(ctx.Request().FormValue("csrf")) { // Check that this token is present and valid in the in-process kvstore
		return app.RedirectRWebWarn(ctx, formURL,
			"Your form has expired and was not saved. Please refresh the form and try again.")
	}
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
	// "items" is filled by the form's preSubmit() script, so an empty value
	// means the script did not run (a JS error, or a post not made from the
	// form), not that the admin wants an empty menu.
	if formJson == "" {
		logger.LogErr(serr.New("No items received for menu"), "menu_id", mnu.Id)
		return app.RedirectRWebError(ctx, formURL,
			"No menu items were received, so the menu was not saved. Please reload the form and try again.")
	}
	form := menu.FormMenuObject{}
	err := json.Unmarshal([]byte(formJson), &form)
	if err != nil {
		logger.LogErr(serr.Wrap(err, "error unmarshaling menu items"), "menu_id", mnu.Id)
		return app.RedirectRWebError(ctx, formURL,
			"The menu items could not be read, so the menu was not saved. Please reload the form and try again.")
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

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return app.RedirectRWebError(ctx, formURL, "The menu could not be saved: the database is unavailable.")
	}

	// Enabling (the menu's published flag) is its own permission, resolved field by field rather than by
	// refusing the save: someone who may edit but not publish can still save
	// their edits, and the flag keeps its stored value (false on create). See
	// authz.ResolveFlag.
	actor, ok := authz.ActorFrom(ctx)
	if !ok {
		// Admin routes always run behind AdminGuardRWeb, so no actor means a
		// wiring bug. Fail closed rather than publish unchecked.
		return app.RedirectRWebError(ctx, formURL, "Your permissions could not be confirmed. The menu was not saved.")
	}
	submittedFlag := mnu.Published
	mnu.Published, err = authz.ResolveFlag(dbH, actor, authz.MenusEnable, authz.FlagMenuPublished, mnu.Id, submittedFlag)
	if err != nil {
		logger.LogErr(err, "Error resolving menu published flag", "menu_id", mnu.Id)
		return app.RedirectRWebError(ctx, formURL, "Error saving the menu. It was not saved.")
	}
	// The form renders the switch disabled (with the stored value in a hidden
	// field) for anyone lacking the permission, so a difference here means a
	// stale form or a hand-built post. Say what happened instead of silently
	// ignoring the box.
	flagNote := ""
	if mnu.Published != submittedFlag {
		flagNote = " Enabling needs the menus.enable permission, so the published setting was left as it was."
	}
	err = menu.UpsertMenu(dbH, mnu)
	if err != nil {
		logger.LogErr(serr.Wrap(err, "Error in menu upsert"))
		return app.RedirectRWebError(ctx, "/admin/menus", "Error saving the menu. Please check it below and try again.")
	}
	msg := "Created"
	if mnu.Id != "0" && mnu.Id != "" {
		msg = "Updated"
	}
	return app.RedirectRWeb(ctx, "/admin/menus", "Menu "+msg+flagNote)
}

func DeleteMenuRWeb(ctx rweb.Context) error {
	// POST + token: the route rejects GET, and the token ties the request to a
	// page we actually rendered (see grid CSRFToken / app.VerifyFormTokenRWeb).
	if ok, err := app.VerifyFormTokenRWeb(ctx, "/admin/menus"); !ok {
		return err
	}
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return app.RedirectRWeb(ctx, "/admin/menus", "Error deleting menu")
	}
	err = menu.DeleteMenuById(dbH, ctx.Request().PathParam("id"))
	msg := "Menu with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete menu with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, "when", "deleting menu")
	}
	return app.RedirectRWeb(ctx, "/admin/menus", msg)
}
