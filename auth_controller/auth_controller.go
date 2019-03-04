package auth_controller

import (
	"bytes"
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/cookie"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/church/template"
	. "github.com/rohanthewiz/logger"
	"gopkg.in/nullbio/null.v6"
	"net/http"
	"strconv"
)

// GET /login - Login Form
func LoginHandler(c echo.Context) error {
	pg, err := page.LoginPage()
	if err != nil {
		c.Error(err)
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNew(c), nil, app.IsLoggedIn(c))
	c.HTMLBlob(200, buf.Bytes())
	return nil
}

// POST /auth  // Authentication
// Todo: test that disabled user cannot login
func AuthHandler(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")
	if len(username) < 1 || len(password) < 1 {
		app.Redirect(c, "/login", "Username and Password required for login")
		return nil
	}
	stored_pass_hash, stored_salt, err := user.UserCreds(username)  // creds from DB
	if err != nil {
		LogErr(err, "Error obtaining user creds from DB", "username", username)
		app.Redirect(c, "/login", "Invalid username and/or password")
		return nil
	}
	if auth.PasswordHash(password, stored_salt) != stored_pass_hash {
		Log("warn", "Login attempt failed", "reason", "Invalid username or password",
			"Username", username, "password", password)
		app.Redirect(c, "/login", "Invalid username and/or password.")
		return nil
	}
	// At this point login is successful
	err = CreateSession(username, c)
	if err != nil {
		c.String(http.StatusInternalServerError,
			"Something went wrong on the server and we weren't able to log you in")
		return nil
	}
	// Login successful
	app.Redirect(c, "/", "Welcome " + username + "!")
	return nil
}

// GET /logout
func LogoutHandler(c echo.Context) (error) {  // don't ever send an error back to Echo - redirect instead
	sessVal, err := cookie.Get(c, session.CookieSession)
	if err != nil {
		Log("Info", "Couldn't retrieve session cookie", "when", "logout")
		app.Redirect(c, "/pages/home", "Hmm, tried to log you out, but you weren't logged in, or something else is quirky.")
		return nil
	}
	err = session.DestroySession(sessVal)
	cookie.Clear(c, session.CookieSession)
	app.Redirect(c, "/pages/home", "Logged out")
	return nil
}

// POST /adduser
// This function is deprecated - security loophole!
func RegisterUser(c echo.Context) error {
	salt := auth.GenSalt("j$&@randomness!!$$$")
	pass_hash := auth.PasswordHash(c.QueryParam("password"), salt)
	role_int, err := strconv.Atoi(c.QueryParam("role"))
	if err != nil {
		LogErr(err, "Invalid role supplied")
		app.Redirect(c, "/admin/users", "Invalid role supplied")
		return nil
	}
	err = user.SaveUser(c.QueryParam("username"), null.NewString(pass_hash, true), null.NewString(salt, true), role_int)
	if err != nil {
		LogErr(err, "Unable to SaveUser")
		app.Redirect(c, "/admin/users", "Unable to register user")
		return nil
	}
	Log("Info", "User successfully created", "user", c.QueryParam("username"))
	app.Redirect(c, "/admin/users", "User successfully registered")
	return nil
}
