package auth_controller

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"gopkg.in/nullbio/null.v6"
)

// GET /login - Login Form
func LoginHandlerRWeb(ctx rweb.Context) error {
	pg, err := page.LoginPage()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		"_global": {"user_agent": ctx.UserAgent()},
	}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

// POST /auth  // Authentication
// Todo: test that disabled user cannot login
func AuthHandlerRWeb(ctx rweb.Context) error {
	username := ctx.Request().FormValue("username")
	password := ctx.Request().FormValue("password")
	if len(username) < 1 || len(password) < 1 {
		return app.RedirectRWeb(ctx, "/login", "Username and Password required for login")
	}
	stored_pass_hash, stored_salt, err := user.UserCreds(username) // creds from DB
	if err != nil {
		logger.LogErr(err, "Error obtaining user creds from DB", "username", username)
		return app.RedirectRWeb(ctx, "/login", "Invalid username and/or password")
	}
	if auth.PasswordHash(password, stored_salt) != stored_pass_hash {
		logger.Log("warn", "Login attempt failed", "reason", "Invalid username or password",
			"Username", username, "password", password)
		return app.RedirectRWeb(ctx, "/login", "Invalid username and/or password.")
	}
	// At this point login is successful
	err = StartSessionRWeb(username, ctx)
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).WriteString(
			"Something went wrong on the server and we weren't able to log you in")
	}
	// Login successful
	return app.RedirectRWeb(ctx, "/", "Welcome "+username+"!")
}

// GET /logout
func LogoutHandlerRWeb(ctx rweb.Context) error { // don't ever send an error back - redirect instead
	sessVal, err := ctx.GetCookie(session.CookieName)
	if err != nil {
		logger.Log("Info", "Couldn't retrieve session cookie", "when", "logout")
		return app.RedirectRWeb(ctx, "/pages/home", "Hmm, tried to log you out, but you weren't logged in, or something else is quirky.")
	}
	err = session.DestroySession(sessVal)
	if err != nil {
		logger.LogErr(err, "Error destroying session")
	}
	ctx.DeleteCookie(session.CookieName)
	return app.RedirectRWeb(ctx, "/pages/home", "Logged out")
}

// POST /adduser
// This function is deprecated - security loophole!
func RegisterUserRWeb(ctx rweb.Context) error {
	salt := auth.GenSalt("j$&@randomness!!$$$")
	pass_hash := auth.PasswordHash(ctx.Request().QueryParam("password"), salt)
	role_int, err := strconv.Atoi(ctx.Request().QueryParam("role"))
	if err != nil {
		logger.LogErr(err, "Invalid role supplied")
		return app.RedirectRWeb(ctx, "/admin/users", "Invalid role supplied")
	}
	err = user.SaveUser(ctx.Request().QueryParam("username"), null.NewString(pass_hash, true), null.NewString(salt, true), role_int)
	if err != nil {
		logger.LogErr(err, "Unable to SaveUser")
		return app.RedirectRWeb(ctx, "/admin/users", "Unable to register user")
	}
	logger.Log("Info", "User successfully created", "user", ctx.Request().QueryParam("username"))
	return app.RedirectRWeb(ctx, "/admin/users", "User successfully registered")
}
