package authz

import (
	"html"
	"strconv"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// ModuleTypeRoleForm is the create/edit form for one role, hardwired into
// /admin/roles/new and /admin/roles/edit/:id (page.RoleForm).
const ModuleTypeRoleForm = "role_form"

// PermFieldPrefix names each permission checkbox: "perm:articles.publish".
// One field per permission (rather than repeated "perms" values) because
// rweb's FormValue returns only a field's first value; the handler walks the
// catalog and asks for each field by name.
const PermFieldPrefix = "perm:"

type ModuleRoleForm struct {
	module.Presenter
	csrf string
}

func NewModuleRoleForm(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleRoleForm)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token")
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

// matrixColumns are the permission matrix's action columns. Publish and
// enable share the last column: no resource has both, and both mean "make it
// live".
var matrixColumns = []struct {
	header  string
	actions []string
}{
	{"Create", []string{ActCreate}},
	{"Read", []string{ActRead}},
	{"Update", []string{ActUpdate}},
	{"Delete", []string{ActDelete}},
	{"Publish / Enable", []string{ActPublish, ActEnable}},
}

const roleFormCSS = `
.rf-matrix-wrap { overflow-x: auto; }
.rf-matrix { border-collapse: collapse; width: 100%; min-width: 34rem; }
.rf-matrix th, .rf-matrix td { padding: 0.45rem 0.6rem; border-bottom: 1px solid rgba(127,127,127,0.25); text-align: center; }
.rf-matrix th:first-child, .rf-matrix td:first-child { text-align: left; }
.rf-matrix td.rf-na { opacity: 0.4; }
.rf-matrix input[type=checkbox] { width: 1.05rem; height: 1.05rem; }
.rf-matrix input[disabled] { cursor: not-allowed; }
.rf-locked { padding: 0.6rem 0.8rem; border-left: 3px solid #d9534f; margin-bottom: 0.8rem; }
`

// Render shows the role's name, description and permission matrix.
//
// Checkboxes for permissions the viewer doesn't hold are disabled: they
// can't grant those (Actor.CanGrant). If the role already holds a permission
// the viewer lacks, the whole form is locked. Saving it would either revoke a
// permission the viewer never held or leave the role beyond their authority,
// and the handler refuses both. The disabled state is presentation only; the
// handler re-checks everything.
func (m *ModuleRoleForm) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok {
		m.SetId(opts)
	}
	actor := FromParams(params)
	b := element.NewBuilder()

	role := Role{Perms: Set{}}
	operation := "Create"
	action := ""
	if len(m.Opts.ItemIds) > 0 {
		dbH, err := db.Db()
		if err != nil {
			logger.LogErr(err, "Could not obtain DB handle for role form")
			return ""
		}
		r, found, err := GetRole(dbH, m.Opts.ItemIds[0])
		if err != nil {
			logger.LogErr(err, "Error loading role for form")
			return ""
		}
		if !found {
			b.DivClass("af-wrap").R(
				b.H3("class", "af-page-title").T("Role not found"),
				b.AClass("af-btn", "href", "/admin/roles").T("Back to roles"),
			)
			return b.String()
		}
		role = r
		operation = "Update"
		action = "/update/" + strconv.FormatInt(role.ID, 10)
	}
	locked := operation == "Update" && !actor.CanGrant(role.Perms)

	b.DivClass("af-wrap af-wrap--wide").R(
		b.Style().T(roleFormCSS),
		b.H3("class", "af-page-title").T(operation+" Role"),
		b.Form("id", "role_form", "method", "post", "action", "/admin/roles"+action).R(
			b.Input("type", "hidden", "name", "role_id", "value", strconv.FormatInt(role.ID, 10)),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),

			b.Wrap(func() {
				if locked {
					b.DivClass("af-card rf-locked").T("This role holds permissions you don't have, " +
						"so you can't change it. Ask an administrator who holds them.")
				}
			}),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Role"),
				b.DivClass("af-row").R(
					b.DivClass("af-field").R(
						b.Label("for", "role_name").R(
							b.T("Name "), b.SpanClass("af-req").T("*"),
						),
						// element writes attribute values verbatim, so stored text
						// is escaped here.
						b.Input("name", "role_name", "id", "role_name", "type", "text", "required", "required",
							"autocomplete", "off", "maxlength", "80", "value", html.EscapeString(role.Name)),
					),
					b.DivClass("af-field").R(
						b.Label("for", "role_description").T("Description"),
						b.Input("name", "role_description", "id", "role_description", "type", "text",
							"autocomplete", "off", "maxlength", "240", "value", html.EscapeString(role.Description)),
					),
				),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Permissions"),
				b.PClass("af-help").T("Tick any combination. Ticking an action also ticks Read for that "+
					"resource, since Read is what opens its list. Permissions you don't hold are greyed out."),
				b.DivClass("rf-matrix-wrap").R(
					b.Table("class", "rf-matrix").R(
						b.THead().R(
							b.Tr().R(
								b.Th().T("Resource"),
								b.Wrap(func() {
									for _, col := range matrixColumns {
										b.Th().T(col.header)
									}
								}),
								b.Th().T(""),
							),
						),
						b.TBody().R(
							b.Wrap(func() {
								for _, res := range catalog {
									m.renderMatrixRow(b, res, role.Perms, actor, locked)
								}
							}),
						),
					),
				),
			),

			b.DivClass("af-footer").R(
				b.AClass("af-btn", "href", "/admin/roles").T("Cancel"),
				b.Wrap(func() {
					if locked {
						b.Input("type", "submit", "class", "af-submit", "value", operation, "disabled", "disabled")
					} else {
						b.Input("type", "submit", "class", "af-submit", "value", operation)
					}
				}),
			),
		),

		b.Script("type", "text/javascript").T(`(function () {
	var form = document.getElementById('role_form');
	if (!form) { return; }
	// Any non-read action implies read (the server normalizes the same way).
	form.addEventListener('change', function (e) {
		var el = e.target;
		if (!el.dataset || !el.dataset.act || !el.checked || el.dataset.act === 'read') { return; }
		var read = form.querySelector('input[data-res="' + el.dataset.res + '"][data-act="read"]');
		if (read && !read.disabled) { read.checked = true; }
	});
	// Row toggle: all on, or all off when every enabled box is already on.
	Array.prototype.forEach.call(form.querySelectorAll('button[data-all]'), function (btn) {
		btn.addEventListener('click', function () {
			var boxes = form.querySelectorAll('input[data-res="' + btn.dataset.all + '"]:not([disabled])');
			var allOn = Array.prototype.every.call(boxes, function (x) { return x.checked; });
			Array.prototype.forEach.call(boxes, function (x) { x.checked = !allOn; });
		});
	});
})();`),
	)
	return b.String()
}

// renderMatrixRow renders one resource: a checkbox per action it supports,
// a dash for actions it doesn't, and a row toggle.
func (m *ModuleRoleForm) renderMatrixRow(b *element.Builder, res Resource, held Set, actor *Actor, locked bool) {
	supports := map[string]bool{}
	for _, a := range res.Actions {
		supports[a] = true
	}

	b.Tr().R(
		b.Td().T(res.Label),
		b.Wrap(func() {
			for _, col := range matrixColumns {
				action := ""
				for _, a := range col.actions {
					if supports[a] {
						action = a
					}
				}
				if action == "" {
					b.TdClass("rf-na").T("—")
					continue
				}
				p := res.Perm(action)
				attrs := []string{"type", "checkbox", "name", PermFieldPrefix + string(p),
					"data-res", res.Key, "data-act", action,
					"title", string(p), "aria-label", res.Label + " " + action}
				if held.Has(p) {
					attrs = append(attrs, "checked", "checked")
				}
				if locked || !actor.Can(p) {
					attrs = append(attrs, "disabled", "disabled")
				}
				b.Td().R(b.Input(attrs...))
			}
		}),
		b.Td().R(
			b.Wrap(func() {
				if !locked {
					b.Button("type", "button", "class", "af-btn", "data-all", res.Key).T("All")
				}
			}),
		),
	)
}
