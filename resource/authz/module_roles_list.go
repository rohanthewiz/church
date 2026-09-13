package authz

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

// ModuleTypeRolesList is the Role Management list, hardwired into
// /admin/roles (page.RolesList). Never offered on dynamic pages.
const ModuleTypeRolesList = "roles_list"

type ModuleRolesList struct {
	module.Presenter
	csrf string // backs the grid's POSTed delete links
}

func NewModuleRolesList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleRolesList)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token")
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

// Render lists every role. There is no server paging: a site has a handful
// of roles, and ListRoles reads them whole anyway (see queries.go).
//
// Edit and delete actions are shown only when the viewer could carry them
// out: they hold the permission, and the role's permissions are a subset of
// their own (the no-escalation rule the handlers enforce).
func (m *ModuleRolesList) Render(params map[string]map[string]string, loggedIn bool) string {
	actor := FromParams(params)
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle for roles list")
		return ""
	}
	roles, err := ListRoles(dbH)
	if err != nil {
		logger.LogErr(err, "Error listing roles")
		return ""
	}

	g := grid.Grid{
		Class:        "roles-list-grid",
		EmptyMessage: "No roles yet",
		CSRFToken:    m.csrf,
		Columns: []grid.Column{
			{Header: "Name"},
			{Header: "Description", Popup: true},
			{Header: "Permissions", Popup: true},
			{Header: "Users", Type: grid.ColNum, Shrink: true},
			{Header: "Actions", NoSort: true, NoFilter: true, Shrink: true},
			{Header: "", NoSort: true, NoFilter: true, Shrink: true},
		},
	}

	for _, r := range roles {
		id := strconv.FormatInt(r.ID, 10)
		grantable := actor.CanGrant(r.Perms)

		nameCell := grid.Text(r.Name)
		editCell := grid.Text("")
		if actor.Can(RolesUpdate) && grantable {
			nameCell = grid.Link(r.Name, m.GetEditURL()+id)
			editCell = grid.EditLinkNamed(m.GetEditURL()+id, r.Name)
		} else if actor.Can(RolesUpdate) {
			// Say why rather than silently omitting the action.
			editCell = grid.Text("locked")
		}
		deleteCell := grid.Text("")
		if actor.Can(RolesDelete) && grantable {
			deleteCell = grid.DeleteLinkNamed(m.GetDeleteURL()+id, r.Name)
		}

		g.Rows = append(g.Rows, []grid.Cell{
			nameCell,
			grid.Text(r.Description),
			{Text: Summary(r.Perms), SortVal: strconv.Itoa(len(r.Perms))},
			grid.Text(strconv.Itoa(r.UserCount)),
			editCell,
			deleteCell,
		})
	}

	b := element.NewBuilder()
	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading").R(
			b.T(m.Opts.Title),
			b.Wrap(func() {
				if actor.Can(RolesCreate) {
					b.A("class", "btn-add", "href", m.GetNewURL(), "title", "Add Role").T("+")
				}
			}),
		),
		b.PClass("af-help").T("A role is any combination of permissions. Assign roles to people on the "+
			"Users screen; someone holding several roles gets all of their permissions. "+
			"\"locked\" marks a role holding permissions you don't have, which you can't change."),
		b.DivClass("list-wrapper").R(
			g.Render(b),
		),
	)
	return b.String()
}

// Summary renders a permission set compactly by resource, in catalog order,
// e.g. "Articles: create read update · Events: read". The whole catalog
// collapses to "Everything".
func Summary(perms Set) string {
	if len(perms) == 0 {
		return "None"
	}
	if AllPermissions().SubsetOf(perms) {
		return "Everything"
	}
	var parts []string
	for _, r := range catalog {
		var acts []string
		for _, a := range r.Actions {
			if perms.Has(r.Perm(a)) {
				acts = append(acts, a)
			}
		}
		if len(acts) > 0 {
			parts = append(parts, r.Label+": "+strings.Join(acts, " "))
		}
	}
	return strings.Join(parts, " · ")
}
