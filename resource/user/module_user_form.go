package user

import (
	"fmt"
	"strconv"

	"github.com/rohanthewiz/church/app"
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
	ser, err := findUserById(m.Opts.ItemIds[0])
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
	if len(m.Opts.ItemIds) > 0 {
		operation = "Update"
		usr, err = m.getData()
		if err != nil {
			LogErr(err, "Error in module render")
			return ""
		}
		action = "/update/" + usr.Id
	}

	b := element.NewBuilder()

	b.DivClass("wrapper-material-form").R(
		b.H3("class", "page-title").T(operation+" "+m.Name.Singular),
		b.Form("method", "post", "action",
			"/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "name", "user_id", "value", usr.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),
			b.DivClass("form-inline").R(
				b.DivClass("form-group").R(
					b.Input("class", "firstname", "name", "firstname", "type", "text",
						"required", "required", "value", usr.Firstname), // we are using 'required' here to drive `input:valid` selector
					b.Label("class", "control-label", "for", "firstname").T("Firstname"),
					b.IClass("bar"),
				),
				b.DivClass("form-group").R(
					b.Input("class", "lastname", "name", "lastname", "type", "text",
						"required", "required", "value", usr.Lastname),
					b.Label("class", "control-label", "for", "lastname").T("Last Name"),
					b.IClass("bar"),
				),
			),
			b.DivClass("form-inline").R(
				b.DivClass("form-group").R(
					b.Input("class", "username", "name", "username", "type", "text",
						"required", "required", "value", usr.Username),
					b.Label("class", "control-label", "for", "username").T("Username"),
					b.IClass("bar"),
				),
				b.DivClass("form-group").R(
					b.Input("class", "email_address", "name", "email_address", "type", "text",
						"required", "required", "value", usr.EmailAddress),
					b.Label("class", "control-label", "for", "email_address").T("Email Address"),
					b.IClass("bar"),
				),
			),
			b.DivClass("form-inline").R(
				b.DivClass("form-group").R(
					b.Input("class", "role", "name", "role", "type", "text",
						"required", "required", "value", strconv.Itoa(usr.Role)),
					b.Label("class", "control-label", "for", "role").T("Role (1 - admin, 5 - publisher, 7 - editor, 9 - registered_user)"),
					b.IClass("bar"),
				),
				b.DivClass("checkbox").R(
					b.Label().R(
						b.Wrap(func() {
							if usr.Enabled {
								b.Input("type", "checkbox", "class", "enabled", "name", "enabled", "checked", "checked")
							} else {
								b.Input("type", "checkbox", "class", "enabled", "name", "enabled")
							}
						}),
						b.IClass("helper"),
						b.Text("User enabled"),
					),
					b.IClass("bar"),
				),
			),
			b.DivClass("form-group").R(
				b.Input("class", "password", "name", "password", "type", "password", "value", ""),
				b.Label("class", "control-label", "for", "password").T("Password for new user or password change"),
				b.IClass("bar"),
			),
			b.DivClass("form-group").R(
				b.Input("class", "password_confirm", "name", "password_confirm", "type", "password",
					"value", ""),
				b.Label("class", "control-label", "for", "password_confirm").T("Password Confirmation"),
				b.IClass("bar"),
			),
			b.DivClass("form-group bootstrap-wrapper").R(
				b.Div("id", "summer1").T(usr.Summary),
				b.TextArea("id", "user_summary", "name", "user_summary", "type", "text", "value", "",
					"style", "display:none"),
				b.Label("class", "control-label", "for", "user_summary").T("Summary"),
			),

			b.DivClass("form-group").R(
				b.Input("type", "submit", "class", "button", "value", operation),
			),
		),

		b.Script("type", "text/javascript").T(
			`$(document).ready(function(){$('#summer1').summernote()});
			function preSubmit() {
				var pass = document.querySelector('.password');
				var conf = document.querySelector('.password_confirm');
				if (pass.value !== conf.value) {
					alert("Passwords do not match. Please try again.");
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
