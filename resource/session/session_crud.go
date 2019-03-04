package session

import (
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/resource/cookie"
	"github.com/rohanthewiz/logger"
)

// This is currently NU because the whole session is retrieved and stored in the custom context
func GetFormReferrer(c echo.Context) (ref string, err error) {
	key, err := cookie.Get(c, CookieSession)
	if err != nil || key == "" {
		return ref, err
	}

	sess, err := GetSession(key)
	if err != nil {
		return ref, err
	}
	ref = sess.FormReferrer
	return
}

func SetFormReferrer(c echo.Context) (err error) {
	key, err := cookie.Get(c, CookieSession)
	if err != nil || key == "" {
		return err
	}

	sess, err := GetSession(key)
	if err != nil {
		return err
	}

	sess.FormReferrer = c.Request().Referer()
	return sess.Save(key)
}

func SetLastDonationURL(c echo.Context, url string) (err error) {
	key, err := cookie.Get(c, CookieSession)
	if err != nil || key == "" {
		return err
	}

	sess, err := GetSession(key)
	if err != nil {
		return err
	}

	sess.LastGivingReceiptURL = url
	return sess.Save(key)
}

// Given a session cookie name, delete it's session from the store
func DestroySession(sess_val string) (err error) {
	if sess_val != "" {
		err = DeleteSession(sess_val) // Delete the session from the store - it should expire anyway
		if err != nil {
			logger.Log("Info", "Unable to delete session", "session_key", sess_val, "Error", err.Error())
		}
		//Log("Info", "Logout", "stage", "Deleted Session from store")
	}
	return
}
