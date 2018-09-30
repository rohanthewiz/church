package article

import (
	"strconv"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/module"
)

const ModuleTypeRecentArticles = "articles_recent"

type ModuleRecentArticles struct {
	module.Presenter
}

func NewModuleRecentArticles(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleRecentArticles)
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

func (m ModuleRecentArticles) GetData() ([]Presenter, error) {
	return QueryArticles(m.Opts.Condition, "updated_at " + m.Order(), m.Opts.Limit, 0)
}

func (m *ModuleRecentArticles) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to us
		if id, ok := opts["limit"]; ok {  // I only ever see us changing the limit
			limit, err := strconv.ParseInt(id, 10, 64)
			if err == nil {
				m.Opts.Limit = limit
			}
		}
	}

	articles, err := m.GetData()

	if err != nil {
		LogErr(err, "Error obtaining data in ModuleRecentArticles")
		return ""
	}
	out := `<div class="ch-module-wrapper ch-` + m.Opts.ModuleType +
		`"><div class="ch-module-heading ch-clickable-heading" onclick="window.location = '/articles'"">` + m.Opts.Title +
		`</div><div class="ch-module-body"><table>`
	if len(articles) < 1 {
		out += `<tr><td colspan="2">No recent articles</td></tr>`
	} else {
		for _, art := range articles {
			out += "<tr>"
			if m.Opts.IsAdmin { out += "<td>" + art.Id + "</td>" }
			out += `<td><a href="/articles/` + art.Id + `">` + art.Title + "</td><td>" + art.UpdatedBy + "</td></tr>"
		}
	}
	out += "</table></div></div>"
	return out
}
