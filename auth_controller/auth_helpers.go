package auth_controller

import (
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/cookie"
	"github.com/rohanthewiz/church/resource/session"
	. "github.com/rohanthewiz/logger"
)

// Establish a new session for the user -- usually done on login
func StartSession(username string, c echo.Context) (err error) {
	key := NewSessionKey(c)

	sess := session.Session{Username: username}
	err = sess.Save(key)
	if err != nil {
		LogErr(err, "Error saving session for user: "+username+", key"+key)
		return err
	}
	Info("Session created successfully for user: " + username + ", key" + key)
	return
}

// Ensure we have a cookie with a session key
func EnsureSessionCookie(c echo.Context) (key string) {
	var err error
	key, err = cookie.Get(c, session.CookieName)
	if err == nil && key != "" {
		Debug("we have an existing session key, returning it: " + key)
		return
	}

	key = NewSessionKey(c)
	return
}

// Basically set new session key into our session cookie
func NewSessionKey(c echo.Context) (key string) {
	key = auth.RandomKey()
	cookie.Set(c, session.CookieName, key)
	Log("Debug", "Setting new session key in cookie: "+key)
	// For session cookie, don't set an expiration so it might be removed on browser window close
	// cookie.Expires = time.Now().Add(24 * time.Hour)
	return
}
