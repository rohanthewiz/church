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

// RedirectRWeb handles redirects with an info-level flash message for RWeb.
// Kept for backward compatibility; delegates to RedirectRWebSev.
func RedirectRWeb(ctx rweb.Context, url, fl_msg string) error {
	return RedirectRWebSev(ctx, url, fl_msg, FlashInfo)
}

// RedirectRWebWarn redirects with a warning-level flash message.
func RedirectRWebWarn(ctx rweb.Context, url, fl_msg string) error {
	return RedirectRWebSev(ctx, url, fl_msg, FlashWarn)
}

// RedirectRWebError redirects with an error-level flash message.
func RedirectRWebError(ctx rweb.Context, url, fl_msg string) error {
	return RedirectRWebSev(ctx, url, fl_msg, FlashError)
}

// RedirectRWebSev redirects (303 See Other) after setting a flash message on the
// channel matching the given severity. An empty message sets no flash.
func RedirectRWebSev(ctx rweb.Context, url, fl_msg string, sev FlashSeverity) error {
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
		fl.SetRWeb(ctx)
	}
	return ctx.Redirect(303, url) // 303 See Other
}

// IsLoggedInRWeb checks if user is logged in based on RWeb context
func IsLoggedInRWeb(ctx rweb.Context) bool {
	return cctx.IsAdminFromRWeb(ctx)
}
