package user_controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

func NewUserRWeb(ctx rweb.Context) error {
	pg, err := page.UserForm()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageNewRWeb(pg, ctx)))
}

func ListUsersRWeb(ctx rweb.Context) error {
	pg, err := page.UsersList()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageListRWeb(pg, ctx)))
}

func EditUserRWeb(ctx rweb.Context) error {
	pg, err := page.UserForm()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageSingleRWeb(pg, ctx)))
}

func UpsertUserRWeb(ctx rweb.Context) error {
	csrf := ctx.Request().FormValue("csrf")
	// Check token valid against Redis
	if !app.VerifyFormToken(csrf) {
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		return err
	}
	efs := user.Presenter{}
	efs.Id = ctx.Request().FormValue("user_id")
	efs.Username = strings.TrimSpace(ctx.Request().FormValue("username"))
	efs.EmailAddress = strings.TrimSpace(ctx.Request().FormValue("email_address"))
	efs.Firstname = strings.TrimSpace(ctx.Request().FormValue("firstname"))
	efs.Lastname = strings.TrimSpace(ctx.Request().FormValue("lastname"))
	efs.Summary = ctx.Request().FormValue("user_summary")
	efs.Password = ctx.Request().FormValue("password")                     // do not trim space!
	efs.PasswordConfirmation = ctx.Request().FormValue("password_confirm") // do not trim space!
	
	// Get username from session
	sess, err := cctx.GetSessionFromRWeb(ctx)
	if err == nil && sess != nil {
		efs.UpdatedBy = sess.Username
	}
	
	role, err := strconv.ParseInt(ctx.Request().FormValue("role"), 10, 64)
	if err != nil {
		logger.LogErr(err, "Error converting role")
		return err
	}
	efs.Role = int(role)
	if ctx.Request().FormValue("enabled") == "on" {
		efs.Enabled = true
	}

	err = efs.UpsertUser()
	if err != nil {
		logger.LogErr(err, "Error in user upsert", "user_presenter", fmt.Sprintf("%#v", efs))
		return err
	}
	msg := "Created"
	if efs.Id != "0" && efs.Id != "" {
		msg = "Updated"
	}
	return app.RedirectRWeb(ctx, "/admin/users", "User "+msg)
}

func DeleteUserRWeb(ctx rweb.Context) error {
	err := user.DeleteUserById(ctx.Request().PathParam("id"))
	msg := "User with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete user with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, "when", "deleting user")
	}
	return app.RedirectRWeb(ctx, "/admin/users", msg)
}