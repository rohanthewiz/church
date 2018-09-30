package errormodule

import (
	"github.com/rohanthewiz/church/module"
)

const ModuleTypeError = "error_module"

type ModuleError struct {
	module.Presenter
}

func NewModuleError(opts module.Opts) *ModuleError {
	mod := new(ModuleError)
	mod.Opts = opts
	return mod
}

func (m *ModuleError) Render(params map[string]map[string]string, loggedIn bool) string {
	out := `<div class="ch-module-wrapper ch-` + m.Opts.ModuleType + `"><div class="ch-module-heading">` + "" +
		`</div><div class="ch-module-body">`
	out += `<tr><td colspan="2">` +  m.Opts.Title + `</td></tr>`
	out += "</table></div></div>"
	return out
}
