package basectlr

import (
	"bytes"

	"github.com/rohanthewiz/church/config"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
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
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{}, IsLoggedInRWeb(ctx))
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
		map[string]map[string]string{pg.MainModuleSlug(): {
			"offset": ctx.Request().QueryParam("offset"), "limit": ctx.Request().QueryParam("limit")},
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
		pg.MainModuleSlug(): {"id": ctx.Request().PathParam("id"), "loggedIn": loggedIn}}, IsLoggedInRWeb(ctx))
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