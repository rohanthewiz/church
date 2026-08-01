package admin_controller

import (
	"bytes"
	"errors"
	"time"

	"github.com/rohanthewiz/church/admin"
	"github.com/rohanthewiz/church/app"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/church/util/stringops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"gopkg.in/nullbio/null.v6"
)

// AdminHandlerRWeb renders the admin dashboard (was a plain-text greeting).
func AdminHandlerRWeb(ctx rweb.Context) error {
	pg, err := page.AdminHome()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		"_global": {"user_agent": ctx.UserAgent()},
	}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func CreateTestEventsRWeb(ctx rweb.Context) error {
	d, err := db.Db()
	if err != nil {
		return err
	}
	
	// Get username from session
	username := ""
	sess, err := cctx.GetSessionFromRWeb(ctx)
	if err == nil && sess != nil {
		username = sess.Username
	}
	
	dte, err := time.Parse("01/02/2006 -07", "06/15/2017 -05")
	if err != nil {
		return errors.New("Error parsing provided event values")
	}
	evt := &models.Event{
		Title:      "Picnic", Summary: null.NewString("It's gonna be great!", true),
		Slug:       stringops.SlugWithRandomString("Picnic"),
		EventDate:  dte,
		EventTime:  "14:30pm",
		Categories: []string{"default"},
		UpdatedBy:  username,
		Published:  true,
	}
	if err := evt.Insert(d); err != nil {
		return err
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
		Categories: []string{"default"},
		UpdatedBy:  username,
		Published:  true,
	}
	if err := evt.Insert(d); err != nil {
		return err
	}
	return ctx.WriteString("Events created")
}

// Create a superadministrator if no superadmins exist and you pass the right token
// This is useful for bootstrapping users
// Query params: token, username, password
func SetupSuperAdminRWeb(ctx rweb.Context) error {
	if admin.SuperToken == "" || ctx.Request().QueryParam("token") != admin.SuperToken {
		return errors.New("ye shalt not pass")
	}

	logger.Log("info", "Creating superadmin", "username", ctx.Request().QueryParam("username"), "password", ctx.Request().QueryParam("password"))
	return admin.CreateSuperUser(ctx.Request().QueryParam("username"), ctx.Request().QueryParam("password"))
}