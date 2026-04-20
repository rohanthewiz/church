package admin_controller

import (
	"database/sql"
	"errors"
	"time"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/admin"
	ctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/church/util/stringops"
	. "github.com/rohanthewiz/logger"
)

func AdminHandler(c echo.Context) error {
	c.String(200, "Hello Administrator!")
	return nil
}

func CreateTestEvents(c echo.Context) error {
	username := c.(*ctx.CustomContext).Session.Username
	dte, err := time.Parse("01/02/2006 -07", "06/15/2017 -05")
	if err != nil {
		return errors.New("Error parsing provided event values")
	}
	evt := &model.Event{
		Title:      "Picnic",
		Summary:    sql.NullString{String: "It's gonna be great!", Valid: true},
		Slug:       stringops.SlugWithRandomString("Picnic"),
		EventDate:  dte,
		EventTime:  "14:30pm",
		Categories: []string{"default"},
		UpdatedBy:  username,
		Published:  true,
	}
	if err := model.InsertEvent(evt); err != nil {
		c.Error(err)
		return err
	}

	dte, err = time.Parse("01/02/2006 -07", "06/12/2017 -05")
	if err != nil {
		return errors.New("Error parsing provided event values")
	}
	evt = &model.Event{
		Title:      "Retreat",
		Summary:    sql.NullString{String: "Get refreshed!", Valid: true},
		Slug:       stringops.SlugWithRandomString("Retreat"),
		EventDate:  dte,
		EventTime:  "10:00AM",
		Categories: []string{"default"},
		UpdatedBy:  username,
		Published:  true,
	}
	if err := model.InsertEvent(evt); err != nil {
		c.Error(err)
		return err
	}
	c.String(200, "Events created")
	return nil
}

// Create a superadministrator if no superadmins exist and you pass the right token
// This is useful for bootstrapping users
// Query params: token, username, password
func SetupSuperAdmin(c echo.Context) error {
	if admin.SuperToken == "" || c.QueryParam("token") != admin.SuperToken { return errors.New("ye shalt not pass") }

	Log("info", "Creating superadmin", "username", c.QueryParam("username"), "password", c.QueryParam("password"))
	return admin.CreateSuperUser(c.QueryParam("username"), c.QueryParam("password"))
}
