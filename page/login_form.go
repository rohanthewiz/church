package page

import (
	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/serr"
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
	// Login was the one form posting without a CSRF token (login CSRF lets an
	// attacker sign a victim into an attacker-controlled account); mint one
	// like every other form module does.
	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token.")
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

func (m *ModuleLoginForm) Render(params map[string]map[string]string, loggedIn bool) string {
	const action = "/auth"

	b := element.NewBuilder()
	b.DivClass("wrapper-material-form").R(
		b.H3Class("page-title").T("Login"),
		b.Form("method", "post", "action", action).R(
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),
			b.DivClass("form-group").R(
				// autocomplete tokens let password managers fill the right
				// fields; autofocus saves the first tap on every login
				b.Input("id", "username", "name", "username", "type", "text",
					"required", "required", "autocomplete", "username",
					"autocapitalize", "none", "autofocus", "autofocus").R(), // 'required' also drives the `input:valid` selector
				b.LabelClass("control-label", "for", "username").T("Username"),
				b.IClass("bar").R(),
			),
			b.DivClass("form-group").R(
				b.Input("type", "password", "id", "password", "name", "password",
					"required", "required", "autocomplete", "current-password").R(),
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
