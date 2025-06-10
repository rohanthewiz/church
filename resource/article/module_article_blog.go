package article

import (
	"fmt"

	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
)

const ModuleTypeArticlesBlog = "articles_blog"

type ModuleArticlesBlog struct {
	module.Presenter
}

func NewModuleArticlesBlog(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleArticlesBlog)
	mod.Name = pres.Name
	mod.Opts = pres.Opts

	// Work out local condition
	cond := "1 = 1"
	if !mod.Opts.IsAdmin && !mod.Opts.ShowUnpublished {
		cond = "published = true"
	}
	// merge with any incoming condition
	if mod.Opts.Condition != "" {
		cond = mod.Opts.Condition + " AND " + cond
	}
	mod.Opts.Condition = cond

	return module.Module(mod), nil
}

// Opts.ItemIds take precedence over other parameters
func (m ModuleArticlesBlog) getData() ([]Presenter, error) {
	if len(m.Opts.ItemIds) > 0 {
		//fmt.Println("*|* About to run PresentersFromIds", "m.Opts.ItemIds", m.Opts.ItemIds)
		return PresentersFromIds(m.Opts.ItemIds)
	}
	return QueryArticles(m.Opts.Condition, "updated_at "+m.Order(), m.Opts.Limit, m.Opts.Offset)
}

func (m *ModuleArticlesBlog) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	articles, err := m.getData()
	if err != nil {
		logger.LogErr(err, "Error obtaining data in ArticlesBlog module", "module_options", fmt.Sprintf("%#v", m.Opts))
		return ""
	}

	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading").R(),
		b.DivClass("ch-module-body").R(
			b.Wrap(func() {
				if len(articles) < 1 {
					b.P().T("No Articles")
				} else {
					b.DivClass("title").T(m.Opts.Title)
					for _, art := range articles {
						b.DivClass("article").R(
							b.DivClass("article__title").R(
								b.A("href", "/"+m.Opts.ItemsURLPath+"/"+art.Id).T(art.Title),
							),
							b.Div().T(art.Summary),
						)
					}
				}
				m.RenderPagination(len(articles))
			}),
		),
	)

	return b.String()
}
