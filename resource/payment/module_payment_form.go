package payment

import (
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
	e := element.New
	out = e("form", "action", "/payments/create", "method", "post", "id", "payment-form").R(
		e("h2", "class", "form-title").R("Give Securely Online"),
		e("p", "class", "subtitle").R(
			"Transactions are securely processed by Stripe payment services (https://stripe.com/about)<br>",
			"All donations are tax-deductible. Please contact Lance Parr, or Rohan Allison with any questions.",
		),
		e("input", "type", "hidden", "name", "csrf", "value", m.csrf).R(),
		e("div", "class", "form-row").R(
			e("label", "for", "fullname").R("First and last name"),
			e("input", "name", "fullname", "type", "text").R(),
		),
		e("div", "class", "form-row").R(
			e("label", "for", "email").R("Email"),
			e("input", "name", "email", "type", "text").R(),
		),
		e("div", "class", "form-row").R(
			e("label", "for", "card-element").R("Credit or Debit card"),
			e("div", "id", "card-element").R(),
		),
		e("div", "class", "form-row").R(
			e("div", "id", "card-errors", "role", "alert").R(),
		),
		e("div", "class", "form-row").R(
			e("label", "for", "amount").R("Giving amount"),
			e("input", "name", "amount", "type", "number", "min", "0", "step", "0.01").R(),
		),
		e("div", "class", "form-row").R(
			e("label", "for", "comment").R("Comment"),
			e("textarea", "name", "comment").R(),
		),
		e("button", "id", "payment_form_submit_btn", "class", "submit-button").R("Send My Gift"),

		e("script", "type", "text/javascript").R(`
			var stripe = Stripe('`+config.Options.Stripe.PubKey+`');
			var elements = stripe.elements();`,
			packed.ModulePaymentForm_js,
		),
	)
	return
}
