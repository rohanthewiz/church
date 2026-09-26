package auth_controller

import (
	"net/http"
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/config"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

// Redirect sets an info-level flash message (when given) and redirects
func Redirect(ctx rweb.Context, url, fl_msg string) error {
	if fl_msg != "" {
		fl := flash.NewFlash()
		fl.Info = fl_msg // todo warn and error
		fl.Set(ctx)
	}
	return ctx.Redirect(http.StatusSeeOther, url)
}

// UseCustomContext is middleware that stores the session in the request context
// Logged in means we have
//  1. a valid session cookie
//  2. a (non-expired) session in the in-process kvstore whose key is the value of the session cookie
func UseCustomContext(ctx rweb.Context) error {
	// Get Session key
	sessKey := EnsureSessionCookie(ctx)

	// Get Session
	sess, err := session.GetSession(sessKey)
	if err != nil {
		if !strings.Contains(err.Error(), session.KeyNotExists) {
			logger.LogErr(serr.Wrap(err, "Unable to obtain session"), "session_key", sessKey)
		}
		// The session may have expired or was not written into the store as yet or some other error
		// so create a fresh session
		sess = session.Session{Key: sessKey}
	}

	// Store session data in RWeb context
	cctx.SetSession(ctx, &sess)

	// Check if admin
	if sess.Username != "" { // admins must have a username in session
		ctx.Set("isAdmin", true)
	}

	// Extend session
	_ = sess.Extend()

	return ctx.Next()
}

// ---- Admin authorization ----
//
// Two layers, because of how rweb runs group middleware:
//
//	request ─► UseCustomContext ─► AdminGuard ─► Require(perm, handler)
//	                                   resolves actor     enforces: runs handler
//	                                   (or a denial)      only if allowed
//
// rweb continues into the route handler whenever a middleware returns nil
// without calling Next() (see rweb Group.go, and the note at
// apitoken.APIGuard). A redirect returns nil. So a group guard that "denies"
// by redirecting still runs the handler behind the 303, and the handler's
// body and side effects happen anyway. The guard therefore can't be what
// enforces access. It resolves who is asking and records any denial. The
// per-route decorators (Require / RequireAdmin) are what keep a handler from
// running, and every admin route is wrapped in one.

// ctxKeyDenial carries the guard's decision to the decorators so they replay
// the same redirect and flash rather than re-deriving them.
const ctxKeyDenial = "authz.denial"

type denial struct {
	url, msg string
	sev      app.FlashSeverity
}

// AdminGuard resolves the signed-in admin (authz.Actor) for the request.
// Permissions are loaded from the database each time, so a revoked role or a
// disabled account takes effect on the next request.
func AdminGuard(ctx rweb.Context) error {
	deny := func(d denial) error {
		ctx.Set(ctxKeyDenial, d)
		return app.RedirectSev(ctx, d.url, d.msg, d.sev)
	}

	sess, err := cctx.GetSession(ctx)
	if err != nil || sess == nil || sess.Username == "" {
		return deny(denial{"/login", "Login required", app.FlashInfo})
	}

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle for admin authorization")
		return deny(denial{"/", "The admin area is unavailable right now. Please try again shortly.", app.FlashError})
	}
	actor, found, err := authz.LoadActor(dbH, sess.Username)
	if err != nil {
		logger.LogErr(err, "Error resolving admin permissions", "username", sess.Username)
		return deny(denial{"/", "The admin area is unavailable right now. Please try again shortly.", app.FlashError})
	}
	if !found {
		// Deleted or disabled since the session began: sign-in no longer holds.
		return deny(denial{"/login", "Login required", app.FlashInfo})
	}
	authz.SetActor(ctx, actor)

	if !actor.HasAdminAccess() {
		return deny(denial{"/", "Your account does not have access to the admin area.", app.FlashWarn})
	}
	logger.Info("Successfully authorized for admin: " + sess.Username)
	return ctx.Next()
}

// RequireAdmin wraps a handler that any admin may use (dashboard, logout).
func RequireAdmin(next rweb.Handler) rweb.Handler {
	return Require("", next)
}

// RequireSuper wraps a handler that only a SuperAdmin may use. It is for
// operator tools rather than content work, such as /debug/*, which flips
// process-wide render state for every visitor. A catalog permission would
// let a site hand that to anyone who can edit roles, and no church role
// needs it.
//
// Denial behaves like Require: the guard's recorded denial replays first,
// and a signed-in admin who isn't a SuperAdmin lands on the dashboard with a
// warning.
func RequireSuper(next rweb.Handler) rweb.Handler {
	return Require("", func(ctx rweb.Context) error {
		// Require has already confirmed an actor with admin access
		if actor, ok := authz.ActorFrom(ctx); !ok || !actor.IsSuper() {
			logger.Info("SuperAdmin-only route denied", "path", ctx.Request().Path())
			return app.RedirectWarn(ctx, config.AdminPrefix+"/home",
				"Only a SuperAdmin can use that.")
		}
		return next(ctx)
	})
}

// Require wraps a handler so it runs only for an admin holding perm. An empty
// perm means admin access alone suffices (see RequireAdmin).
//
// A signed-in admin lacking the permission goes to the dashboard with a
// warning rather than a 403 page: the dashboard only offers what they can do,
// so it is the useful place to land.
func Require(perm authz.Permission, next rweb.Handler) rweb.Handler {
	return func(ctx rweb.Context) error {
		if d, ok := ctx.Get(ctxKeyDenial).(denial); ok {
			return app.RedirectSev(ctx, d.url, d.msg, d.sev)
		}
		actor, ok := authz.ActorFrom(ctx)
		if !ok {
			// No guard ran ahead of this route: a wiring bug. Fail closed.
			logger.Warn("admin route reached without AdminGuard", "path", ctx.Request().Path())
			return app.Redirect(ctx, "/login", "Login required")
		}
		if !actor.HasAdminAccess() {
			return app.RedirectWarn(ctx, "/", "Your account does not have access to the admin area.")
		}
		if perm != "" && !actor.Can(perm) {
			logger.Info("Admin permission denied", "username", actor.Username, "permission", string(perm),
				"path", ctx.Request().Path())
			return app.RedirectWarn(ctx, config.AdminPrefix+"/home",
				"You don't have permission to do that ("+string(perm)+").")
		}
		return next(ctx)
	}
}

// EnsureSessionCookie returns the session key, setting a new session cookie if needed
func EnsureSessionCookie(ctx rweb.Context) string {
	key, err := ctx.GetCookie(session.CookieName)
	if err == nil && key != "" {
		logger.Debug("we have an existing session key, returning it: " + key)
		return key
	}

	// Create new session key
	key = auth.RandomKey()
	err = ctx.SetCookie(session.CookieName, key)
	if err != nil {
		logger.LogErr(err, "failed to set session cookie")
	}
	logger.Debug("Setting new session key in cookie: " + key)
	return key
}
