package article

import (
	"strconv"

	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
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

func (m ModuleRecentArticles) GetData() ([]Presenter, error) {
	return QueryArticles(m.Opts.Condition, "updated_at "+m.Order(), m.Opts.Limit, 0)
}

func (m *ModuleRecentArticles) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		if id, ok := opts["limit"]; ok { // I only ever see us changing the limit
			limit, err := strconv.ParseInt(id, 10, 64)
			if err == nil {
				m.Opts.Limit = limit
			}
		}
	}

	articles, err := m.GetData()

	if err != nil {
		logger.LogErr(err, "Error obtaining data in ModuleRecentArticles")
		return ""
	}

	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper ch-" + m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading ch-clickable-heading", "onclick", "window.location = '/articles'").T(m.Opts.Title),
		b.DivClass("ch-module-body").R(
			b.Table().R(
				b.Wrap(func() {
					if len(articles) < 1 {
						b.Tr().R(
							b.Td("colspan", "2").T("No recent articles"),
						)
					} else {
						for _, art := range articles {
							b.Tr().R(
								b.Wrap(func() {
									if m.Opts.IsAdmin {
										b.Td().T(art.Id)
									}
								}),
								b.Td().R(
									b.A("href", "/articles/"+art.Id).T(art.Title),
								),
								b.Td().T(art.UpdatedBy),
							)
						}
					}
				},
				),
			),
		),
	)

	return b.String()
}
