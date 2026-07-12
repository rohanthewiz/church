package sermon

import (
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/grid"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const ModuleTypeSermonsList = "sermons_list"

type ModuleSermonsList struct {
	module.Presenter
	csrf string // backs the grid's POSTed delete links (admin renders only)
}

func NewModuleSermonsList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleSermonsList)
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

func (m ModuleSermonsList) GetData() ([]Presenter, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Could not obtain DB handle")
	}
	return QuerySermons(dbH, m.Opts.Condition, "date_taught "+m.Order(), m.Opts.Limit, m.Opts.Offset)
}

func (m *ModuleSermonsList) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	sermons, err := m.GetData()
	if err != nil {
		logger.LogErr(err, "Error obtaining data in module", "module_slug", m.Opts.Slug,
			"module_type", m.Opts.ModuleType)
		return ""
	}
	if len(sermons) == 0 {
		logger.Log("Warn", "No sermons found")
	} else {
		logger.Log("Info", strconv.Itoa(len(sermons))+" sermon(s) found")
	}

	// Grid setup — columns mirror the former AG Grid defs; the date column
	// drives the year-grouping toggle. Column and row ordering must agree.
	g := grid.Grid{
		Class:        "sermons-list-grid",
		EmptyMessage: "No sermons found",
		Limit:        m.Opts.Limit,
		Offset:       m.Opts.Offset,
		CSRFToken:    m.csrf,
	}
	if m.Opts.IsAdmin {
		g.Columns = append(g.Columns, grid.Column{Header: "Id", Type: grid.ColNum, Shrink: true})
	}
	g.Columns = append(g.Columns,
		grid.Column{Header: "Date Preached", Type: grid.ColDate, Width: 120, GroupBy: true},
		grid.Column{Header: "Title"},
		grid.Column{Header: "Scripture Refs."},
		grid.Column{Header: "Categories", Popup: true},
	)
	if m.Opts.IsAdmin {
		g.Columns = append(g.Columns,
			grid.Column{Header: "Slug", Popup: true},
			grid.Column{Header: "Updated By"},
			grid.Column{Header: "Published"},
			grid.Column{Header: "", NoSort: true, NoFilter: true, Shrink: true}, // edit
			grid.Column{Header: "", NoSort: true, NoFilter: true, Shrink: true}, // delete
		)
	}

	for _, ser := range sermons {
		published := "draft"
		if ser.Published {
			published = "published"
		}

		var row []grid.Cell
		if m.Opts.IsAdmin {
			row = append(row, grid.Text(ser.Id))
		}
		row = append(row,
			grid.Text(ser.DateTaught),
			grid.Link(ser.Title, "/"+m.Opts.ItemsURLPath+"/"+ser.Id),
			grid.Text(strings.Join(ser.ScriptureRefs, ", ")),
			grid.Text(strings.Join(ser.Categories, ", ")),
		)
		if m.Opts.IsAdmin {
			row = append(row,
				grid.Text(ser.Slug),
				grid.Text(ser.UpdatedBy),
				grid.Text(published),
				grid.EditLink(m.GetEditURL()+ser.Id),
				grid.DeleteLink(m.GetDeleteURL()+ser.Id),
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
					b.A("class", "btn-add", "href", m.GetNewURL(), "title", "Add Sermon").T("+")
				}
			}),
		),
		b.DivClass("ch-sermons-list-wrapper").R(
			g.Render(b),
		),
	)

	return b.String()
}
