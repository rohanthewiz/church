package page

import (
	"github.com/rohanthewiz/church/module"
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

	b := element.NewBuilder()
	b.DivClass("wrapper-material-form").R(
		b.H3Class("page-title").T("Login"),
		b.Form("method", "post", "action", action).R(
			b.DivClass("form-group").R(
				b.Input("id", "username", "name", "username", "type", "text",
					"required", "required").R(), // we are using 'required' here to drive `input:valid` selector
				b.LabelClass("control-label", "for", "username").T("Username"),
				b.IClass("bar").R(),
			),
			b.DivClass("form-group").R(
				b.Input("type", "password", "id", "password", "name", "password",
					"required", "required").R(),
				b.LabelClass("control-label", "for", "password").T("Password"),
				b.IClass("bar").R(),
			),
			b.DivClass("form-group").R(
				b.Input("type", "submit", "class", "button", "value", "Login").R(),
			),
		),
	)

	return b.String()
}
