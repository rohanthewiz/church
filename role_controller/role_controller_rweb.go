// Package role_controller serves Role Management (/admin/roles). Routes are
// permission-wrapped in router_rweb.go; the handlers here add the
// no-escalation rule, which a route permission can't express because it
// depends on the role being saved.
package role_controller

import (
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/util/inputerr"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

const rolesURL = "/admin/roles"

func ListRolesRWeb(ctx rweb.Context) error {
	pg, err := page.RolesList()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageListRWeb(pg, ctx)))
}

func NewRoleRWeb(ctx rweb.Context) error {
	pg, err := page.RoleForm()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageNewRWeb(pg, ctx)))
}

func EditRoleRWeb(ctx rweb.Context) error {
	pg, err := page.RoleForm()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageSingleRWeb(pg, ctx)))
}

// UpsertRoleRWeb saves the role form.
//
//	expired token / bad id / input error     ─► form, warn/error    nothing written
//	role holds perms the actor lacks         ─► form, error         nothing written
//	submitted perms the actor lacks          ─► form, error         nothing written
//	would leave no role manager (non-super)  ─► form, error         nothing written
//	DB fault mid-save                       ─► form, error         possibly partial:
//	                                                                fewer grants, never
//	                                                                more (see SaveRole)
func UpsertRoleRWeb(ctx rweb.Context) error {
	id := strings.TrimSpace(ctx.Request().FormValue("role_id"))
	formURL := rolesURL + "/new"
	if id != "" && id != "0" {
		formURL = rolesURL + "/edit/" + id
	}
	if !app.VerifyFormToken(ctx.Request().FormValue("csrf")) {
		return app.RedirectRWebWarn(ctx, formURL,
			"Your form has expired and was not saved. Please refresh the form and try again.")
	}
	actor, ok := authz.ActorFrom(ctx)
	if !ok { // unreachable behind Require; fail closed
		return app.RedirectRWebError(ctx, formURL, "Could not confirm your permissions. The role was not saved.")
	}

	var roleID int64
	if id != "" && id != "0" {
		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return app.RedirectRWebError(ctx, rolesURL, "That role could not be found. Nothing was saved.")
		}
		roleID = parsed
	}

	// Walk the catalog and ask for each permission's field by name. Anything
	// posted outside the catalog is never looked at, so a tampered form can't
	// smuggle in an unknown permission string.
	perms := authz.Set{}
	for _, res := range authz.Catalog() {
		for _, act := range res.Actions {
			p := res.Perm(act)
			if ctx.Request().FormValue(authz.PermFieldPrefix+string(p)) == "on" {
				perms[p] = struct{}{}
			}
		}
	}
	// Normalized before the grant check so an implied read is authorized too.
	perms = authz.Normalize(perms)

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return app.RedirectRWebError(ctx, formURL, "The role could not be saved: the database is unavailable.")
	}

	if roleID != 0 {
		existing, found, err := authz.GetRole(dbH, roleID)
		if err != nil {
			logger.LogErr(err, "Error loading role for update", "role_id", id)
			return app.RedirectRWebError(ctx, formURL, "Error saving the role. It was not saved.")
		}
		if !found {
			return app.RedirectRWebError(ctx, rolesURL, "That role no longer exists. Nothing was saved.")
		}
		// Changing a role that holds permissions beyond the actor would let
		// them revoke grants they never held.
		if !actor.CanGrant(existing.Perms) {
			return app.RedirectRWebError(ctx, formURL,
				"This role holds permissions you don't have, so you can't change it. The role was not saved.")
		}
	}
	if !actor.CanGrant(perms) {
		return app.RedirectRWebError(ctx, formURL,
			"You can only grant permissions you hold yourself. The role was not saved.")
	}
	// Taking roles.update out of the only role that grants it to anyone would
	// leave only a SuperAdmin able to manage roles (see authz/lockout.go). A
	// SuperAdmin is exempt, and a new role only adds.
	if roleID != 0 && !actor.IsSuper() {
		locks, err := authz.LocksOutRoleManagers(dbH, authz.PendingChange{RoleID: roleID, RolePerms: perms})
		if err != nil {
			logger.LogErr(err, "Error checking role-manager lockout", "role_id", id)
			return app.RedirectRWebError(ctx, formURL, "Error saving the role. It was not saved.")
		}
		if locks {
			return app.RedirectRWebError(ctx, formURL, authz.RoleManagerLockoutMsg+" The role was not saved.")
		}
	}

	_, err = authz.SaveRole(dbH, authz.Role{
		ID:          roleID,
		Name:        ctx.Request().FormValue("role_name"),
		Description: ctx.Request().FormValue("role_description"),
		Perms:       perms,
	}, actor.Username)
	if err != nil {
		if msg, isInput := inputerr.UserMessage(err); isInput {
			return app.RedirectRWebError(ctx, formURL, msg+". The role was not saved.")
		}
		logger.LogErr(err, "Error saving role", "role_id", id)
		return app.RedirectRWebError(ctx, formURL, "Error saving the role. Please check it and save again.")
	}

	msg := "Role created"
	if roleID != 0 {
		msg = "Role updated"
	}
	return app.RedirectRWeb(ctx, rolesURL, msg)
}

// DeleteRoleRWeb deletes a role and its assignments. The people who held it
// keep their other roles.
func DeleteRoleRWeb(ctx rweb.Context) error {
	if ok, err := app.VerifyFormTokenRWeb(ctx, rolesURL); !ok {
		return err
	}
	actor, ok := authz.ActorFrom(ctx)
	if !ok {
		return app.RedirectRWebError(ctx, rolesURL, "Could not confirm your permissions. Nothing was deleted.")
	}
	roleID, err := strconv.ParseInt(ctx.Request().PathParam("id"), 10, 64)
	if err != nil {
		return app.RedirectRWebError(ctx, rolesURL, "That role could not be found. Nothing was deleted.")
	}
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return app.RedirectRWebError(ctx, rolesURL, "The role could not be deleted: the database is unavailable.")
	}
	role, found, err := authz.GetRole(dbH, roleID)
	if err != nil {
		logger.LogErr(err, "Error loading role for delete", "role_id", ctx.Request().PathParam("id"))
		return app.RedirectRWebError(ctx, rolesURL, "Error deleting the role.")
	}
	if !found {
		return app.RedirectRWeb(ctx, rolesURL, "That role was already deleted.")
	}
	// Deleting a role revokes its grants from everyone who holds it, so it
	// falls under the same rule as editing one.
	if !actor.CanGrant(role.Perms) {
		return app.RedirectRWebError(ctx, rolesURL,
			"That role holds permissions you don't have, so you can't delete it.")
	}
	if !actor.IsSuper() {
		locks, err := authz.LocksOutRoleManagers(dbH, authz.PendingChange{RoleID: roleID, DeleteRole: true})
		if err != nil {
			logger.LogErr(err, "Error checking role-manager lockout", "role_id", ctx.Request().PathParam("id"))
			return app.RedirectRWebError(ctx, rolesURL, "Error deleting the role.")
		}
		if locks {
			return app.RedirectRWebError(ctx, rolesURL, authz.RoleManagerLockoutMsg+" Nothing was deleted.")
		}
	}
	if err := authz.DeleteRole(dbH, roleID); err != nil {
		logger.LogErr(err, "Error deleting role", "role_id", ctx.Request().PathParam("id"))
		return app.RedirectRWebError(ctx, rolesURL, "Error deleting the role. Please try again.")
	}
	return app.RedirectRWeb(ctx, rolesURL, "Role deleted")
}
