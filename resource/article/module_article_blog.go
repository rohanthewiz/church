package article

import (
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/logger"
	"bytes"
	"fmt"
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
	if !mod.Opts.IsAdmin && !mod.Opts.ShowUnpublished{
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
	return QueryArticles(m.Opts.Condition, "updated_at " + m.Order(), m.Opts.Limit, m.Opts.Offset)
}


func (m *ModuleArticlesBlog) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	articles, err := m.getData()
	if err != nil {
		logger.LogErr(err, "Error obtaining data in ArticlesBlog module", "module_options",  fmt.Sprintf("%#v", m.Opts))
		return ""
	}

	out := new(bytes.Buffer)
	ows := out.WriteString // a little abbrev here
	ows(`<div class="ch-module-wrapper ch-`); ows(m.Opts.ModuleType)
	ows(`"><div class="ch-module-heading">`);
	ows(`</div><div class="ch-module-body">`)
	if len(articles) < 1 {
		ows(`<p>No Articles</p>`)
	} else {
		ows(`<div class="title">`); ows(m.Opts.Title); ows(`</div>`)
		for _, art := range articles {
			ows(`<div class="article"><div class="article__title">`)
			ows(`<a href="/`)
			ows(m.Opts.ItemsURLPath + "/" + art.Id); ows(`">`)
			ows(art.Title); ows(`</a></div><div>`)
			ows(art.Summary); ows("</div></div>")
		}
	}
	m.RenderPagination(len(articles))
	ows("</div></div>")

	return out.String()
}
