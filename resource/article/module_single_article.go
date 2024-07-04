package article

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const ModuleTypeSingleArticle = "article_single"

type ModuleSingleArticle struct {
	module.Presenter
}

func NewModuleSingleArticle(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleSingleArticle)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

func (m ModuleSingleArticle) getData() (pres Presenter, err error) {
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

func (m *ModuleSingleArticle) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us (there may be none)
		m.SetId(opts)
	}
	art, err := m.getData()
	if err != nil {
		LogErr(err, "Error in module single article render")
		return ""
	}
	klass := "ch-module-wrapper ch-" + m.Opts.ModuleType
	if m.Opts.CustomClass != "" {
		klass += " " + m.Opts.CustomClass
	}
	b := element.NewBuilder()
	e := b.E
	e("div", "class", klass).R(
		e("h3", "class", "article-title").R(art.Title),
		e("p").R(art.Summary),
		e("p").R(art.Body),
		func() (str string) {
			// if len(art.Categories) > 0 {
			//	str = e("div", "class", "categories").R(strings.Join(art.Categories, ", "))
			// }
			if loggedIn && len(m.Opts.ItemIds) > 0 {
				e("a", "class", "edit-link", "href", m.GetEditURL()+
					strconv.FormatInt(m.Opts.ItemIds[0], 10)).R(
					e("img", "class", "edit-icon", "title", "Edit Article", "src", "/assets/images/edit_article.svg").R(),
				)
			}
			return
		}(),
	)
	return b.S()
}
