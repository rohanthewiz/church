package payment_controller

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/config"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/payment"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/paymentintent"
)

// Minimum-charge and description helpers moved to resource/payment (giving.go)
// when the mobile API endpoint needed to share them — controllers may import
// resources but never the reverse.

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

	// The recorder hard-requires a customer name — a nameless completed gift
	// charges the card but fails to save locally (and pins the webhook in
	// Stripe's retry loop). Enforce here like the mobile endpoint always has.
	if fullname == "" {
		return ctx.WriteJSON(map[string]string{"error": "Please provide your full name"})
	}
	if email != "" && !strings.Contains(email, "@") {
		return ctx.WriteJSON(map[string]string{"error": "Please provide a valid email address"})
	}
	if len(comment) > payment.MaxCommentLen {
		comment = comment[:payment.MaxCommentLen] // Stripe metadata caps at 500/value
	}

	amt, err := strconv.ParseFloat(strAmount, 64)
	// NaN/Inf pass ParseFloat but produce garbage cents; reject them with the
	// bad-amount message rather than an opaque Stripe failure downstream.
	if err != nil || math.IsNaN(amt) || math.IsInf(amt, 0) {
		logger.LogErr(err, "Unable to parse donation amount", "amount", strAmount)
		return ctx.WriteJSON(map[string]string{"error": "Please enter a valid giving amount"})
	}
	// math.Round, not a bare cast: int64(32.57 * 100) truncates 3256.9999... to 3256,
	// silently shorting the gift by a cent.
	amtCents := int64(math.Round(amt * 100.0))
	if amtCents < payment.MinChargeCents {
		return ctx.WriteJSON(map[string]string{"error": "The minimum giving amount is $0.50"})
	}
	if amtCents > payment.MaxChargeCents {
		return ctx.WriteJSON(map[string]string{
			"error": "That amount is above our online giving limit — please contact us to give directly",
		})
	}

	// Same per-IP budget as the mobile endpoint — this route previously had no
	// rate limit, leaving an open card-testing surface.
	if !payment.AllowIntent(ctx.ClientIP()) {
		logger.Log("Warn", "Web payment intent creation rate limited", "ip", ctx.ClientIP())
		return ctx.WriteJSON(map[string]string{
			"error": "Too many giving attempts. Please try again later.",
		})
	}

	stripe.Key = config.Options.Stripe.PrivKey

	params := &stripe.PaymentIntentParams{
		Amount:      stripe.Int64(amtCents),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String(payment.TxDescription()),
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
	params.AddMetadata("source", "web") // mirrors the mobile endpoint's "mobile_app"
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

	// Stripe appends redirect_status to the return_url; a failed confirmation
	// still redirects here. Rendering "Thanks for your donation!" (and, worse,
	// falling back to the *previous* gift's receipt from the session) on a
	// failed payment misleads the giver — show the failure honestly instead.
	if ctx.Request().QueryParam("redirect_status") == "failed" {
		pg, err := page.PaymentReceipt(`{"failed": true}`)
		if err != nil {
			return err
		}
		return ctx.WriteHTML(string(base.RenderPageNewRWeb(pg, ctx)))
	}

	givenAmount, givenDate := "", ""
	if piID := strings.TrimSpace(ctx.Request().QueryParam("payment_intent")); piID != "" {
		url, pi, err := finalizePayment(piID)
		if err != nil {
			logger.LogErr(err, "Error finalizing payment", "payment_intent", piID)
			// Fall through and render the page anyway - the giver's payment succeeded
			// on Stripe's side; our bookkeeping problem must not read as a failed gift.
		}
		receiptURL = url
		if pi != nil {
			// Show the gift on the receipt page itself — for many givers this page
			// is the only confirmation they read.
			cents := pi.AmountReceived
			if cents == 0 {
				cents = pi.Amount // processing (bank-debit) intents haven't "received" yet
			}
			givenAmount = fmt.Sprintf("$%d.%02d", cents/100, cents%100)
			givenDate = time.Unix(pi.Created, 0).Format("January 2, 2006")
		}

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

	// The module accepts either a bare receipt URL or a JSON meta blob; send
	// JSON so the amount/date reach the page when we have them.
	meta := receiptURL
	if givenAmount != "" {
		if bts, jerr := json.Marshal(payment.ReceiptMeta{
			URL: receiptURL, Amount: givenAmount, Date: givenDate,
		}); jerr == nil {
			meta = string(bts)
		}
	}

	pg, err := page.PaymentReceipt(meta)
	if err != nil {
		logger.LogErr(err, "Error obtaining payment receipt")
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageNewRWeb(pg, ctx)))
}

// finalizePayment retrieves the intent from Stripe and records it if the payment
// actually went through. Separated from the handler so the webhook handler
// (payment_intent.succeeded) can share it verbatim. The retrieved intent is
// returned so the receipt page can show the gift amount/date.
func finalizePayment(piID string) (receiptURL string, pi *stripe.PaymentIntent, err error) {
	stripe.Key = config.Options.Stripe.PrivKey

	getParams := &stripe.PaymentIntentParams{}
	getParams.AddExpand("latest_charge") // receipt fields + billing details live on the charge

	pi, err = paymentintent.Get(piID, getParams)
	if err != nil {
		return "", nil, serr.Wrap(err, "Stripe: unable to retrieve payment intent", "payment_intent", piID)
	}

	// "processing" covers bank-debit style methods that settle later; record those too
	// so we have the row when the funds land. Anything else (requires_action, canceled...)
	// is not money in motion and gets no local record.
	if pi.Status != stripe.PaymentIntentStatusSucceeded &&
		pi.Status != stripe.PaymentIntentStatusProcessing {
		return "", nil, serr.New("Payment intent not in a completed state",
			"payment_intent", piID, "status", string(pi.Status))
	}

	receiptURL, err = recordPaymentIntent(pi)
	return receiptURL, pi, err
}

// Legacy token+Charges handler, superseded by CreatePaymentIntentRWeb above
// (Charges API is deprecated by Stripe: no SCA/3DS, no wallets, and it never
// recorded the giver's name on the Stripe transaction). Kept for reference;
// its Echo twin was removed along with the rest of the Echo stack.
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
