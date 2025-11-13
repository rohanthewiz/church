package context

import (
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

// GetSessionFromRWeb retrieves the session from RWeb context
func GetSessionFromRWeb(ctx rweb.Context) (*session.Session, error) {
	if ctx.Has("session") {
		sess, ok := ctx.Get("session").(*session.Session)
		if ok && sess != nil {
			return sess, nil
		}
	}
	return nil, serr.New("no session found in context")
}

// IsAdminFromRWeb checks if the current user is an admin
func IsAdminFromRWeb(ctx rweb.Context) bool {
	if ctx.Has("isAdmin") {
		return ctx.Get("isAdmin").(bool)
	}
	return false
}


// GetUsernameFromRWeb retrieves the username from context
func GetUsernameFromRWeb(ctx rweb.Context) string {
	if ctx.Has("username") {
		if username, ok := ctx.Get("username").(string); ok {
			return username
		}
	}
	return ""
}

// SetSessionInRWeb stores session data in RWeb context
func SetSessionInRWeb(ctx rweb.Context, sess *session.Session) {
	if sess != nil {
		ctx.Set("session", sess)
		// Note: isAdmin is set separately in the middleware based on whether username exists
		ctx.Set("username", sess.Username)
	}
}

// ClearSessionFromRWeb removes session data from RWeb context
func ClearSessionFromRWeb(ctx rweb.Context) {
	ctx.Delete("session")
	ctx.Delete("isAdmin")
	ctx.Delete("username")
}

// SetFormReferrerRWeb saves the referrer URL to the session
func SetFormReferrerRWeb(ctx rweb.Context) error {
	sess, err := GetSessionFromRWeb(ctx)
	if err != nil {
		return serr.Wrap(err, "unable to get session")
	}
	
	sess.FormReferrer = ctx.Request().Header("Referer")
	return sess.Save(sess.Key)
}

// SetLastDonationURLRWeb saves the last donation receipt URL to the session
func SetLastDonationURLRWeb(ctx rweb.Context, url string) error {
	sess, err := GetSessionFromRWeb(ctx)
	if err != nil {
		return serr.Wrap(err, "unable to get session")
	}
	
	sess.LastGivingReceiptURL = url
	return sess.Save(sess.Key)
}