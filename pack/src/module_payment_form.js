var stripe = Stripe('` + config.Options.Stripe.PubKey+ `');
var elements = stripe.elements();

// ROPACKER START ModulePaymentForm_js
$(document).ready(function() {
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
        sbtn = document.getElementById("payment_form_submit_btn");
        sbtn.setAttribute('disabled', 'disabled');
        sbtn.InnerHTML = "Processing...";
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
}
// ROPACKER END