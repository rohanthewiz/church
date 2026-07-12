package user

import (
	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/grid"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const ModuleTypeUsersList = "users_list"

type ModuleUsersList struct {
	module.Presenter
	csrf string // backs the grid's POSTed delete links (admin renders only)
}

func NewModuleUsersList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleUsersList)
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
		cond = "enabled = true"
	}
	// merge with any incoming condition
	if mod.Opts.Condition != "" {
		cond = mod.Opts.Condition + " AND " + cond
	}
	mod.Opts.Condition = cond

	return module.Module(mod), nil
}

func (m ModuleUsersList) GetData() ([]Presenter, error) {
	return QueryUsers(m.Opts.Condition, "first_name "+m.Order(), m.Opts.Limit, m.Opts.Offset)
}

func (m *ModuleUsersList) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to this module
		m.SetLimitAndOffset(opts)
	}
	users, err := m.GetData()
	if err != nil {
		logger.LogErr(err, "Error obtaining data in module", "module_slug", m.Opts.Slug,
			"module_type", m.Opts.ModuleType)
		return ""
	}

	// Grid setup. The old grid defined Enabled twice for admins — collapsed
	// to a single column here. Editing is reached via the First Name link,
	// so there is no separate edit column (matching the old behavior).
	g := grid.Grid{
		Class:        "users-list-grid",
		EmptyMessage: "No users found",
		Limit:        m.Opts.Limit,
		Offset:       m.Opts.Offset,
		CSRFToken:    m.csrf,
	}
	if m.Opts.IsAdmin {
		g.Columns = append(g.Columns, grid.Column{Header: "Id", Type: grid.ColNum, Shrink: true})
	}
	g.Columns = append(g.Columns,
		grid.Column{Header: "Enabled"},
		grid.Column{Header: "First Name"},
	)
	if m.Opts.IsAdmin {
		g.Columns = append(g.Columns,
			grid.Column{Header: "Username", Popup: true},
			grid.Column{Header: "Email Address", Popup: true},
			grid.Column{Header: "Role", Popup: true},
			grid.Column{Header: "Updated By", Popup: true},
			grid.Column{Header: "", NoSort: true, NoFilter: true, Shrink: true}, // delete
		)
	}

	for _, usr := range users {
		enabled := "disabled"
		if usr.Enabled {
			enabled = "enabled"
		}

		var row []grid.Cell
		if m.Opts.IsAdmin {
			row = append(row, grid.Text(usr.Id))
		}
		row = append(row,
			grid.Text(enabled),
			grid.Link(usr.Firstname, m.GetEditURL()+usr.Id),
		)
		if m.Opts.IsAdmin {
			row = append(row,
				grid.Text(usr.Username),
				grid.Text(usr.EmailAddress),
				grid.Text(RoleToString[usr.Role]),
				grid.Text(usr.UpdatedBy),
				grid.DeleteLink(m.GetDeleteURL()+usr.Id),
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
					b.A("class", "btn-add", "href", m.GetNewURL(), "title", "Add User").T("+")
				}
			}),
		),
		b.DivClass("list-wrapper").R(
			g.Render(b),
		),
	)

	return b.String()
}
