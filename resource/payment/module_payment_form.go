package payment

import (
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/pack/packed"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/serr"
)

const ModuleTypePaymentForm = "payment_form"

type ModulePaymentForm struct {
	module.Presenter
	csrf string
}

func NewModulePaymentForm(pres module.Presenter) (module.Module, error) {
	mod := new(ModulePaymentForm)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token")
	}
	mod.csrf = csrf
	return mod, nil
}

// Idea for go generate
// https://blog.carlmjohnson.net/post/2016-11-27-how-to-use-go-generate/
// Start the script tag
// Define variables
// In the js fragment define dummy variables - we will skip those lines in the go generate process
// Refer to the fragment as a string

func (m *ModulePaymentForm) Render(params map[string]map[string]string, loggedIn bool) (out string) {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}

	givingContactsMsg := fmt.Sprintf("Please contact %s with any questions.",
		strings.Join(config.Options.GivingContacts, " or "))

	b := element.NewBuilder()

	b.Form("action", "/payments/create", "method", "post", "id", "payment-form").R(
		b.H2Class("form-title").T("Give Securely Online"),
		b.PClass("subtitle").R(
			b.T("Transactions are securely processed by Stripe payment services (https://stripe.com/about)"),
			b.Br(),
			b.T("All donations are tax-deductible. "+givingContactsMsg),
		),
		b.Input("type", "hidden", "name", "csrf", "value", m.csrf).R(),
		b.DivClass("form-row").R(
			b.Label("for", "fullname").T("First and last name"),
			b.Input("name", "fullname", "type", "text").R(),
		),
		b.DivClass("form-row").R(
			b.Label("for", "email").T("Email"),
			b.Input("name", "email", "type", "text").R(),
		),
		b.DivClass("form-row").R(
			b.Label("for", "card-element").T("Credit or Debit card"),
			b.Div("id", "card-element").R(),
		),
		b.DivClass("form-row").R(
			b.Div("id", "card-errors", "role", "alert").R(),
		),
		b.DivClass("form-row").R(
			b.Label("for", "amount").T("Giving amount"),
			b.Input("name", "amount", "type", "number", "min", "0", "step", "0.01").R(),
		),
		b.DivClass("form-row").R(
			b.Label("for", "comment").T("Comment"),
			b.TextArea("name", "comment").R(),
		),
		b.Button("id", "payment_form_submit_btn", "class", "submit-button").T("Send My Gift"),

		b.Script("type", "text/javascript").T(`
			var stripe = Stripe('`+config.Options.Stripe.PubKey+`');
			var elements = stripe.elements();`+
			packed.ModulePaymentForm_js),
	)
	return b.String()
}
