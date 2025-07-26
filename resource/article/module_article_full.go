package article

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
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
		pres, err = presenterFromId(m.Opts.ItemIds[0]) // Todo presenterFromId for other resources
	}
	return
}

func (m *ModuleArticleFull) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}

	art, err := m.getData()
	if err != nil {
		LogErr(err, "Error rendering module", "module_type", m.Opts.ModuleType)
		return ""
	}

	b := element.NewBuilder()

	b.Wrap(func() {
		if art.Published {
			b.H3().T(art.Title)
			b.Div().T(art.Summary)
			b.Div().T(art.Body)
			if len(art.Categories) > 0 {
				b.DivClass("categories").T(strings.Join(art.Categories, ", "))
			}
		}
	})

	return b.String()
}
