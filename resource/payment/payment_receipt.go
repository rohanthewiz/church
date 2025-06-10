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
	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.H3Class("article-title").T("Thanks for your donation!"),
		b.PClass("receipt-info").R(
			b.T("Your receipt is available"),
			b.A("href", m.Opts.Meta, "target", "_blank").T(" here."),
			b.Br(),
			b.T("Please save a copy for your records, and close the browser window when finished"),
		),
	)
	return b.String()
}
