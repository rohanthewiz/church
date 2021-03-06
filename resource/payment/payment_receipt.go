package payment

import (
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
)

const ModuleTypePaymentReceipt = "payment_receipt"

type ModulePaymentReceipt struct {
	module.Presenter
}

func NewModulePaymentReceipt(pres module.Presenter) (module.Module, error) {
	mod := new(ModulePaymentReceipt)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return mod, nil
}

func (m ModulePaymentReceipt) Render(params map[string]map[string]string, loggerIn bool) (out string) {
	e := element.New
	out = e("div", "class", "ch-module-wrapper ch-" + m.Opts.ModuleType).R(
		e("h3", "class", "article-title").R("Thanks for your donation!"),
		e("p", "class", "receipt-info").R(
			"Your receipt is available",
			e("a", "href", m.Opts.Meta, "target", "_blank").R(" here."),
			"<br>Please save a copy for your records, and close the browser window when finished",
		),
	)
	return
}