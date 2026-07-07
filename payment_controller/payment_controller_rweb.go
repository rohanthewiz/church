package payment_controller

import (
	"math"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/config"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/paymentintent"
)

// Stripe's documented minimum charge for USD, in cents. Validated server-side
// because the form's min attribute is client-side only.
const minChargeCents = 50

func NewPaymentRWeb(ctx rweb.Context) error {
	pg, err := page.PaymentForm()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageNewRWeb(pg, ctx)))
}

// CreatePaymentIntentRWeb backs the Payment Element flow:
// the giving form JS posts the form fields here *before* confirming the payment,
// we create a PaymentIntent carrying the giver's details, and return its client
// secret. The browser then confirms the intent directly with Stripe (SCA/3DS and
// wallet flows happen there), and Stripe redirects to /payments/receipt.
//
// Design choice: name/email/comment ride along as intent *metadata* at creation
// time. That way the data is attached server-side and survives even if the giver
// completes payment but never lands back on our receipt page (we can still
// recover everything from Stripe). The name additionally lands on the payment
// method's billing_details via the JS confirm call, which is what makes it show
// on the transaction in the Stripe dashboard.
func CreatePaymentIntentRWeb(ctx rweb.Context) error {
	req := ctx.Request()

	// Same CSRF gate as the old form-post flow - the fetch() includes the token
	if !app.VerifyFormToken(req.FormValue("csrf")) {
		logger.Log("Warn", "CSRF verification failed on payment intent creation")
		return ctx.WriteJSON(map[string]string{
			"error": "Your form has expired. Please refresh the page and try again",
		})
	}

	strAmount := strings.TrimSpace(req.FormValue("amount"))
	fullname := strings.TrimSpace(req.FormValue("fullname"))
	email := strings.TrimSpace(req.FormValue("email"))
	comment := strings.TrimSpace(req.FormValue("comment"))

	amt, err := strconv.ParseFloat(strAmount, 64)
	if err != nil {
		logger.LogErr(err, "Unable to parse donation amount", "amount", strAmount)
		return ctx.WriteJSON(map[string]string{"error": "Please enter a valid giving amount"})
	}
	// math.Round, not a bare cast: int64(32.57 * 100) truncates 3256.9999... to 3256,
	// silently shorting the gift by a cent.
	amtCents := int64(math.Round(amt * 100.0))
	if amtCents < minChargeCents {
		return ctx.WriteJSON(map[string]string{"error": "The minimum giving amount is $0.50"})
	}

	stripe.Key = config.Options.Stripe.PrivKey

	params := &stripe.PaymentIntentParams{
		Amount:      stripe.Int64(amtCents),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String(txDescription()),
		// Let Stripe offer whatever methods are enabled on the account
		// (cards, Apple/Google Pay, Link, ...) through the single Payment Element.
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	if email != "" {
		params.ReceiptEmail = stripe.String(email) // Stripe also emails its own receipt
	}
	// Metadata is our server-side copy of the form fields (see func comment)
	params.AddMetadata("customer_name", fullname)
	params.AddMetadata("customer_email", email)
	if comment != "" {
		params.AddMetadata("comment", comment)
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		logger.LogErr(err, "Stripe: unable to create payment intent",
			"amount_cents", strconv.FormatInt(amtCents, 10), "fullname", fullname)
		return ctx.WriteJSON(map[string]string{
			"error": "We could not start the payment. Please try again shortly",
		})
	}
	logger.Info("Stripe payment intent created", "payment_intent", pi.ID,
		"amount_cents", strconv.FormatInt(amtCents, 10), "customer_name", fullname)

	return ctx.WriteJSON(map[string]string{"clientSecret": pi.ClientSecret})
}

// PaymentReceiptRWeb is both the Stripe return_url and the plain receipt page.
// When Stripe redirects back it appends ?payment_intent=pi_xxx; we retrieve the
// intent (with its latest charge expanded), record it locally, and email the
// receipt. Recording is idempotent, so refreshing this page is harmless.
// Without the query param (e.g. revisiting from the menu) we fall back to the
// receipt URL stored in the session.
func PaymentReceiptRWeb(ctx rweb.Context) error {
	receiptURL := ""

	if piID := strings.TrimSpace(ctx.Request().QueryParam("payment_intent")); piID != "" {
		url, err := finalizePayment(piID)
		if err != nil {
			logger.LogErr(err, "Error finalizing payment", "payment_intent", piID)
			// Fall through and render the page anyway - the giver's payment succeeded
			// on Stripe's side; our bookkeeping problem must not read as a failed gift.
		}
		receiptURL = url

		// Stash in the session too, so a later visit sans query param still finds it
		if receiptURL != "" {
			if err = cctx.SetLastDonationURLRWeb(ctx, receiptURL); err != nil {
				logger.LogErr(err, "Unable to set last donation receipt url into session",
					"url", receiptURL)
			}
		}
	}

	if receiptURL == "" {
		sess, err := cctx.GetSessionFromRWeb(ctx)
		if err == nil && sess != nil {
			receiptURL = sess.LastGivingReceiptURL
		}
	}

	pg, err := page.PaymentReceipt(receiptURL)
	if err != nil {
		logger.LogErr(err, "Error obtaining payment receipt")
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageNewRWeb(pg, ctx)))
}

// finalizePayment retrieves the intent from Stripe and records it if the payment
// actually went through. Separated from the handler so a future webhook handler
// (payment_intent.succeeded) can share it verbatim.
func finalizePayment(piID string) (receiptURL string, err error) {
	stripe.Key = config.Options.Stripe.PrivKey

	getParams := &stripe.PaymentIntentParams{}
	getParams.AddExpand("latest_charge") // receipt fields + billing details live on the charge

	pi, err := paymentintent.Get(piID, getParams)
	if err != nil {
		return "", serr.Wrap(err, "Stripe: unable to retrieve payment intent", "payment_intent", piID)
	}

	// "processing" covers bank-debit style methods that settle later; record those too
	// so we have the row when the funds land. Anything else (requires_action, canceled...)
	// is not money in motion and gets no local record.
	if pi.Status != stripe.PaymentIntentStatusSucceeded &&
		pi.Status != stripe.PaymentIntentStatusProcessing {
		return "", serr.New("Payment intent not in a completed state",
			"payment_intent", piID, "status", string(pi.Status))
	}

	return recordPaymentIntent(pi)
}

// Legacy token+Charges handler, superseded by CreatePaymentIntentRWeb above
// (Charges API is deprecated by Stripe: no SCA/3DS, no wallets, and it never
// recorded the giver's name on the Stripe transaction). Kept for reference;
// its Echo twin UpsertPayment remains live in payment_controller.go.
//
// func UpsertPaymentRWeb(ctx rweb.Context) error {
// 	csrf := ctx.Request().FormValue("csrf")
// 	// Check token valid against the kv store
// 	if !app.VerifyFormToken(csrf) {
// 		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
// 		logger.LogErr(err, "CSRF failed")
// 		return err
// 	}
// 	paymentToken := ctx.Request().FormValue("stripeToken")
// 	strAmount := ctx.Request().FormValue("amount")
// 	fullname := ctx.Request().FormValue("fullname")
// 	email := ctx.Request().FormValue("email")
// 	comment := ctx.Request().FormValue("comment")
// 	amt, err := strconv.ParseFloat(strAmount, 64)
// 	if err != nil {
// 		logger.LogErr(err, "Unable to parse donation amount")
// 		return err
// 	}
// 	// Make the Charge
// 	stripe.Key = config.Options.Stripe.PrivKey
// 	chgParams := &stripe.ChargeParams{
// 		Amount:      stripe.Int64(int64(math.Round(amt * 100.0))),
// 		Currency:    stripe.String(string(stripe.CurrencyUSD)),
// 		Description: stripe.String(txDescription()),
// 	}
// 	err = chgParams.SetSource(paymentToken)
// 	if err != nil {
// 		logger.LogErr(err, "Stripe: unable to set token source", "token", paymentToken)
// 		return err
// 	}
// 	chgResult, err := charge.New(chgParams)
// 	if err != nil {
// 		logger.LogErr(err, "Stripe: unable to charge donation amount: "+strAmount, "token", paymentToken,
// 			"fullname", fullname)
// 		return err
// 	}
// 	logger.LogAsync("Info", "Stripe payment charged", "charge", fmt.Sprintf("%#v", chgResult))
//
// 	go savePayment(chgResult, fullname, email, comment, paymentToken)
//
// 	msg := "Thank you! Your payment of $" + strAmount + " processed successfully"
//
// 	err = cctx.SetLastDonationURLRWeb(ctx, chgResult.ReceiptURL)
// 	if err != nil {
// 		logger.LogErr(err, "Unable to set last donation receipt url into session",
// 			"url", chgResult.ReceiptURL)
// 	}
// 	return app.RedirectRWeb(ctx, "/payments/receipt", msg)
// }
