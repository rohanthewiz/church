package page

import (
	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/grid"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const ModuleTypePagesList = "pages_list"

type ModulePagesList struct {
	module.Presenter
	csrf string // backs the grid's POSTed delete links (admin renders only)
}

func NewModulePagesList(pres module.Presenter) (module.Module, error) {
	mod := new(ModulePagesList)
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

func (m ModulePagesList) GetData() ([]Presenter, error) {
	return queryPages(m.Opts.Condition, "updated_at "+m.Order(), m.Opts.Limit, m.Opts.Offset)
}

func (m *ModulePagesList) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	pgs, err := m.GetData()
	if err != nil {
		LogErr(err, "Error obtaining data in module", "module_slug", m.Opts.Slug,
			"module_type", m.Opts.ModuleType)
		return ""
	}

	// Grid setup — the pages list is admin-only, so every column is present.
	// Page URL renders as a real link to the public page (the old grid showed
	// it as text with a click-popup).
	g := grid.Grid{
		Class:        "list-grid",
		EmptyMessage: "No pages found",
		Limit:        m.Opts.Limit,
		Offset:       m.Opts.Offset,
		CSRFToken:    m.csrf,
	}
	g.Columns = []grid.Column{
		{Header: "Id", Type: grid.ColNum, Shrink: true},
		{Header: "Title"},
		{Header: "Page URL"},
		{Header: "Published"},
		{Header: "Updated By"},
		{Header: "", NoSort: true, NoFilter: true, Shrink: true}, // edit
		{Header: "", NoSort: true, NoFilter: true, Shrink: true}, // delete
	}

	for _, pg := range pgs {
		published := "draft"
		if pg.Published {
			published = "published"
		}

		g.Rows = append(g.Rows, []grid.Cell{
			grid.Text(pg.Id),
			grid.Link(pg.Title, "/admin/"+m.Opts.ItemsURLPath+"/"+pg.Id),
			grid.Link("/pages/"+pg.Slug, "/pages/"+pg.Slug),
			grid.Text(published),
			grid.Text(pg.UpdatedBy),
			grid.EditLink(m.GetEditURL() + pg.Id),
			grid.DeleteLink(m.GetDeleteURL() + pg.Id),
		})
	}

	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading").R(
			b.T(m.Opts.Title),
			b.Wrap(func() {
				if m.Opts.IsAdmin {
					b.A("class", "btn-add", "href", m.GetNewURL(), "title", "Add Page").T("+")
				}
			}),
		),
		b.DivClass("list-wrapper").R(
			g.Render(b),
		),
	)

	return b.String()
}
