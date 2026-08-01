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

// Self-contained styling (same zero-site-rebuild pattern as the admin forms):
// the site stylus gave #payment-form a viewport-relative 88vw width that
// overflows its column on desktop and leaves inputs at browser-default width
// on phones. Emitted with the module, these rules come after app.css in the
// document and win the (equal-specificity) cascade.
const paymentFormCSS = `
#payment-form { display: block; width: 100%; max-width: 30rem;
	margin: 0.5rem auto 2rem; padding: 0 0.5rem; }
#payment-form .form-title { text-align: center; }
#payment-form .subtitle { font-size: 0.85rem; color: #667; }
#payment-form .form-row { margin-bottom: 0.9rem; }
#payment-form .form-row label { display: block; font-size: 0.85rem;
	font-weight: 600; color: #445; margin-bottom: 0.3rem; }
#payment-form .form-row input, #payment-form .form-row textarea {
	width: 100%; box-sizing: border-box; font-size: 1rem;
	padding: 0.5rem 0.6rem; border: 1px solid #c5ccc9; border-radius: 5px;
	background: #fff; }
#payment-form .form-row textarea { min-height: 4.5rem; resize: vertical; }
#payment-form #card-errors { color: #b02a37; font-size: 0.9rem; min-height: 1.2em; }
#payment-form .submit-button { display: block; width: 100%;
	background: #2e7d32; color: #fff; border: none; border-radius: 6px;
	font-size: 1.05rem; padding: 0.7rem 1rem; cursor: pointer;
	box-shadow: 0 2px 5px rgba(0,0,0,0.15); }
#payment-form .submit-button:hover { background: #256428; }
#payment-form .submit-button[disabled] { opacity: 0.6; cursor: progress; }
`

func (m *ModulePaymentForm) Render(params map[string]map[string]string, loggedIn bool) (out string) {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}

	givingContactsMsg := fmt.Sprintf("Please contact %s with any questions.",
		strings.Join(config.Options.GivingContacts, " or "))

	b := element.NewBuilder()

	// action="#" + inline return false: the whole flow lives in the submit
	// listener (packed JS). The old action pointed at the retired
	// /payments/create route, so a submit with broken JS 404'd and lost the
	// giver's input; now it is a no-op with a <noscript> explanation.
	b.Form("action", "#", "onsubmit", "return false;", "method", "post", "id", "payment-form").R(
		b.Style().T(paymentFormCSS),
		b.H2Class("form-title").T("Give Securely Online"),
		b.PClass("subtitle").R(
			b.T("Transactions are securely processed by Stripe payment services (https://stripe.com/about)"),
			b.Br(),
			b.T("All donations are tax-deductible. "+givingContactsMsg),
		),
		b.Noscript().R(
			b.P("style", "color:#a33").T("Online giving requires JavaScript. Please enable it, or use the contact above."),
		),
		b.Input("type", "hidden", "name", "csrf", "value", m.csrf).R(),
		b.DivClass("form-row").R(
			b.Label("for", "fullname").T("First and last name"),
			b.Input("name", "fullname", "id", "fullname", "type", "text",
				"required", "required", "autocomplete", "name").R(),
		),
		b.DivClass("form-row").R(
			b.Label("for", "email").T("Email (for your receipt)"),
			b.Input("name", "email", "id", "email", "type", "email",
				"required", "required", "autocomplete", "email").R(),
		),
		b.DivClass("form-row").R(
			b.Label("for", "card-element").T("Credit or Debit card"),
			b.Div("id", "card-element").R(),
		),
		b.DivClass("form-row").R(
			b.Label("for", "amount").T("Giving amount (USD, minimum $0.50)"),
			// min mirrors Stripe's $0.50 USD minimum charge (also enforced server-side)
			b.Input("name", "amount", "id", "amount", "type", "number", "min", "0.50", "step", "0.01",
				"required", "required", "inputmode", "decimal", "placeholder", "0.00").R(),
		),
		// Error surface below the amount so amount errors are visible where
		// the giver is looking (it previously sat above, off-screen on phones)
		b.DivClass("form-row").R(
			b.Div("id", "card-errors", "role", "alert", "aria-live", "polite").R(),
		),
		b.DivClass("form-row").R(
			b.Label("for", "comment").T("Comment (optional)"),
			b.TextArea("name", "comment", "id", "comment", "maxlength", "480").R(),
		),
		b.Button("id", "payment_form_submit_btn", "class", "submit-button", "type", "submit").T("Send My Gift"),

		// Only the Stripe handle is initialized here; the packed script creates
		// `elements` itself (Payment Element in deferred-intent mode needs
		// mode/amount/currency options, which live with the rest of the JS logic)
		b.Script("type", "text/javascript").T(`
			var stripe = Stripe('`+config.Options.Stripe.PubKey+`');
			var elements;`+
			packed.ModulePaymentForm_js),
	)
	return b.String()
}
