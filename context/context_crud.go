package context

import (
	"fmt"
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/resource/cookie"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

func SetFormReferrer(c echo.Context) (err error) {
	//key, err := cookie.Get(c, CookieName)
	//if err != nil || key == "" {
	//	return err
	//}
	//
	//sess, err := GetSession(key)
	//if err != nil {
	//	return err
	//}
	cc, ok := c.(*CustomContext)
	if !ok {
		err = serr.NewSErr("Couldn't typecast Echo context to CustomContext")
		logger.LogErr(err)
		return
	}

	//sess, err := GetSession(key)
	//if err != nil {
	//	return serr.Wrap(err, "Unable to obtain session", "key", key)
	//}
	sess := cc.Session

	sess.FormReferrer = c.Request().Referer()
	return sess.Save(sess.Key)
}

func SetLastDonationURL(c echo.Context, url string) (err error) {
	//key, err := cookie.Get(c, CookieName)
	//if err != nil {
	//	return serr.Wrap(err, "Unable to get value of session cookie")
	//}
	//
	//if key == "" {
	//	return serr.NewSErr("Session cookie is empty")
	//}
	cc, ok := c.(*CustomContext)
	if !ok {
		err = serr.NewSErr("Couldn't typecast Echo context to CustomContext")
		logger.LogErr(err)
		return
	}
	sess := cc.Session
	sess.LastGivingReceiptURL = url
	logger.Log("Info", fmt.Sprintf("On set of Last Donation URL - Session -> %#v\n", sess))
	return sess.Save(sess.Key)
}

// This is currently NU because the whole session is retrieved and stored in the custom context
func GetFormReferrer(c echo.Context) (ref string, err error) {
	key, err := cookie.Get(c, session.CookieName)
	if err != nil || key == "" {
		return ref, err
	}

	sess, err := session.GetSession(key)
	if err != nil {
		return ref, err
	}
	ref = sess.FormReferrer
	return
}

