package auth_controller

import (
	"net/http"
	"strings"

	"github.com/rohanthewiz/church/app"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/cookie"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

// RWeb version of redirect with flash message
func RedirectRWeb(ctx rweb.Context, url, fl_msg string) error {
	if fl_msg != "" {
		fl := flash.NewFlash()
		fl.Info = fl_msg // todo warn and error
		fl.SetRWeb(ctx)
	}
	ctx.Response().Header().Set("Location", url)
	ctx.Response().WriteHeader(http.StatusSeeOther)
	return nil
}

// RWeb Middleware for storing session in context
// Logged in means we have
//  1. a valid session cookie
//  2. a (non-expired) session in redis whose key is the value of the session cookie
func UseCustomContextRWeb(ctx rweb.Context) error {
	// Get Session key
	sessKey := EnsureSessionCookieRWeb(ctx)
	
	// Get Session
	sess, err := session.GetSession(sessKey)
	if err != nil {
		if !strings.Contains(err.Error(), session.KeyNotExists) {
			logger.LogErr(serr.Wrap(err, "Unable to obtain session"), "session_key", sessKey)
		}
		// The session may have expired or was not written into the store as yet or some other error
		// so create a fresh session
		sess = session.Session{Key: sessKey}
	}
	
	// Store session data in RWeb context
	cctx.SetSessionInRWeb(ctx, &sess)
	
	// Check if admin
	if sess.Username != "" { // admins must have a username in session
		ctx.Set("isAdmin", true)
	}
	
	// Extend session
	_ = sess.Extend()
	
	return ctx.Next()
}

// RWeb Authorization middleware - Check if user is admin
func AdminGuardRWeb(ctx rweb.Context) error {
	if cctx.IsAdminFromRWeb(ctx) {
		sess, _ := cctx.GetSessionFromRWeb(ctx)
		if sess != nil {
			logger.Info("Successfully authorized for admin: " + sess.Username)
		}
		return ctx.Next()
	}
	// Redirect to login
	return RedirectRWeb(ctx, "/login", "Login required")
}

// EnsureSessionCookieRWeb - RWeb version
// Get session key from cookie or create new one
func EnsureSessionCookieRWeb(ctx rweb.Context) string {
	key, err := cookie.GetRWeb(ctx, session.CookieName)
	if err == nil && key != "" {
		logger.Debug("we have an existing session key, returning it: " + key)
		return key
	}

	// Create new session key
	key = auth.RandomKey()
	cookie.SetRWeb(ctx, session.CookieName, key)
	logger.Debug("Setting new session key in cookie: " + key)
	return key
}