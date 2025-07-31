package context

import (
	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

// GetSessionFromRWeb retrieves the session from RWeb context
func GetSessionFromRWeb(ctx rweb.Context) (*app.Session, error) {
	if ctx.Has("session") {
		sess, ok := ctx.Get("session").(*app.Session)
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

// GetUserIdFromRWeb retrieves the user ID from context
func GetUserIdFromRWeb(ctx rweb.Context) string {
	if ctx.Has("userId") {
		if userId, ok := ctx.Get("userId").(string); ok {
			return userId
		}
	}
	return ""
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
func SetSessionInRWeb(ctx rweb.Context, sess *app.Session) {
	if sess != nil {
		ctx.Set("session", sess)
		ctx.Set("isAdmin", sess.IsAdmin())
		ctx.Set("userId", sess.UserId)
		ctx.Set("username", sess.Username)
	}
}

// ClearSessionFromRWeb removes session data from RWeb context
func ClearSessionFromRWeb(ctx rweb.Context) {
	ctx.Delete("session")
	ctx.Delete("isAdmin")
	ctx.Delete("userId")
	ctx.Delete("username")
}