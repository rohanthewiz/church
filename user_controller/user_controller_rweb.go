package user_controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/core/formdraft"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/apitoken"
	"github.com/rohanthewiz/church/resource/authz"
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
	id := strings.TrimSpace(ctx.Request().FormValue("user_id"))
	formURL := "/admin/users/new"
	if id != "" && id != "0" {
		formURL = "/admin/users/edit/" + id
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

	// A refused save returns to the form with what was typed (core/formdraft),
	// except the password, which is never kept: the admin types it again.
	// Role ticks are read against the role list; if that can't be loaded the
	// form falls back to the stored assignments.
	saveDraft := func() {
		d := user.FormDraft{Presenter: efs}
		d.Password, d.PasswordConfirmation = "", ""
		d.Enabled = ctx.Request().FormValue("enabled") == "on"
		if r, err := strconv.Atoi(ctx.Request().FormValue("role")); err == nil {
			d.Role = r
		}
		if dbH, err := db.Db(); err == nil {
			if rs, err := authz.ListRoles(dbH); err == nil {
				d.RolesPosted = true
				for _, r := range rs {
					if ctx.Request().FormValue(authz.RoleFieldPrefix+strconv.FormatInt(r.ID, 10)) == "on" {
						d.RoleIDs = append(d.RoleIDs, r.ID)
					}
				}
			}
		}
		formdraft.Save(ctx, formURL, d)
	}
	refuse := func(msg string) error {
		saveDraft()
		return app.RedirectRWebError(ctx, formURL, msg)
	}

	// An expired token is routine (a form left open too long), so send the admin
	// back to the same form with a warning rather than a bare 500. Nothing has
	// been written yet. Checked after reading the form, which has no side
	// effects, so the draft keeps their edits.
	csrf := ctx.Request().FormValue("csrf")
	// Check token valid against the in-process kvstore
	if !app.VerifyFormToken(csrf) {
		saveDraft()
		return app.RedirectRWebWarn(ctx, formURL,
			"Your form has expired and was not saved. Your changes are still in the form except the password; please save again.")
	}
	
	// Failures go back to the form as an error flash. UpsertUser is a single
	// Insert or Update, so a failed save wrote nothing and the form (not the
	// list) is the right place to retry from.
	role, err := strconv.ParseInt(ctx.Request().FormValue("role"), 10, 64)
	if err != nil {
		// The role is a <select>, so this is a tampered or broken form rather than
		// a typo; still logged, but the admin gets the form back, not a 500.
		logger.LogErr(err, "Error converting role")
		return refuse("Please choose a role. The user was not saved.")
	}
	efs.Role = int(role)
	submittedEnabled := ctx.Request().FormValue("enabled") == "on"

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return refuse("The user could not be saved: the database is unavailable.")
	}

	// ---- Authorization beyond the route permission ----
	// Every refusal here happens before any write.
	actor, ok := authz.ActorFrom(ctx)
	if !ok { // unreachable behind Require; fail closed
		return refuse("Could not confirm your permissions. The user was not saved.")
	}
	isUpdate := efs.Id != "" && efs.Id != "0"
	var userID int64
	if isUpdate {
		if userID, err = strconv.ParseInt(efs.Id, 10, 64); err != nil {
			return app.RedirectRWebError(ctx, "/admin/users", "That user could not be found. Nothing was saved.")
		}
		// No editing someone more powerful: a password reset on their
		// account would hand over everything they hold.
		canManage, err := authz.CanManageUser(dbH, actor, userID)
		if err != nil {
			logger.LogErr(err, "Error checking user management permission", "user_id", efs.Id)
			return refuse("Error saving the user. It was not saved.")
		}
		if !canManage {
			return refuse("This person can do things you can't, so you can't change their account. The user was not saved.")
		}
	}
	if efs.Role == authz.SuperAdminRole && !actor.IsSuper() {
		return refuse("Only a SuperAdmin can make someone a SuperAdmin. The user was not saved.")
	}
	enabled, err := authz.ResolveFlag(dbH, actor, authz.UsersEnable, authz.FlagUserEnabled, efs.Id, submittedEnabled)
	if err != nil {
		logger.LogErr(err, "Error resolving user enabled flag", "user_id", efs.Id)
		return refuse("Error saving the user. It was not saved.")
	}
	efs.Enabled = enabled

	// Current assignments are read before the save so the role step below
	// can tell "kept because locked" from "newly ticked".
	currentRoles := map[int64]bool{}
	if isUpdate {
		ids, err := authz.RoleIDsForUser(dbH, userID)
		if err != nil {
			logger.LogErr(err, "Error loading user roles", "user_id", efs.Id)
			return refuse("Error saving the user. It was not saved.")
		}
		for _, id := range ids {
			currentRoles[id] = true
		}
	}
	allRoles, err := authz.ListRoles(dbH)
	if err != nil {
		logger.LogErr(err, "Error listing roles for user save")
		return refuse("Error saving the user. It was not saved.")
	}

	// ---- Role assignment (decided here, written after the account) ----
	// Roles the actor can grant (a subset of their own permissions) follow
	// the form. Roles they can't grant keep their current state whatever was
	// posted: their checkboxes were disabled, and a disabled box posts
	// nothing, which must not read as "remove". Computed before any write so
	// the lockout check below sees the final assignment.
	var finalRoles []int64
	for _, r := range allRoles {
		field := authz.RoleFieldPrefix + strconv.FormatInt(r.ID, 10)
		if actor.CanGrant(r.Perms) {
			if ctx.Request().FormValue(field) == "on" {
				finalRoles = append(finalRoles, r.ID)
			}
		} else if currentRoles[r.ID] {
			finalRoles = append(finalRoles, r.ID)
		}
	}

	// Unticking the last role manager's roles, or disabling that account,
	// would leave only a SuperAdmin able to fix roles (see authz/lockout.go).
	// A SuperAdmin is exempt: they are the recovery path. A new account only
	// adds, so only updates are checked.
	if isUpdate && !actor.IsSuper() {
		locks, err := authz.LocksOutRoleManagers(dbH, authz.PendingChange{
			UserID: userID, SetUserRoles: true, UserRoles: finalRoles, RemoveUser: !efs.Enabled,
		})
		if err != nil {
			logger.LogErr(err, "Error checking role-manager lockout", "user_id", efs.Id)
			return refuse("Error saving the user. It was not saved.")
		}
		if locks {
			return refuse(authz.RoleManagerLockoutMsg+" The user was not saved.")
		}
	}

	savedID, err := efs.UpsertUserID(dbH)
	if err != nil {
		if msg, isInput := inputerr.UserMessage(err); isInput {
			// The admin's mistake, not ours: no error log, just the reason
			return refuse(msg+". The user was not saved.")
		}
		// Password fields are blanked before logging: %#v of the presenter would
		// otherwise put the typed password in the log.
		logged := efs
		logged.Password, logged.PasswordConfirmation = "", ""
		logger.LogErr(err, "Error in user upsert", "user_presenter", fmt.Sprintf("%#v", logged))
		return refuse("Error saving the user. It was not saved.")
	}
	// ---- Role assignment (finalRoles was decided before the save) ----
	if err := authz.SetUserRoles(dbH, savedID, finalRoles); err != nil {
		// The account itself saved; say exactly what didn't.
		logger.LogErr(err, "Error saving user roles", "user_id", strconv.FormatInt(savedID, 10))
		return app.RedirectRWebError(ctx, "/admin/users/edit/"+strconv.FormatInt(savedID, 10),
			"The user was saved, but their roles could not be updated. Please check them and save again.")
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
	// Same rule as editing: no deleting someone more powerful than yourself.
	actor, ok := authz.ActorFrom(ctx)
	if !ok {
		return app.RedirectRWebError(ctx, "/admin/users", "Could not confirm your permissions. Nothing was deleted.")
	}
	if userID, convErr := strconv.ParseInt(ctx.Request().PathParam("id"), 10, 64); convErr == nil {
		canManage, err := authz.CanManageUser(dbH, actor, userID)
		if err != nil {
			logger.LogErr(err, "Error checking user management permission", "user_id", ctx.Request().PathParam("id"))
			return app.RedirectRWebError(ctx, "/admin/users", "Error deleting user")
		}
		if !canManage {
			return app.RedirectRWebError(ctx, "/admin/users",
				"That person can do things you can't, so you can't delete their account.")
		}
		// Deleting the last role manager's account has the same effect as
		// removing their roles (see authz/lockout.go). A SuperAdmin is exempt.
		if !actor.IsSuper() {
			locks, err := authz.LocksOutRoleManagers(dbH, authz.PendingChange{UserID: userID, RemoveUser: true})
			if err != nil {
				logger.LogErr(err, "Error checking role-manager lockout", "user_id", ctx.Request().PathParam("id"))
				return app.RedirectRWebError(ctx, "/admin/users", "Error deleting user")
			}
			if locks {
				return app.RedirectRWebError(ctx, "/admin/users", authz.RoleManagerLockoutMsg+" Nothing was deleted.")
			}
		}
	}
	err = user.DeleteUserById(dbH, ctx.Request().PathParam("id"))
	msg := "User with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete user with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, "when", "deleting user")
	}
	return app.RedirectRWeb(ctx, "/admin/users", msg)
}