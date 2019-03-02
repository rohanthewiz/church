// We can put required variable (dummy) declarations here before "// PACKER START"
// These are not packed
// Actual variable definitions must be in place where the asset is included
var stripe;
var elements;

// The format is PACKER START <varname_to_hold_contents>
// PACKER START ModulePaymentForm_js
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
// PACKER END