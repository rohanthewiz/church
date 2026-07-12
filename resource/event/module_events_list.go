package event

import (
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/grid"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const ModuleTypeEventsList = "events_list"

type ModuleEventsList struct {
	module.Presenter
	csrf string // backs the grid's POSTed delete links (admin renders only)
}

func NewModuleEventsList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleEventsList)
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

func (m ModuleEventsList) getData() ([]Presenter, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Could not obtain DB handle")
	}
	return QueryEvents(dbH, m.Opts.Condition, "event_date "+m.Order(), m.Opts.Limit, m.Opts.Offset)
}

func (m *ModuleEventsList) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	evts, err := m.getData()
	if err != nil {
		Log("Error", "Error obtaining data in module", "module_slug", m.Opts.Slug,
			"module_type", m.Opts.ModuleType, "error", err.Error())
		return ""
	}

	// Grid setup — the event date column powers sorting and year grouping
	g := grid.Grid{
		Class:        "events-list-grid",
		EmptyMessage: "No events found",
		Limit:        m.Opts.Limit,
		Offset:       m.Opts.Offset,
		CSRFToken:    m.csrf,
	}
	if m.Opts.IsAdmin {
		g.Columns = append(g.Columns, grid.Column{Header: "Id", Type: grid.ColNum, Shrink: true})
	}
	g.Columns = append(g.Columns,
		grid.Column{Header: "Event Date", Type: grid.ColDate, Width: 120, GroupBy: true},
		grid.Column{Header: "Title"},
	)
	if m.Opts.IsAdmin {
		g.Columns = append(g.Columns,
			grid.Column{Header: "Slug", Popup: true},
			grid.Column{Header: "Categories", Popup: true},
			grid.Column{Header: "Updated By"},
			grid.Column{Header: "Published"},
			grid.Column{Header: "", NoSort: true, NoFilter: true, Shrink: true}, // edit
			grid.Column{Header: "", NoSort: true, NoFilter: true, Shrink: true}, // delete
		)
	}

	for _, evt := range evts {
		published := "draft"
		if evt.Published {
			published = "published"
		}

		var row []grid.Cell
		if m.Opts.IsAdmin {
			row = append(row, grid.Text(evt.Id))
		}
		row = append(row,
			grid.Text(evt.EventDate),
			grid.Link(evt.Title, "/"+m.Opts.ItemsURLPath+"/"+evt.Id),
		)
		if m.Opts.IsAdmin {
			row = append(row,
				grid.Text(evt.Slug),
				grid.Text(strings.Join(evt.Categories, ", ")),
				grid.Text(evt.UpdatedBy),
				grid.Text(published),
				grid.EditLink(m.GetEditURL()+evt.Id),
				grid.DeleteLink(m.GetDeleteURL()+evt.Id),
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
					b.AClass("btn-add", "href", m.GetNewURL(), "title", "Add Events").T("+")
				}
			}),
		),
		b.DivClass("list-wrapper").R(
			g.Render(b),
		),
	)

	return b.String()
}
