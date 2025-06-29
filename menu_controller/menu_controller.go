package menu_controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	ctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/menu"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// Admin Pages

func NewMenu(c echo.Context) error {
	pg, err := page.MenuForm()
	if err != nil {
		c.Error(err)
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{}, app.IsLoggedIn(c))
	c.HTMLBlob(200, buf.Bytes())
	return nil
}

// func AdminShowMenu(c echo.Context) error {
//	pg, err := menu.MenuFromId(c.Param("id"))
//	if err != nil {
//		logger.LogErr(err, "Error in AdminShowMenu", "location", logger.FunctionLoc())
//		c.Error(err)
//		return err
//	}
//	buf := new(bytes.Buffer)
//	template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{})
//	c.HTMLBlob(200, buf.Bytes())
//	return  nil
// }

func AdminListMenus(c echo.Context) error {
	pg, err := page.MenusList()
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

func EditMenu(c echo.Context) error {
	pg, err := page.MenuForm()
	fmt.Println("*|* (In menu_controller) MenuForm - mainModuleSlug:", pg.MainModuleSlug())
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

func UpsertMenu(c echo.Context) error {
	if !app.VerifyFormToken(c.FormValue("csrf")) { // Check that this token is present and valid in Redis
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		c.Error(err)
		return err
	}
	mnu := menu.MenuDef{}
	mnu.Id = strings.TrimSpace(c.FormValue("menu_id"))
	mnu.Title = strings.TrimSpace(c.FormValue("menu_title"))
	// slugs are updated on the backend only //mnu.Slug = strings.TrimSpace(c.FormValue("menu_slug"))
	if c.FormValue("published") == "on" {
		mnu.Published = true
	}
	if c.FormValue("is_admin") == "on" {
		mnu.IsAdmin = true
	}

	// The entire form data is serialized into the "modules" field (behavior of the js serializer)
	// We are only interested in the Items portions of that though
	formJson := strings.TrimSpace(c.FormValue("items"))
	logger.Debug("Form data", "json", formJson)
	if formJson == "" {
		err := errors.New("No items received for menu")
		c.Error(err)
		return serr.Wrap(err)
	}
	form := menu.FormMenuObject{}
	err := json.Unmarshal([]byte(formJson), &form)
	for _, item := range form.Items {
		menuItemDef := menu.MenuItemDef{
			Label:       strings.TrimSpace(item.Label),
			Url:         strings.TrimSpace(item.Url),
			SubMenuSlug: item.SubMenuSlug,
		}
		mnu.Items = append(mnu.Items, menuItemDef)
	}

	mnu.UpdatedBy = c.(*ctx.CustomContext).Session.Username
	fmt.Printf("*|* menu: %#v\n", mnu)

	err = menu.UpsertMenu(mnu)
	if err != nil {
		logger.LogErr(serr.Wrap(err, "Error in event upsert"))
		c.Error(err)
		return serr.Wrap(err)
	}
	msg := "Created"
	if mnu.Id != "0" && mnu.Id != "" {
		msg = "Updated"
	}
	app.Redirect(c, "/admin/menus", "Menu "+msg)
	return nil
}

func DeleteMenu(c echo.Context) error {
	err := menu.DeleteMenuById(c.Param("id"))
	msg := "Menu with id: " + c.Param("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete menu with id: " + c.Param("id")
		logger.LogErr(err, "when", "deleting menu")
	}
	app.Redirect(c, "/admin/menus", msg)
	return nil
}
