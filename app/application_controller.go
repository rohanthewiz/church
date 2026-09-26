package app

import (
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/rweb"
)

// FlashSeverity selects which flash channel a redirect message uses. The flash
// renderer styles each channel differently (flash-info / flash-warn / flash-error).
type FlashSeverity int

const (
	FlashInfo FlashSeverity = iota
	FlashWarn
	FlashError
)

// Redirect handles redirects with an info-level flash message for RWeb.
// Kept for backward compatibility; delegates to RedirectSev.
func Redirect(ctx rweb.Context, url, fl_msg string) error {
	return RedirectSev(ctx, url, fl_msg, FlashInfo)
}

// RedirectWarn redirects with a warning-level flash message.
func RedirectWarn(ctx rweb.Context, url, fl_msg string) error {
	return RedirectSev(ctx, url, fl_msg, FlashWarn)
}

// RedirectError redirects with an error-level flash message.
func RedirectError(ctx rweb.Context, url, fl_msg string) error {
	return RedirectSev(ctx, url, fl_msg, FlashError)
}

// RedirectSev redirects (303 See Other) after setting a flash message on the
// channel matching the given severity. An empty message sets no flash.
func RedirectSev(ctx rweb.Context, url, fl_msg string, sev FlashSeverity) error {
	if fl_msg != "" {
		fl := flash.NewFlash()
		switch sev {
		case FlashWarn:
			fl.Warn = fl_msg
		case FlashError:
			fl.Error = fl_msg
		default:
			fl.Info = fl_msg
		}
		fl.Set(ctx)
	}
	return ctx.Redirect(303, url) // 303 See Other
}

// IsLoggedIn checks if user is logged in based on RWeb context
func IsLoggedIn(ctx rweb.Context) bool {
	return cctx.IsAdmin(ctx)
}

// VerifyRequestFormToken validates the posted "csrf" form token for state-changing
// actions (deletes and other POSTs that aren't full forms). On failure it
// issues the redirect itself so callers stay one line:
//
//	if ok, err := app.VerifyRequestFormToken(ctx, "/admin/users"); !ok {
//		return err
//	}
//
// The failure is a warn-level flash, not an error page: the common cause is a
// token aged out of the kvstore (1h TTL) on a long-idle admin tab, which a
// page refresh fixes.
func VerifyRequestFormToken(ctx rweb.Context, redirectTo string) (ok bool, err error) {
	if VerifyFormToken(ctx.Request().FormValue("csrf")) {
		return true, nil
	}
	return false, RedirectWarn(ctx, redirectTo,
		"Your page has expired. Please refresh the page and try again.")
}
