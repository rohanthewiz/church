package user

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/module"
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

	b := element.NewBuilder()

	b.DivClass("af-wrap").R(
		b.H3("class", "af-page-title").T(operation+" "+m.Name.Singular),
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
						b.Label("for", "role").T("Role"),
						b.Select("name", "role", "id", "role").R(
							b.Wrap(func() {
								for _, n := range roleNums {
									valStr := strconv.Itoa(n)
									attrs := []string{"value", valStr}
									if n == usr.Role {
										attrs = append(attrs, "selected", "selected")
									}
									b.Option(attrs...).T(RoleToString[n] + " (" + valStr + ")")
								}
							}),
						),
						b.PClass("af-help").T("Lower numbers carry more privilege; SuperAdmin (99) is the exception."),
					),
					b.DivClass("af-field").R(
						b.Label().T("Status"),
						b.LabelClass("af-switch", "style", "margin-top:0.3rem").R(
							b.Wrap(func() {
								if usr.Enabled {
									b.Input("type", "checkbox", "name", "enabled", "checked", "checked")
								} else {
									b.Input("type", "checkbox", "name", "enabled")
								}
							}),
							b.SpanClass("af-slider").T(""),
							b.SpanClass("af-switch-text").T("User enabled"),
						),
						b.PClass("af-help").T("Disabling a user also signs them out of the mobile app (their API tokens are revoked)."),
					),
				),
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
				b.Input("type", "submit", "class", "af-submit", "value", operation),
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
