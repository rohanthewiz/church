package user

import (
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/grid"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/authz"
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

// GetData is a module boundary: modules are invoked by the page renderer
// (which knows nothing of databases), so this is where the DB handle is
// fetched and handed to the query layer.
func (m ModuleUsersList) GetData() ([]Presenter, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Could not obtain DB handle")
	}
	return QueryUsers(dbH, m.Opts.Condition, "first_name "+m.Order(), m.Opts.Limit, m.Opts.Offset)
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

	// Roles column and row locking (admin renders only). A row is locked when
	// the viewer couldn't manage that user: a SuperAdmin, or someone holding
	// permissions the viewer lacks (the authz.CanManageUser rule, computed from
	// two whole-table reads rather than per row). A failed read degrades to
	// "no roles shown, rows locked" rather than an empty page.
	actor := authz.FromParams(params)
	var roleNames map[int64][]string
	var userPerms map[int64]authz.Set
	accessLoaded := false
	if m.Opts.IsAdmin {
		if dbH, err := db.Db(); err != nil {
			logger.LogErr(err, "Could not obtain DB handle for user roles")
		} else if roleNames, err = authz.RoleNamesByUser(dbH); err != nil {
			logger.LogErr(err, "Error loading user role names")
		} else if userPerms, err = authz.PermsByUser(dbH); err != nil {
			logger.LogErr(err, "Error loading user permissions")
		} else {
			accessLoaded = true
		}
	}
	manageable := func(usr Presenter, uid int64) bool {
		if actor.IsSuper() {
			return true
		}
		if !accessLoaded || usr.Role == authz.SuperAdminRole {
			return false
		}
		return actor.CanGrant(userPerms[uid])
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
			grid.Column{Header: "Base Role", Popup: true},
			grid.Column{Header: "Roles", Popup: true},
			grid.Column{Header: "Updated By", Popup: true},
			grid.Column{Header: "Actions", NoSort: true, NoFilter: true, Shrink: true}, // edit
			grid.Column{Header: "", NoSort: true, NoFilter: true, Shrink: true},        // delete
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
		// The first name doubles as a link into the editor, so it is gated on
		// exactly what the Actions "Edit" is gated on. It used to be an
		// unconditional link, which handed a read-only role a working-looking
		// way into a form the route then refused — the same leak item 16 closed
		// for the menu list's title link.
		uid, _ := strconv.ParseInt(usr.Id, 10, 64)
		canManage := m.Opts.IsAdmin && manageable(usr, uid)
		mayEdit := m.Opts.IsAdmin && actor.Can(authz.UsersUpdate) && canManage

		nameCell := grid.Text(usr.Firstname)
		if mayEdit {
			nameCell = grid.Link(usr.Firstname, m.GetEditURL()+usr.Id)
		}
		row = append(row,
			grid.Text(enabled),
			nameCell,
		)
		if m.Opts.IsAdmin {
			editCell := grid.EditLinkNamed(m.GetEditURL()+usr.Id, usr.Username)
			if !actor.Can(authz.UsersUpdate) {
				editCell = grid.Text("")
			} else if !canManage {
				editCell = grid.Text("locked")
			}
			deleteCell := grid.Text("")
			if actor.Can(authz.UsersDelete) && canManage {
				deleteCell = grid.DeleteLinkNamed(m.GetDeleteURL()+usr.Id, usr.Username)
			}

			row = append(row,
				grid.Text(usr.Username),
				grid.Text(usr.EmailAddress),
				grid.Text(RoleToString[usr.Role]),
				grid.Text(strings.Join(roleNames[uid], ", ")),
				grid.Text(usr.UpdatedBy),
				editCell,
				deleteCell,
			)
		}
		g.Rows = append(g.Rows, row)
	}

	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading").R(
			b.T(m.Opts.Title),
			b.Wrap(func() {
				if m.Opts.IsAdmin && actor.Can(authz.UsersCreate) {
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
