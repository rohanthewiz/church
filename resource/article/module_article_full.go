package article

import (
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/module"
	"strings"
	"github.com/rohanthewiz/serr"
	"fmt"
	"errors"
)

// This module should not be used for now
const ModuleTypeArticleFull = "article_full"

type ModuleArticleFull struct {
	module.Presenter
}

func NewModuleArticleFull(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleArticleFull)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

func (m ModuleArticleFull) getData() (pres Presenter, err error) {
	// If the module instance has an item slug defined, it takes highest precedence
	if m.Opts.ItemSlug != "" {
		pres, err = presenterFromSlug(m.Opts.ItemSlug)
	} else {
		if len(m.Opts.ItemIds) < 1 {
			return pres, serr.Wrap(errors.New("No item ids found"),
				"module_options", fmt.Sprintf("%#v", m.Opts))
		}
		pres, err = presenterFromId(m.Opts.ItemIds[0])  // Todo presenterFromId for other resources
	}
	return
}

func (m *ModuleArticleFull) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to us
		m.SetId(opts)
	}

	art, err := m.getData()
	if err != nil {
		LogErr(err, "Error rendering module", "module_type", m.Opts.ModuleType)
		return ""
	}
	out := ""
	if art.Published {
		out = "<h3>" + art.Title + "</h3><div>" + art.Summary + "</div><div>" + art.Body + "</div>"
		if len(art.Categories) > 0 {
			out += `<div class="categories">` + strings.Join(art.Categories, ", ") + "</div>"
		}
	}
	return out
}
