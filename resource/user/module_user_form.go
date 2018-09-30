package user

import (
	"fmt"
	"github.com/rohanthewiz/serr"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/app"
	"strconv"
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
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to us
		m.SetId(opts)
	}
	usr := Presenter{}; var err error

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

	e := element.New

	elEnabled := e("input", "type", "checkbox", "class", "enabled", "name", "enabled")
	if usr.Enabled {
		elEnabled.AddAttributes("checked", "checked")
	}

	out := e("div", "class", "wrapper-material-form").R(
		e("h3", "class", "page-title").R(operation + " " + m.Name.Singular),
		e("form", "method", "post", "action",
				"/admin/" + m.Name.Plural + action, "onSubmit", "return preSubmit();").R(
			e("input", "type", "hidden", "name", "user_id", "value", usr.Id).R(),
			e("input", "type", "hidden", "name", "csrf", "value", m.csrf).R(),
			e("div", "class", "form-inline").R(
				e("div", "class", "form-group").R(
					e("input", "class", "firstname", "name", "firstname", "type", "text",
						"required", "required", "value", usr.Firstname).R(),  // we are using 'required' here to drive `input:valid` selector
					e("label", "class", "control-label", "for", "firstname").R("Firstname"),
					e("i", "class", "bar").R(),
				),
				e("div", "class", "form-group").R(
					e("input", "class", "lastname", "name", "lastname", "type", "text",
						"required", "required", "value", usr.Lastname).R(),
					e("label", "class", "control-label", "for", "lastname").R("Last Name"),
					e("i", "class", "bar").R(),
				),
			),
			e("div", "class", "form-inline").R(
				e("div", "class", "form-group").R(
					e("input", "class", "username", "name", "username", "type", "text",
						"required", "required", "value", usr.Username).R(),
					e("label", "class", "control-label", "for", "username").R("Username"),
					e("i", "class", "bar").R(),
				),
				e("div", "class", "form-group").R(
					e("input", "class", "email_address", "name", "email_address", "type", "text",
						"required", "required", "value", usr.EmailAddress).R(),
					e("label", "class", "control-label", "for", "email_address").R("Email Address"),
					e("i", "class", "bar").R(),
				),
			),
			e("div", "class", "form-inline").R(
				e("div", "class", "form-group").R(
					e("input", "class", "role", "name", "role", "type", "text",
						"required", "required", "value", strconv.Itoa(usr.Role)).R(),
					e("label", "class", "control-label", "for", "role").R("Role (1 - admin, 5 - publisher, 7 - editor, 9 - registered_user)"),
					e("i", "class", "bar").R(),
				),
				e("div", "class", "checkbox").R(
					e("label").R(
						elEnabled.R(),
						e("i", "class", "helper").R(),
						"User enabled",
					),
					e("i", "class", "bar").R(),
				),
			),
			e("div", "class", "form-group").R(
				e("input", "class", "password", "name", "password", "type", "password", "value", "").R(),
				e("label", "class", "control-label", "for", "password").R("Password for new user or password change"),
				e("i", "class", "bar").R(),
			),
			e("div", "class", "form-group").R(
				e("input", "class", "password_confirm", "name", "password_confirm", "type", "password",
					"value", "").R(),
				e("label", "class", "control-label", "for", "password_confirm").R("Password Confirmation"),
				e("i", "class", "bar").R(),
			),
			e("div", "class", "form-group bootstrap-wrapper").R(
				e("div", "id", "summer1").R(usr.Summary),
				e("textarea", "id", "user_summary", "name", "user_summary", "type", "text", "value", "",
					"style", "display:none").R(),
				e("label", "class", "control-label", "for", "user_summary").R("Summary"),
			),

			e("div", "class", "form-group").R(
				e("input", "type", "submit", "class", "button", "value", operation).R(),
			),
		),

		e("script", "type", "text/javascript").R(
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
			}
		`),
	)
	return out
}