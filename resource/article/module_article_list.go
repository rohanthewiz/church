package article

import (
	"strings"

	"github.com/rohanthewiz/church/grid"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
)

const ModuleTypeArticlesList = "articles_list"

type ModuleArticlesList struct {
	module.Presenter
}

func NewModuleArticlesList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleArticlesList)
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

func (m ModuleArticlesList) GetData() ([]Presenter, error) {
	return QueryArticles(m.Opts.Condition, "updated_at "+m.Order(), m.Opts.Limit, m.Opts.Offset)
}

func (m *ModuleArticlesList) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	articles, err := m.GetData()
	if err != nil {
		logger.LogErr(err, "Error obtaining data in module", "module_slug", m.Opts.Slug, "module_type", m.Opts.ModuleType)
		return ""
	}

	// Grid setup. The Summary column holds editor-authored HTML: it renders
	// clamped in the row (no more base64 shuttling through JSON) and the Popup
	// flag lets a click show the full content — same UX as the old AG Grid
	// content renderer + swal combo.
	g := grid.Grid{
		Class:        "articles-list-grid",
		EmptyMessage: "No articles found",
		Limit:        m.Opts.Limit,
		Offset:       m.Opts.Offset,
	}
	if m.Opts.IsAdmin {
		g.Columns = append(g.Columns, grid.Column{Header: "Id", Type: grid.ColNum, Shrink: true})
	}
	g.Columns = append(g.Columns,
		grid.Column{Header: "Title", Width: 210},
		grid.Column{Header: "Summary", Width: 230, Popup: true},
	)
	if m.Opts.IsAdmin {
		g.Columns = append(g.Columns,
			grid.Column{Header: "Slug", Popup: true},
			grid.Column{Header: "Categories", Popup: true},
			grid.Column{Header: "Updated By"},
			grid.Column{Header: "Published"},
			grid.Column{Header: "Edit", NoSort: true, NoFilter: true, Shrink: true},
			grid.Column{Header: "Del", NoSort: true, NoFilter: true, Shrink: true},
		)
	}

	for _, art := range articles {
		published := "draft"
		if art.Published {
			published = "published"
		}

		var row []grid.Cell
		if m.Opts.IsAdmin {
			row = append(row, grid.Text(art.Id))
		}
		row = append(row,
			grid.Link(art.Title, "/"+m.Opts.ItemsURLPath+"/"+art.Id),
			grid.HTML(art.Summary),
		)
		if m.Opts.IsAdmin {
			row = append(row,
				grid.Text(art.Slug),
				grid.Text(strings.Join(art.Categories, ", ")),
				grid.Text(art.UpdatedBy),
				grid.Text(published),
				grid.EditLink(m.GetEditURL()+art.Id),
				grid.DeleteLink(m.GetDeleteURL()+art.Id),
			)
		}
		g.Rows = append(g.Rows, row)
	}

	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading").R(
			b.T(m.Opts.Title),
			b.Wrap(func() {
				if m.Opts.IsAdmin {
					b.A("class", "btn-add", "href", m.GetNewURL(), "title", "Add Article").T("+")
				}
			}),
		),
		b.DivClass("list-wrapper").R(
			g.Render(b),
		),
	)

	return b.String()
}
