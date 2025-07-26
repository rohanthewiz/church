package errormodule

import (
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
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
	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper", "ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading"), // empty heading
		b.DivClass("ch-module-body").R(
			b.Table().R(
				b.Tr().R(
					b.Td("colspan", "2").T(m.Opts.Title),
				),
			),
		),
	)

	return b.String()
}
