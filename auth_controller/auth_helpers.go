package auth_controller

import (
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/cookie"
	"github.com/rohanthewiz/church/resource/session"
	. "github.com/rohanthewiz/logger"
)

// Creates a new session for the user -- usually done on login
func CreateSession(username string, c echo.Context) error {
	oldKey, err := cookie.Get(c, session.CookieSession)
	if err == nil {
		session.DestroySession(oldKey) // destroy any existing session
	}
	newKey := auth.RandomKey()
	Log("Debug", "Creating new session cookie", "name", session.CookieSession, "value", newKey)
	cookie.Set(c, session.CookieSession, newKey)
	// For session cookie, don't set an expiration so it might be removed on browser window close
	// cookie.Expires = time.Now().Add(24 * time.Hour)

	sess := session.Session{ Username: username }
	err = sess.Save(newKey)
	if err != nil {
		LogErr(err, "Error creating session", "username", username, "key", newKey)
		return err
	}
	Log("Info", "Session created successfully", "username", username, "key", newKey)
	return err
}

