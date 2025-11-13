package auth_controller

import (
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

// Establish a new session for the user -- usually done on login
func StartSessionRWeb(username string, ctx rweb.Context) (err error) {
	key := NewSessionKeyRWeb(ctx)

	sess := session.Session{Username: username}
	err = sess.Save(key)
	if err != nil {
		logger.LogErr(err, "Error saving session for user: "+username+", key"+key)
		return err
	}
	logger.Info("Session created successfully for user: " + username + ", key" + key)
	return
}

// Basically set new session key into our session cookie
func NewSessionKeyRWeb(ctx rweb.Context) (key string) {
	key = auth.RandomKey()
	err := ctx.SetCookie(session.CookieName, key)
	if err != nil {
		logger.LogErr(err, "Failed to set session cookie")
	}
	logger.Log("Debug", "Setting new session key in cookie: "+key)
	// For session cookie, don't set an expiration so it might be removed on browser window close
	return
}