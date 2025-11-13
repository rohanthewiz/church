package app

import (
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/rweb"
)

// RedirectRWeb handles redirects with flash messages for RWeb
func RedirectRWeb(ctx rweb.Context, url, fl_msg string) error {
	if fl_msg != "" {
		fl := flash.NewFlash()
		fl.Info = fl_msg // todo warn and error
		fl.SetRWeb(ctx)
	}
	return ctx.Redirect(303, url) // 303 See Other
}

// IsLoggedInRWeb checks if user is logged in based on RWeb context
func IsLoggedInRWeb(ctx rweb.Context) bool {
	return cctx.IsAdminFromRWeb(ctx)
}
