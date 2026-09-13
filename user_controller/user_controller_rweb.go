package user_controller

import (
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
	"github.com/rohanthewiz/church/util/inputerr"
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
	// An expired token is routine (a form left open too long), so send the admin
	// back to the same form with a warning rather than a bare 500. Nothing has
	// been written yet. The form re-renders from the DB, so unsaved edits
	// (including a typed password) are lost.
	id := strings.TrimSpace(ctx.Request().FormValue("user_id"))
	formURL := "/admin/users/new"
	if id != "" && id != "0" {
		formURL = "/admin/users/edit/" + id
	}
	csrf := ctx.Request().FormValue("csrf")
	// Check token valid against the in-process kvstore
	if !app.VerifyFormToken(csrf) {
		return app.RedirectRWebWarn(ctx, formURL,
			"Your form has expired and was not saved. Please refresh the form and try again.")
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
	
	// Failures go back to the form as an error flash. UpsertUser is a single
	// Insert or Update, so a failed save wrote nothing and the form (not the
	// list) is the right place to retry from.
	role, err := strconv.ParseInt(ctx.Request().FormValue("role"), 10, 64)
	if err != nil {
		// The role is a <select>, so this is a tampered or broken form rather than
		// a typo; still logged, but the admin gets the form back, not a 500.
		logger.LogErr(err, "Error converting role")
		return app.RedirectRWebError(ctx, formURL, "Please choose a role. The user was not saved.")
	}
	efs.Role = int(role)
	if ctx.Request().FormValue("enabled") == "on" {
		efs.Enabled = true
	}

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return app.RedirectRWebError(ctx, formURL, "The user could not be saved: the database is unavailable.")
	}
	err = efs.UpsertUser(dbH)
	if err != nil {
		if msg, isInput := inputerr.UserMessage(err); isInput {
			// The admin's mistake, not ours: no error log, just the reason
			return app.RedirectRWebError(ctx, formURL, msg+". The user was not saved.")
		}
		// Password fields are blanked before logging: %#v of the presenter would
		// otherwise put the typed password in the log.
		logged := efs
		logged.Password, logged.PasswordConfirmation = "", ""
		logger.LogErr(err, "Error in user upsert", "user_presenter", fmt.Sprintf("%#v", logged))
		return app.RedirectRWebError(ctx, formURL, "Error saving the user. It was not saved.")
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