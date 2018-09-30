package auth_controller

import (
	. "github.com/rohanthewiz/logger"
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/cookie"
)

// Creates a new session for the user -- usually done on login
func CreateSession(username string, c echo.Context) error {
	oldKey, err := cookie.Get(c, session.CookieSession)
	if err == nil {
		DestroySession(oldKey) // destroy any existing session
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

// This is currently NU because the whole session is retrieved and stored in the custom context
func GetFormReferrer(c echo.Context) (ref string, err error) {
	key, err := cookie.Get(c, session.CookieSession)
	if err != nil || key == "" { return ref, err }

	sess, err := session.GetSession(key)
	if err != nil { return ref, err }
	ref = sess.FormReferrer
	return
}

func SetFormReferrer(c echo.Context) (err error) {
	key, err := cookie.Get(c, session.CookieSession)
	if err != nil || key == "" { return err }

	sess, err := session.GetSession(key)
	if err != nil { return err }

	sess.FormReferrer = c.Request().Referer()
	return sess.Save(key)
}

// Given a session cookie name, delete it's session from the store
func DestroySession(sess_val string) (err error) {
	if sess_val != "" {
		err = session.DeleteSession(sess_val)  // Delete the session from the store - it should expire anyway
		if err != nil {
			Log("Info", "Unable to delete session", "session_key", sess_val, "Error", err.Error())
		}
		//Log("Info", "Logout", "stage", "Deleted Session from store")
	}
	return
}
