package user_controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/apitoken"
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
	// Check token valid against the in-process kvstore
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

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return err
	}
	err = efs.UpsertUser(dbH)
	if err != nil {
		logger.LogErr(err, "Error in user upsert", "user_presenter", fmt.Sprintf("%#v", efs))
		return err
	}
	msg := "Created"
	if efs.Id != "0" && efs.Id != "" {
		msg = "Updated"
		// Security sweep: a password change or account disable must also kill
		// every mobile API session — the old credential/permission must not
		// live on in phones for up to the 30-day token TTL. Unconditional on
		// disable (we don't load the prior enabled state; re-revoking an
		// already-disabled user's zero tokens is a harmless no-op). Lives here
		// rather than in resource/user because apitoken imports resource/user
		// — calling the other way would be an import cycle.
		if efs.Password != "" || !efs.Enabled {
			if userID, convErr := strconv.ParseInt(efs.Id, 10, 64); convErr == nil {
				if revErr := apitoken.RevokeAllForUser(dbH, userID); revErr != nil {
					// The upsert itself succeeded — log loudly but don't fail
					// the admin's save over the token sweep.
					logger.LogErr(revErr, "Error revoking user's api tokens after update", "user_id", efs.Id)
				}
			}
		}
	}
	return app.RedirectRWeb(ctx, "/admin/users", "User "+msg)
}

func DeleteUserRWeb(ctx rweb.Context) error {
	// POST + token: the route rejects GET, and the token ties the request to a
	// page we actually rendered (see grid CSRFToken / app.VerifyFormTokenRWeb).
	if ok, err := app.VerifyFormTokenRWeb(ctx, "/admin/users"); !ok {
		return err
	}
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return app.RedirectRWeb(ctx, "/admin/users", "Error deleting user")
	}
	err = user.DeleteUserById(dbH, ctx.Request().PathParam("id"))
	msg := "User with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete user with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, "when", "deleting user")
	}
	return app.RedirectRWeb(ctx, "/admin/users", msg)
}