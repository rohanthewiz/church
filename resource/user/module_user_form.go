package user

import (
	"fmt"
	"html"
	"sort"
	"strconv"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/core/formdraft"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/element"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const ModuleTypeUserForm = "user_form"

type ModuleUserForm struct {
	module.Presenter
	csrf string
}

// User Form deals with only a single item referenced in ItemIds[0] or a new one otherwise
func NewModuleUserForm(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleUserForm)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token.")
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

func (m ModuleUserForm) getData() (pres Presenter, err error) {
	dbH, err := db.Db()
	if err != nil {
		return pres, serr.Wrap(err, "Could not obtain DB handle")
	}
	ser, err := findUserById(dbH, m.Opts.ItemIds[0])
	if err != nil {
		return pres, serr.Wrap(err, "Unable to obtain user", "id", fmt.Sprintf("%d", m.Opts.ItemIds[0]))
	}
	return presenterFromModel(ser), nil
}

func (m *ModuleUserForm) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	usr := Presenter{}
	var err error

	operation := "Create"
	action := ""
	isUpdate := false
	if len(m.Opts.ItemIds) > 0 {
		operation = "Update"
		isUpdate = true
		usr, err = m.getData()
		if err != nil {
			LogErr(err, "Error in module render")
			return ""
		}
		action = "/update/" + usr.Id
	}

	// Role numbers in ascending privilege-number order so the select reads
	// stably (RoleToString map iteration order is random)
	roleNums := make([]int, 0, len(RoleToString))
	for n := range RoleToString {
		roleNums = append(roleNums, n)
	}
	sort.Ints(roleNums)

	// ---- Authorization context for the form ----
	// What the viewer may change here. All of it is re-checked by
	// user_controller.UpsertUserRWeb; the form only avoids offering what
	// would be refused.
	actor := authz.FromParams(params)
	var roles []authz.Role
	assigned := map[int64]bool{}
	manageable := true // may the viewer edit this user at all (authz.CanManageUser)
	if dbH, err := db.Db(); err != nil {
		LogErr(err, "Could not obtain DB handle for user form roles")
	} else {
		if roles, err = authz.ListRoles(dbH); err != nil {
			LogErr(err, "Error listing roles for user form")
		}
		if isUpdate {
			if uid, err := strconv.ParseInt(usr.Id, 10, 64); err == nil {
				ids, err := authz.RoleIDsForUser(dbH, uid)
				if err != nil {
					LogErr(err, "Error loading user roles for form", "user_id", usr.Id)
				}
				for _, id := range ids {
					assigned[id] = true
				}
				if manageable, err = authz.CanManageUser(dbH, actor, uid); err != nil {
					LogErr(err, "Error checking whether viewer can manage user", "user_id", usr.Id)
					manageable = false // fail closed: lock the form
				}
			}
		}
	}
	canEnable := manageable && actor.Can(authz.UsersEnable)

	// A refused save comes back with what was typed (core/formdraft). The
	// draft never holds a password. Role ticks apply only to roles the viewer
	// may grant; locked roles keep showing their stored state, which is also
	// what the save handler does with them.
	var draft FormDraft
	if formdraft.FromParams(params, &draft) && formdraft.SameItem(draft.Id, m.Opts.ItemIds) {
		usr = draft.Presenter
		if draft.RolesPosted && manageable {
			ticked := map[int64]bool{}
			for _, id := range draft.RoleIDs {
				ticked[id] = true
			}
			for _, r := range roles {
				if actor.CanGrant(r.Perms) {
					assigned[r.ID] = ticked[r.ID]
				}
			}
		}
	}

	b := element.NewBuilder()

	b.DivClass("af-wrap").R(
		b.Style().T(userFormCSS),
		b.H3("class", "af-page-title").T(operation+" "+m.Name.Singular),
		b.Wrap(func() {
			if !manageable {
				b.DivClass("af-card uf-locked").T("This person can do things you can't (or is a SuperAdmin), " +
					"so you can't change their account. Ask an administrator who holds those permissions.")
			}
		}),
		b.Form("method", "post", "action",
			"/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "name", "user_id", "value", usr.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Identity"),
				b.DivClass("af-row").R(
					b.DivClass("af-field").R(
						b.Label("for", "firstname").R(
							b.T("First Name "), b.SpanClass("af-req").T("*"),
						),
						b.Input("name", "firstname", "id", "firstname", "type", "text",
							"required", "required", "autocomplete", "off", "value", usr.Firstname),
					),
					b.DivClass("af-field").R(
						b.Label("for", "lastname").T("Last Name"),
						b.Input("name", "lastname", "id", "lastname", "type", "text",
							"autocomplete", "off", "value", usr.Lastname),
					),
				),
				b.DivClass("af-row", "style", "margin-top:0.8rem").R(
					b.DivClass("af-field").R(
						b.Label("for", "username").R(
							b.T("Username "), b.SpanClass("af-req").T("*"),
						),
						// Username is fixed at creation (modelFromPresenter only
						// sets it on create), so don't offer a dead edit on update
						b.Wrap(func() {
							if isUpdate {
								b.Input("name", "username", "id", "username", "type", "text",
									"readonly", "readonly", "value", usr.Username)
							} else {
								b.Input("name", "username", "id", "username", "type", "text",
									"required", "required", "autocomplete", "off", "value", usr.Username)
							}
						}),
						b.Wrap(func() {
							if isUpdate {
								b.PClass("af-help").T("Usernames cannot be changed after creation.")
							}
						}),
					),
					b.DivClass("af-field").R(
						b.Label("for", "email_address").R(
							b.T("Email Address "), b.SpanClass("af-req").T("*"),
						),
						b.Input("name", "email_address", "id", "email_address", "type", "email",
							"required", "required", "autocomplete", "off", "value", usr.EmailAddress),
					),
				),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Access"),
				b.DivClass("af-row").R(
					b.DivClass("af-field").R(
						b.Label("for", "role").T("Base Role"),
						b.Select("name", "role", "id", "role").R(
							b.Wrap(func() {
								for _, n := range roleNums {
									// SuperAdmin bypasses every permission, so only a
									// SuperAdmin may hand it out. It stays listed on a
									// SuperAdmin's own record so the select shows the truth.
									if n == authz.SuperAdminRole && !actor.IsSuper() && usr.Role != n {
										continue
									}
									valStr := strconv.Itoa(n)
									attrs := []string{"value", valStr}
									if n == usr.Role {
										attrs = append(attrs, "selected", "selected")
									}
									b.Option(attrs...).T(RoleToString[n] + " (" + valStr + ")")
								}
							}),
						),
						b.PClass("af-help").T("Drives chat and prayer-wall moderation and the role the mobile app shows. "+
							"Admin screen access comes from Roles below, except SuperAdmin (99), which can do everything."),
					),
					b.DivClass("af-field").R(
						b.Label().T("Status"),
						b.LabelClass("af-switch", "style", "margin-top:0.3rem").R(
							b.Wrap(func() {
								attrs := []string{"type", "checkbox", "name", "enabled"}
								if usr.Enabled {
									attrs = append(attrs, "checked", "checked")
								}
								// Without users.enable the switch is shown but inert; the
								// handler keeps the stored value (authz.ResolveFlag).
								if !canEnable {
									attrs = append(attrs, "disabled", "disabled")
								}
								b.Input(attrs...)
							}),
							b.SpanClass("af-slider").T(""),
							b.SpanClass("af-switch-text").T("User enabled"),
						),
						b.PClass("af-help").T("Disabling a user also signs them out of the mobile app (their API tokens are revoked)."),
						b.Wrap(func() {
							if !canEnable {
								b.PClass("af-help").T("Enabling or disabling users requires the users.enable permission.")
							}
						}),
					),
				),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Roles"),
				b.PClass("af-help").T("Roles decide which admin screens this person can use. They can hold "+
					"several, and get every permission those roles grant. Roles holding permissions you don't "+
					"have are locked."),
				b.Wrap(func() {
					if len(roles) == 0 {
						b.PClass("af-help").T("No roles are defined yet. Create them under Role Management.")
						return
					}
					b.DivClass("uf-roles").R(
						b.Wrap(func() {
							for _, r := range roles {
								field := authz.RoleFieldPrefix + strconv.FormatInt(r.ID, 10)
								attrs := []string{"type", "checkbox", "name", field, "id", field}
								if assigned[r.ID] {
									attrs = append(attrs, "checked", "checked")
								}
								if !manageable || !actor.CanGrant(r.Perms) {
									attrs = append(attrs, "disabled", "disabled")
								}
								b.LabelClass("uf-role", "for", field).R(
									b.Input(attrs...),
									b.Span().R(
										// element writes text verbatim, so stored text is escaped
										b.SpanClass("uf-role__name").T(html.EscapeString(r.Name)),
										b.SpanClass("uf-role__perms").T(html.EscapeString(authz.Summary(r.Perms))),
									),
								)
							}
						}),
					)
				}),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Password"),
				b.DivClass("af-row").R(
					b.DivClass("af-field").R(
						b.Label("for", "password").R(
							b.Wrap(func() {
								if isUpdate {
									b.T("New Password ")
									b.SpanClass("af-opt").T("(leave blank to keep current)")
								} else {
									b.T("Password ")
									b.SpanClass("af-req").T("*")
								}
							}),
						),
						b.Wrap(func() {
							if isUpdate {
								b.Input("name", "password", "id", "password", "type", "password",
									"autocomplete", "new-password", "value", "")
							} else {
								b.Input("name", "password", "id", "password", "type", "password",
									"required", "required", "autocomplete", "new-password", "value", "")
							}
						}),
					),
					b.DivClass("af-field").R(
						b.Label("for", "password_confirm").T("Confirm Password"),
						b.Input("name", "password_confirm", "id", "password_confirm", "type", "password",
							"autocomplete", "new-password", "value", ""),
						b.P("id", "pw_mismatch", "class", "af-help", "style", "display:none;color:#d9534f").
							T("Passwords do not match."),
					),
				),
				b.Wrap(func() {
					if isUpdate {
						b.PClass("af-help").T("Changing the password also signs the user out of the mobile app.")
					}
				}),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Summary"),
				b.DivClass("af-editor bootstrap-wrapper").R(
					b.Div("id", "summer1").T(usr.Summary),
					b.TextArea("id", "user_summary", "name", "user_summary", "type", "text", "value", "",
						"style", "display:none").R(),
				),
			),

			b.DivClass("af-footer").R(
				b.AClass("af-btn", "href", "/admin/"+m.Name.Plural).T("Cancel"),
				b.Wrap(func() {
					if manageable {
						b.Input("type", "submit", "class", "af-submit", "value", operation)
					} else {
						b.Input("type", "submit", "class", "af-submit", "value", operation, "disabled", "disabled")
					}
				}),
			),
		),

		b.Script("type", "text/javascript").T(
			`$(document).ready(function(){$('#summer1').summernote()});
			// Live password-match feedback instead of a submit-time alert()
			(function () {
				var pass = document.getElementById('password');
				var conf = document.getElementById('password_confirm');
				var warn = document.getElementById('pw_mismatch');
				function check() {
					var bad = conf.value !== '' && pass.value !== conf.value;
					warn.style.display = bad ? '' : 'none';
					return !bad && (pass.value === conf.value || conf.value === '');
				}
				pass.addEventListener('input', check);
				conf.addEventListener('input', check);
				window.pwMatches = function () { return pass.value === conf.value; };
			})();
			function preSubmit() {
				if (!window.pwMatches()) {
					document.getElementById('pw_mismatch').style.display = '';
					document.getElementById('password_confirm').focus();
					return false;
				}
				var s1 = $("#summer1");
				var ser_summary = document.getElementById("user_summary");
				if (s1 && ser_summary) {
					ser_summary.innerHTML = s1.summernote('code');
				}
				return true;
			}`),
	)
	return b.String()
}

const userFormCSS = `
.uf-roles { display: grid; gap: 0.55rem; }
.uf-role { display: flex; gap: 0.6rem; align-items: flex-start; cursor: pointer; }
.uf-role input[type=checkbox] { margin-top: 0.2rem; width: 1.05rem; height: 1.05rem; }
.uf-role input[disabled] { cursor: not-allowed; }
.uf-role__name { display: block; font-weight: 600; }
.uf-role__perms { display: block; font-size: 0.85em; opacity: 0.75; }
.uf-locked { padding: 0.6rem 0.8rem; border-left: 3px solid #d9534f; }
`
