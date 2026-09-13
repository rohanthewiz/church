package basectlr

import (
	"bytes"

	"github.com/rohanthewiz/church/config"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/rweb"
)

func RenderPageNewRWeb(pg *page.Page, ctx rweb.Context) (out []byte) {
	defer func() {
		if config.AppEnv != config.Environments.Production {
			return
		}
		if p := recover(); p != nil {
			logPanic(p)
			out = []byte(recoverMsg)
		}
	}()
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		"_global": {"user_agent": ctx.UserAgent(), "username": cctx.GetUsernameFromRWeb(ctx),
			// What the viewer may do, so admin modules offer only permitted actions
			authz.ParamKey: authz.ParamValue(ctx)},
	}, IsLoggedInRWeb(ctx))
	out = buf.Bytes()
	return
}

// RenderPageListWithOptsRWeb renders like RenderPageListRWeb but hands the
// main module its own options instead of offset/limit, e.g. a report's
// {"year": "2025"}. It shares the production panic recovery, which a direct
// template.Page call in a controller would skip.
func RenderPageListWithOptsRWeb(pg *page.Page, ctx rweb.Context, mainOpts map[string]string) (out []byte) {
	defer func() {
		if config.AppEnv != config.Environments.Production {
			return
		}
		if p := recover(); p != nil {
			logPanic(p)
			out = []byte(recoverMsg)
		}
	}()
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx),
		map[string]map[string]string{
			pg.MainModuleSlug(): mainOpts,
			"_global": {"user_agent": ctx.UserAgent(), "username": cctx.GetUsernameFromRWeb(ctx),
				// What the viewer may do, so admin modules offer only permitted actions
				authz.ParamKey: authz.ParamValue(ctx)},
		}, IsLoggedInRWeb(ctx),
	)
	out = buf.Bytes()
	return
}

func RenderPageListRWeb(pg *page.Page, ctx rweb.Context) (out []byte) {
	defer func() {
		if config.AppEnv != config.Environments.Production {
			return
		}
		if p := recover(); p != nil {
			logPanic(p)
			out = []byte(recoverMsg)
		}
	}()
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx),
		map[string]map[string]string{
			pg.MainModuleSlug(): {
				"offset": ctx.Request().QueryParam("offset"), "limit": ctx.Request().QueryParam("limit")},
			"_global": {"user_agent": ctx.UserAgent(), "username": cctx.GetUsernameFromRWeb(ctx),
			// What the viewer may do, so admin modules offer only permitted actions
			authz.ParamKey: authz.ParamValue(ctx)},
		}, IsLoggedInRWeb(ctx),
	)
	out = buf.Bytes()
	return
}

func RenderPageSingleRWeb(pg *page.Page, ctx rweb.Context) (out []byte) {
	defer func() {
		if config.AppEnv != config.Environments.Production {
			return
		} // bypass recovery for non-prod envs
		if p := recover(); p != nil {
			logPanic(p)
			out = []byte(recoverMsg)
		}
	}()
	loggedIn := "no"
	if IsLoggedInRWeb(ctx) {
		loggedIn = "yes"
	}

	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		pg.MainModuleSlug(): {"id": ctx.Request().PathParam("id"), "loggedIn": loggedIn},
		// item_id rides _global (not just the main module's params) so
		// secondary modules can key off the displayed item — the chat
		// discussion strip derives its per-article channel from it.
		// username likewise lets modules tailor controls to the viewer.
		"_global": {"user_agent": ctx.UserAgent(), "item_id": ctx.Request().PathParam("id"),
			"username": cctx.GetUsernameFromRWeb(ctx), authz.ParamKey: authz.ParamValue(ctx)},
	}, IsLoggedInRWeb(ctx))
	out = buf.Bytes()
	return
}

// IsLoggedInRWeb checks if user is logged in based on RWeb context
func IsLoggedInRWeb(ctx rweb.Context) bool {
	// Check if we have a valid session in context
	sess, err := cctx.GetSessionFromRWeb(ctx)
	if err != nil {
		return false
	}
	// Check if session has a username (indicating logged in user)
	return sess != nil && sess.Username != ""
}
