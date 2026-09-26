package auth_controller

import (
	"bytes"
	"net/http"

	"github.com/rohanthewiz/church/app"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

// GET /login - Login Form
func LoginHandler(ctx rweb.Context) error {
	pg, err := page.LoginPage()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNew(ctx), map[string]map[string]string{
		// username lets the nav filter the Admin submenu for a signed-in viewer
		"_global": {"user_agent": ctx.UserAgent(), "username": cctx.GetUsername(ctx)},
	}, app.IsLoggedIn(ctx))
	return ctx.WriteHTML(buf.String())
}

// POST /auth  // Authentication
// Todo: test that disabled user cannot login
func AuthHandler(ctx rweb.Context) error {
	// The login form carries a CSRF token like every other form (login CSRF
	// would let an attacker silently sign a victim into an account they
	// control). Expiry redirects back to a fresh form rather than erroring.
	if !app.VerifyFormToken(ctx.Request().FormValue("csrf")) {
		return app.Redirect(ctx, "/login", "Your login form expired. Please try again.")
	}
	username := ctx.Request().FormValue("username")
	password := ctx.Request().FormValue("password")
	if len(username) < 1 || len(password) < 1 {
		return app.Redirect(ctx, "/login", "Username and Password required for login")
	}
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle for login")
		return app.Redirect(ctx, "/login", "Something went wrong. Please try again.")
	}
	stored_pass_hash, stored_salt, err := user.UserCreds(dbH, username) // creds from DB
	if err != nil {
		logger.LogErr(err, "Error obtaining user creds from DB", "username", username)
		return app.Redirect(ctx, "/login", "Invalid username and/or password")
	}
	if auth.PasswordHash(password, stored_salt) != stored_pass_hash {
		// Log the username only — never the submitted password. A failed attempt
		// is often a typo of the real password, so logging it would put live
		// credentials in the log stream (and in any log aggregator downstream).
		logger.Log("warn", "Login attempt failed", "reason", "Invalid username or password",
			"Username", username)
		return app.Redirect(ctx, "/login", "Invalid username and/or password.")
	}
	// At this point login is successful
	err = StartSession(username, ctx)
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).WriteString(
			"Something went wrong on the server and we weren't able to log you in")
	}
	// Login successful
	return app.Redirect(ctx, "/", "Welcome "+username+"!")
}

// GET /logout
func LogoutHandler(ctx rweb.Context) error { // don't ever send an error back - redirect instead
	sessVal, err := ctx.GetCookie(session.CookieName)
	if err != nil {
		logger.Log("Info", "Couldn't retrieve session cookie", "when", "logout")
		return app.Redirect(ctx, "/pages/home", "Hmm, tried to log you out, but you weren't logged in, or something else is quirky.")
	}
	err = session.DestroySession(sessVal)
	if err != nil {
		logger.LogErr(err, "Error destroying session")
	}
	ctx.DeleteCookie(session.CookieName)
	return app.Redirect(ctx, "/pages/home", "Logged out")
}

// RegisterUser was deprecated as a security loophole (unauthenticated user
// creation with a caller-chosen role, credentials in the query string) and is no
// longer routed. User creation goes through the admin-guarded
// user_controller.UpsertUser instead. Kept for reference only.
// func RegisterUser(ctx rweb.Context) error {
// 	salt := auth.GenSalt("j$&@randomness!!$$$")
// 	pass_hash := auth.PasswordHash(ctx.Request().QueryParam("password"), salt)
// 	role_int, err := strconv.Atoi(ctx.Request().QueryParam("role"))
// 	if err != nil {
// 		logger.LogErr(err, "Invalid role supplied")
// 		return app.Redirect(ctx, "/admin/users", "Invalid role supplied")
// 	}
// 	err = user.SaveUser(ctx.Request().QueryParam("username"), null.NewString(pass_hash, true), null.NewString(salt, true), role_int)
// 	if err != nil {
// 		logger.LogErr(err, "Unable to SaveUser")
// 		return app.Redirect(ctx, "/admin/users", "Unable to register user")
// 	}
// 	logger.Log("Info", "User successfully created", "user", ctx.Request().QueryParam("username"))
// 	return app.Redirect(ctx, "/admin/users", "User successfully registered")
// }
