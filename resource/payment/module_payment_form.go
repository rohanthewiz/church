package payment

import (
	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/module"
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

func (m *ModulePaymentForm) Render(params map[string]map[string]string, loggedIn bool) (out string) {
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to us
		m.SetId(opts)
	}
	e := element.New
	out = e("form", "action", "/payments/create", "method", "post", "id", "payment-form").R(
		e("div", "class", "form-row").R(
			e("input", "type", "hidden", "name", "csrf", "value", m.csrf).R(),

			e("label", "for", "fullname").R("First and last name"),
			e("input", "name", "fullname", "type", "text").R(),

			e("label", "for", "card-element").R("Credit or Debit card"),
			e("div", "id", "card-element").R(),
			e("div", "id", "card-errors", "role", "alert").R(),

			e("label", "for", "amount").R("Giving amount"),
			e("input", "name", "amount", "type", "number", "min", "0", "step", "0.01").R(),
		),
		e("button", "id", "payment_form_submit_btn", "class", "submit-button").R("Submit Payment"),

		e("script", "type", "text/javascript").R(`
			var stripe = Stripe('` + config.Options.Stripe.PubKey+ `');
			var elements = stripe.elements();
			$(document).ready(function(){
				var style = {
					base: {
						fontSize: '15px',
						color: "#32325d",
					}
				};
				var card = elements.create('card', {style: style});
				card.mount('#card-element');
				card.addEventListener('change', function(event) {
				  var displayError = document.getElementById('card-errors');
				  if (event.error) {
				    displayError.textContent = event.error.message;
				  } else {
				    displayError.textContent = '';
				  }
				});
				// Create a token or display error on form submission
				var form = document.getElementById('payment-form');
				form.addEventListener('submit', function(event) {
					sbtn = document.getElementById("payment_form_submit_btn")
					sbtn.InnerHTML = "Processing..."
					event.preventDefault();
					stripe.createToken(card).then( function(result) {
						if (result.error) {
							var errorEle = document.getElementById('card-errors');
							errorEle.textContent = result.error.message;
						} else {
							stripeTokenHandler(result.token);
						}
					});
				});
			});
			function stripeTokenHandler(token) {
				var form = document.getElementById('payment-form');
				var hInput = document.createElement('input');
				hInput.setAttribute('type', 'hidden');
				hInput.setAttribute('name', 'stripeToken');
				hInput.setAttribute('value', token.id);
				form.appendChild(hInput);
				form.submit();
			}`,
		),
	)
	return
}
