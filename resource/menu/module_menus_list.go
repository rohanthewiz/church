package menu

import (
	"github.com/rohanthewiz/church/app"
	theDB "github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/grid"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const ModuleTypeMenusList = "menus_list"

type ModuleMenusList struct {
	module.Presenter
	csrf string // backs the grid's POSTed delete links (admin renders only)
}

func NewModuleMenusList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleMenusList)
	mod.Name = pres.Name
	mod.Opts = pres.Opts

	// Delete links POST with a CSRF token (same tokens the edit forms use).
	// Only admins see delete links, so public renders skip the kvstore write.
	if mod.Opts.IsAdmin {
		csrf, err := app.GenerateFormToken()
		if err != nil {
			return nil, serr.Wrap(err, "Could not generate form token")
		}
		mod.csrf = csrf
	}

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

func (m ModuleMenusList) GetData() ([]MenuDef, error) {
	dbH, err := theDB.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Could not obtain DB handle")
	}
	return queryMenus(dbH, m.Opts.Condition, "updated_at "+m.Order(), m.Opts.Limit, m.Opts.Offset)
}

func (m *ModuleMenusList) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	mnus, err := m.GetData()
	if err != nil {
		logger.LogErr(err, "Error obtaining data in module", "module_slug", m.Opts.Slug,
			"module_type", m.Opts.ModuleType)
		return ""
	}

	// Grid setup. Admin-only columns are gated like the other list modules
	// (the old grid always defined them but left the cells blank for non-admin).
	g := grid.Grid{
		Class:        "menu-list-grid",
		EmptyMessage: "No menus found",
		Limit:        m.Opts.Limit,
		Offset:       m.Opts.Offset,
		CSRFToken:    m.csrf,
	}
	if m.Opts.IsAdmin {
		g.Columns = append(g.Columns, grid.Column{Header: "Id", Type: grid.ColNum, Shrink: true})
	}
	g.Columns = append(g.Columns,
		grid.Column{Header: "Title"},
		grid.Column{Header: "Slug", Popup: true},
	)
	if m.Opts.IsAdmin {
		g.Columns = append(g.Columns,
			grid.Column{Header: "Published"},
			grid.Column{Header: "Updated By"},
			grid.Column{Header: "", NoSort: true, NoFilter: true, Shrink: true}, // edit
			grid.Column{Header: "", NoSort: true, NoFilter: true, Shrink: true}, // delete
		)
	}

	for _, mnu := range mnus {
		published := "draft"
		if mnu.Published {
			published = "published"
		}

		var row []grid.Cell
		if m.Opts.IsAdmin {
			row = append(row, grid.Text(mnu.Id))
		}
		// Title links to the menu editor (menus have no public single view)
		titleCell := grid.Text(mnu.Title)
		if m.Opts.IsAdmin {
			titleCell = grid.Link(mnu.Title, m.GetEditURL()+mnu.Id)
		}
		row = append(row, titleCell, grid.Text(mnu.Slug))
		if m.Opts.IsAdmin {
			row = append(row,
				grid.Text(published),
				grid.Text(mnu.UpdatedBy),
				grid.EditLink(m.GetEditURL()+mnu.Id),
				grid.DeleteLink(m.GetDeleteURL()+mnu.Id),
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
					b.AClass("btn-add", "href", m.GetNewURL(), "title", "Add Menu").T("+")
				}
			}),
		),
		b.DivClass("list-wrapper").R(
			g.Render(b),
		),
	)

	return b.String()
}
