package auth_controller

import (
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/cookie"
	"github.com/rohanthewiz/church/resource/session"
	. "github.com/rohanthewiz/logger"
)

// Creates a new session for the user -- usually done on login
func CreateSession(username string, c echo.Context) (err error) {
	key := EnsureSessionCookie(c)

	sess := session.Session{ Username: username }
	err = sess.Save(key)
	if err != nil {
		LogErr(err, "Error creating session", "username", username, "key", key)
		return err
	}
	Log("Info", "Session created successfully", "username", username, "key", key)
	return
}


// Ensure we have a cookie with a session key
// TODO ! Be sure loglevel is somewhere above DEBUG in production
func EnsureSessionCookie(c echo.Context) (key string) {
	var err error
	key, err = cookie.Get(c, session.CookieName)
	if err == nil && key != "" {
		Log("Info", "we have a good existing session key, return it", "key", key)
		return
	}

	key = auth.RandomKey()
	Log("Debug", "Setting new session key in cookie", "cookie_name", session.CookieName, "value", key)
	cookie.Set(c, session.CookieName, key)
	// For session cookie, don't set an expiration so it might be removed on browser window close
	// cookie.Expires = time.Now().Add(24 * time.Hour)
	return
}