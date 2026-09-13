package authz

import (
	"strings"

	"github.com/rohanthewiz/rweb"
)

// SuperAdminRole is the legacy users.role value that bypasses permission
// checks. It mirrors user.Roles.SuperAdmin; it is repeated here rather than
// imported because resource/user imports this package (the user form lists
// roles), and importing back would be a cycle.
const SuperAdminRole = 99

// Actor is the signed-in admin a request acts as: who they are and what they
// may do. It is resolved from the database on every admin request (see
// LoadActor) instead of being cached in the session, so revoking a role or
// disabling an account takes effect on the user's next click rather than
// when their session happens to expire.
type Actor struct {
	UserID   int64
	Username string
	BaseRole int // legacy users.role; only SuperAdminRole matters here
	perms    Set
}

// IsSuper reports the SuperAdmin bypass.
func (a *Actor) IsSuper() bool {
	return a != nil && a.BaseRole == SuperAdminRole
}

// Can reports whether the actor holds p. A nil actor (anonymous, or a render
// path that never resolved one) can do nothing.
func (a *Actor) Can(p Permission) bool {
	if a == nil {
		return false
	}
	return a.IsSuper() || a.perms.Has(p)
}

// HasAdminAccess reports whether the actor may enter the admin area at all:
// SuperAdmin, or at least one admin-area permission. A signed-in member with
// no roles (e.g. a chat participant) is a user of the site, not of its admin,
// and so is one whose roles grant only site-only permissions (chat.moderate).
func (a *Actor) HasAdminAccess() bool {
	if a == nil {
		return false
	}
	if a.IsSuper() {
		return true
	}
	for p := range a.perms {
		if !siteOnlyPerms[p] {
			return true
		}
	}
	return false
}

// Permissions returns the actor's effective set. For SuperAdmin that is the
// whole catalog, so subset tests (CanGrant) need no special case.
func (a *Actor) Permissions() Set {
	if a == nil {
		return Set{}
	}
	if a.IsSuper() {
		return AllPermissions()
	}
	return a.perms
}

// CanGrant is the no-escalation rule: an actor may only hand out, or take
// away, permissions they hold themselves. It applies to defining a role's
// permissions and to assigning a role to a user. Without it, "users.update"
// would be a skeleton key — assign yourself a role that holds everything.
func (a *Actor) CanGrant(perms Set) bool {
	if a == nil {
		return false
	}
	return perms.SubsetOf(a.Permissions())
}

// ---- Request context ----

// ctxKeyActor is unexported for the reason apitoken keeps its key private:
// nothing outside this package can plant an actor in the context.
const ctxKeyActor = "authz.actor"

// SetActor stores the resolved actor for the rest of the request.
func SetActor(ctx rweb.Context, a *Actor) {
	ctx.Set(ctxKeyActor, a)
}

// ActorFrom returns the actor the admin guard resolved. ok=false means none
// was resolved (anonymous, or a route outside the admin guard).
func ActorFrom(ctx rweb.Context) (a *Actor, ok bool) {
	if !ctx.Has(ctxKeyActor) {
		return nil, false
	}
	a, ok = ctx.Get(ctxKeyActor).(*Actor)
	return a, ok && a != nil
}

// ---- Render params ----
//
// Modules render from a params map (map[string]map[string]string), not from
// the request context, so the actor's permissions are handed to them as a
// string in params["_global"]. This keeps the module.Module interface
// unchanged. The encoding is only ever used to decide what UI to show; every
// action the UI offers is re-checked by its handler.

// ParamKey is the params["_global"] key carrying the encoded permissions.
const ParamKey = "perms"

// superToken encodes the SuperAdmin bypass. "*" can never collide with a
// catalog permission.
const superToken = "*"

// ParamValue encodes the request's actor for render params. An empty string
// means no actor, which renders like one with no permissions.
func ParamValue(ctx rweb.Context) string {
	a, ok := ActorFrom(ctx)
	if !ok {
		return ""
	}
	return encode(a)
}

// encode is ParamValue without the context, so the encoding is testable.
func encode(a *Actor) string {
	if a.IsSuper() {
		return superToken
	}
	parts := make([]string, 0, len(a.perms))
	for _, p := range a.perms.Sorted() {
		parts = append(parts, string(p))
	}
	return strings.Join(parts, ",")
}

// FromParams decodes the actor that ParamValue encoded. Only permissions (and
// the username, already in _global) survive the trip; UserID is 0.
func FromParams(params map[string]map[string]string) *Actor {
	glob := params["_global"]
	a := &Actor{Username: glob["username"], perms: Set{}}
	raw := glob[ParamKey]
	if raw == superToken {
		a.BaseRole = SuperAdminRole
		return a
	}
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			a.perms[Permission(p)] = struct{}{}
		}
	}
	return a
}

// NewActorForTest builds an actor with the given permissions. For tests in
// other packages, which can't set the unexported perms field.
func NewActorForTest(userID int64, username string, baseRole int, perms ...Permission) *Actor {
	return &Actor{UserID: userID, Username: username, BaseRole: baseRole, perms: NewSet(perms...)}
}
