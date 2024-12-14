package basectlr

import (
	"bytes"
	"fmt"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const recoverMsg = "Oops, we encountered a server error. Try refreshing the page."

func RenderPageNew(pg *page.Page, c echo.Context) (out []byte) {
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
	template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{}, app.IsLoggedIn(c))
	out = buf.Bytes()
	return
}

func RenderPageList(pg *page.Page, c echo.Context) (out []byte) {
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
	template.Page(buf, pg, flash.GetOrNew(c),
		map[string]map[string]string{pg.MainModuleSlug(): {
			"offset": c.QueryParam("offset"), "limit": c.QueryParam("limit")},
		}, app.IsLoggedIn(c),
	)
	out = buf.Bytes()
	return
}

func RenderPageSingle(pg *page.Page, c echo.Context) (out []byte) {
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
	if app.IsLoggedIn(c) {
		loggedIn = "yes"
	}

	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{
		pg.MainModuleSlug(): {"id": c.Param("id"), "loggedIn": loggedIn}}, app.IsLoggedIn(c))
	out = buf.Bytes()
	return
}

func logPanic(p interface{}) {
	logger.LogErr(serr.NewSErr("Panic occurred", "panic", fmt.Sprintf("%v", p)))
}
