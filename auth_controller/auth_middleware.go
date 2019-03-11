package auth_controller

import (
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	customctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/resource/cookie"
	"github.com/rohanthewiz/church/resource/session"
	. "github.com/rohanthewiz/logger"
)

// Authorization middleware - Read the Admin value on the custom context
// assuming that the LoadSessionIntoContext middleware always runs before this
func AuthAdmin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if cc, ok := c.(*customctx.CustomContext); ok && cc.Admin {
			Log("Info", "Successfully authorized for admin")
			return next(c)
		}
		Log("Warn", "In Authorization - Admin is false - redirecting")
		app.Redirect(c, "/login", "Login required")
		return nil
	}
}

// Middleware for storing session in our custom context
// Logged in means we have
// 		1) a valid session cookie
// 		2) a (non-expired) session in redis whose key is the value of the session cookie
// Note: Form tokens will use the same concept
// 		1) a valid form token,
// 		2) a (non-expired) session in redis whose key is the value of the form token
func LoadSessionIntoContext(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		Log("Info", "Request: " + c.Request().Method + " " + c.Request().RequestURI)

		cc, ok := c.(*customctx.CustomContext)
		if !ok { Log("Error", "Couldn't typecast to CustomContext"); cc.Admin = false; return next(c) }

		sessKey := EnsureSessionCookie(c)
		//sessKey, err := cookie.Get(c, session.CookieName)
		//if err != nil || sessKey == "" {
		//	//Log("Debug", "Could not retrieve session key from cookie: " + session.CookieName)
		//	cc.Admin = false; return next(c)
		//}

		sess, err := session.GetSession(sessKey)
		if err != nil {
			Log("Warn", "Unable to retrieve session from store", "session_key", sessKey,
				"tip", "The session is probably expired - we will blank the session cookie")
			cookie.Clear(c, session.CookieName)
			cc.Admin = false; return next(c)
		}
		//Log("Info", "We have a valid (admin) session. Setting `Admin = true` on context")
		cc.Admin = true
		cc.Session = sess
		//Log("Info", "Extending session", "username", username, "session_key", sessKey)
		sess.Extend(sessKey)
		return next(c)
	}
}

func LoadSessionIntoNonAdminContext(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		Log("Info", "Request: " + c.Request().Method + " " + c.Request().RequestURI)

		cc, ok := c.(*customctx.CustomContext)
		if !ok {
			Log("Error", "Couldn't typecast Echo context to CustomContext")
			cc.Admin = false;	return next(c)
		}

		sessKey := EnsureSessionCookie(c)
		//sessKey, err := cookie.Get(c, session.CookieName)
		//if err != nil || sessKey == "" {
		//	//Log("Debug", "Could not retrieve session key from cookie: " + session.CookieName)
		//
		//	cc.Admin = false; return next(c)
		//}

		sess, err := session.GetSession(sessKey)
		if err != nil {
			Log("Warn", "Unable to retrieve session from store", "session_key", sessKey,
				"tip", "The session is probably expired - we will blank the session cookie")
			cookie.Clear(c, session.CookieName)
			cc.Admin = false; return next(c)
		}
		//Log("Info", "We have a valid (admin) session. Setting `Admin = true` on context")
		//cc.Admin = true
		cc.Session = sess
		//Log("Info", "Extending session", "username", username, "session_key", sessKey)
		sess.Extend(sessKey)
		return next(c)
	}
}
