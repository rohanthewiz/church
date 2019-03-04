package admin_controller

import (
	"errors"
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/admin"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/util/stringops"
	ctx "github.com/rohanthewiz/church/context"
	. "github.com/rohanthewiz/logger"
	"gopkg.in/nullbio/null.v6"
	"time"
)

func AdminHandler(c echo.Context) error {
	c.String(200, "Hello Administrator!")
	return nil
}

func CreateTestEvents(c echo.Context) error {
	d, err := db.Db()
	if err != nil {
		c.Error(err); return err
	}
	username := c.(*ctx.CustomContext).Session.Username
	dte, err := time.Parse("01/02/2006 -07", "06/15/2017 -05")
	if err != nil {
		return errors.New("Error parsing provided event values")
	}
	evt := &models.Event{
		Title:      "Picnic", Summary: null.NewString("It's gonna be great!", true),
		Slug:       stringops.SlugWithRandomString("Picnic"),
		EventDate:  dte,
		EventTime:  "14:30pm",
		Categories: []string{ "default" },
		UpdatedBy:  username,
		Published:  true,
	}
	if err := evt.Insert(d); err != nil {
		c.Error(err); return err
	}

	dte, err = time.Parse("01/02/2006 -07", "06/12/2017 -05")
	if err != nil {
		return errors.New("Error parsing provided event values")
	}
	evt = &models.Event{
		Title:      "Retreat", Summary: null.NewString("Get refreshed!", true),
		Slug:       stringops.SlugWithRandomString("Retreat"),
		EventDate:  dte,
		EventTime:  "10:00AM",
		Categories: []string{ "default" },
		UpdatedBy:  username,
		Published:  true,
	}
	if err := evt.Insert(d); err != nil {
		c.Error(err); return err
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
