package page

import (
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/element"
)

const ModuleTypeLoginForm = "login_form"

type ModuleLoginForm struct {
	module.Presenter
	csrf string
}

func NewModuleLoginForm(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleLoginForm)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

func (m *ModuleLoginForm) Render(params map[string]map[string]string, loggedIn bool) string {
	const action = "/auth"

	e := element.New
	out := e("div", "class", "wrapper-material-form").R(
		e("h3", "class", "page-title").R("Login"),
		e("form", "method", "post", "action", action).R(
			e("div", "class", "form-group").R(
				e("input", "id", "username", "name", "username", "type", "text",
					"required", "required").R(),  // we are using 'required' here to drive `input:valid` selector
				e("label", "class", "control-label", "for", "username").R("Username"),
				e("i", "class", "bar").R(),
			),
			e("div", "class", "form-group").R(
				e("input", "type", "password", "id", "password", "name", "password",
					"required", "required").R(),
				e("label", "class", "control-label", "for", "password").R("Password"),
				e("i", "class", "bar").R(),
			),
			e("div", "class", "form-group").R(
				e("input", "type", "submit", "class", "button", "value", "Login").R(),
			),
		),
	)

	return out
}
