package auth_controller

import (
	"strings"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/resource/session"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// Authorization middleware - Read the Admin value on the custom context
// assuming that the UseCustomContext middleware always runs before this
func AdminGuard(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if cc, ok := c.(*cctx.CustomContext); ok && cc.Admin {
			Info("Successfully authorized for admin: " + cc.Session.Username)
			return next(c)
		}
		// Turning this off for deployment // Warn("In Authorization - Admin is false - redirecting")
		app.Redirect(c, "/login", "Login required")
		return nil
	}
}

// Middleware for storing session in our custom context
// Logged in means we have
//  1. a valid session cookie
//  2. a (non-expired) session in redis whose key is the value of the session cookie
//
// Note: Form tokens will use the same concept
//  1. a valid form token,
//  2. a (non-expired) session in redis whose key is the value of the form token
//
// Be sure to return the custom context to the next handler
func UseCustomContext(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// This needs to go into a separate middleware called Request Logger
		// Log("Info", "Request: " + c.Request().Method + " " + c.Request().RequestURI)

		// Wrap existing context
		cc := &cctx.CustomContext{
			c, false, session.Session{},
		}
		// Get Session key
		sessKey := EnsureSessionCookie(c)
		// Get Session
		sess, err := session.GetSession(sessKey)
		if err != nil {
			if !strings.Contains(err.Error(), session.KeyNotExists) {
				LogErr(serr.Wrap(err, "Unable to obtain session", "session_key", sessKey))
			}
			// The session may have expired or was not written into the store as yet or some other error
			// so create a fresh session
			sess = session.Session{Key: sessKey}
		}
		if sess.Username != "" { // admins must have a username in sesssion
			cc.Admin = true
		}
		cc.Session = sess
		// Log("Info", "Extending session", "username", username, "session_key", sessKey)
		_ = sess.Extend()
		return next(cc)
	}
}

// func UseCustomContext(next echo.HandlerFunc) echo.HandlerFunc {
//	return func(c echo.Context) error {
//		Log("Info", "Request: " + c.Request().Method + " " + c.Request().RequestURI)
//
//		cc, ok := c.(*cctx.CustomContext)
//		if !ok {
//			Log("Error", "Couldn't typecast to CustomContext")
//			cc.Admin = false; return next(c)
//		}
//		Log("Debug", "At this point we should have a custom context")
//
//		sessKey := EnsureSessionCookie(c)
//		//sessKey, err := cookie.Get(c, session.CookieName)
//		//if err != nil || sessKey == "" {
//		//	//Log("Debug", "Could not retrieve session key from cookie: " + session.CookieName)
//		//	cc.Admin = false; return next(c)
//		//}
//
//		sess, err := session.GetSession(sessKey)
//		if err != nil {
//			LogErr(err, "In Admin: Unable to retrieve session from store", "session_key", sessKey)
//				//"tip", "The session is probably expired - we will blank the session cookie")
//			//cookie.Clear(c, session.CookieName)
//			cc.Admin = false
//			return next(c)
//		}
//		Log("Info", "We have a valid (admin) session. Setting `Admin = true` on context")
//		cc.Admin = true
//		sess.Key = sessKey
//		cc.Session = sess
//		//Log("Info", "Extending session", "username", username, "session_key", sessKey)
//		sess.Extend(sessKey)
//		return next(c)
//	}
// }

// func LoadSessionIntoNonAdminContext(next echo.HandlerFunc) echo.HandlerFunc {
//	return func(c echo.Context) error {
//		Log("Info", "Request: " + c.Request().Method + " " + c.Request().RequestURI)
//
//		cc, ok := c.(*cctx.CustomContext)
//		if !ok {
//			Log("Error", "Couldn't typecast Echo context to CustomContext for non-admin")
//			cc.Admin = false;	return next(c)
//		}
//
//		sessKey := EnsureSessionCookie(c)
//		//sessKey, err := cookie.Get(c, session.CookieName)
//		//if err != nil || sessKey == "" {
//		//	//Log("Debug", "Could not retrieve session key from cookie: " + session.CookieName)
//		//
//		//	cc.Admin = false; return next(c)
//		//}
//
//		sess, err := session.GetSession(sessKey)
//		if err != nil {
//			Log("Warn", "In NonAdmin: Unable to retrieve session from store", "session_key", sessKey,
//				"tip", "The session is probably expired - we will blank the session cookie")
//			cookie.Clear(c, session.CookieName)
//			cc.Admin = false; return next(c)
//		}
//		Log("Info", "We have a valid (non-admin) session.")
//		cc.Admin = false
//		sess.Key = sessKey
//		cc.Session = sess
//		//Log("Info", "Extending session", "username", username, "session_key", sessKey)
//		sess.Extend(sessKey)
//		return next(c)
//	}
// }
