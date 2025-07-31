package payment_controller

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/config"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/page"
	// payment and gmail are used by savePayment in payment_controller.go
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
)

func NewPaymentRWeb(ctx rweb.Context) error {
	pg, err := page.PaymentForm()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageNewRWeb(pg, ctx)))
}

func PaymentReceiptRWeb(ctx rweb.Context) error {
	sess, err := cctx.GetSessionFromRWeb(ctx)
	receiptURL := ""
	if err == nil && sess != nil {
		receiptURL = sess.LastGivingReceiptURL
	}
	
	pg, err := page.PaymentReceipt(receiptURL)
	if err != nil {
		logger.LogErr(err, "Error obtaining payment receipt")
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageNewRWeb(pg, ctx)))
}

// txDescription is defined in payment_controller.go

func UpsertPaymentRWeb(ctx rweb.Context) error {
	csrf := ctx.Request().FormValue("csrf")
	// Check token valid against Redis
	if !app.VerifyFormToken(csrf) { // Todo better logging here
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		logger.LogErr(err, "CSRF failed")
		return err
	}
	paymentToken := ctx.Request().FormValue("stripeToken")
	strAmount := ctx.Request().FormValue("amount")
	fullname := ctx.Request().FormValue("fullname")
	email := ctx.Request().FormValue("email")
	comment := ctx.Request().FormValue("comment")
	// logger.Debug(fmt.Sprintf("Stripe token: '%s'", paymentToken))
	amt, err := strconv.ParseFloat(strAmount, 64)
	if err != nil {
		logger.LogErr(err, "Unable to parse donation amount")
		return err
	}
	// Make the Charge
	stripe.Key = config.Options.Stripe.PrivKey // Todo! create env var override //os.Getenv("STRIPE_PRIV_KEY")
	chgParams := &stripe.ChargeParams{
		Amount:      stripe.Int64(int64(amt * 100.0)), // Todo! Verify amount is expressed as cents
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String(txDescription),
	}
	err = chgParams.SetSource(paymentToken)
	if err != nil {
		logger.LogErr(err, "Stripe: unable to set token source", "token", paymentToken)
		return err
	}
	chgResult, err := charge.New(chgParams)
	if err != nil {
		logger.LogErr(err, "Stripe: unable to charge donation amount: "+strAmount, "token", paymentToken,
			"fullname", fullname)
		return err
	}
	logger.LogAsync("Info", "Stripe payment charged", "charge", fmt.Sprintf("%#v", chgResult))

	go savePayment(chgResult, fullname, email, comment, paymentToken) // uses the function from payment_controller.go

	msg := "Thank you! Your payment of $" + strAmount + " processed successfully"
	// Todo - if updateOp { msg = "Payment Updated" }

	logger.LogAsync("Info", "Charge "+msg, "customer_name", fullname, "amount_paid (cents)", strAmount,
		"receipt_number", chgResult.ReceiptNumber, "receipt url", chgResult.ReceiptURL)

	err = cctx.SetLastDonationURLRWeb(ctx, chgResult.ReceiptURL) // store in session so can be picked up by the receipt page
	if err != nil {
		logger.LogErr(err, "Unable to set last donation receipt url into session",
			"url", chgResult.ReceiptURL)
	} else {
		logger.LogAsync("Info", "Saved receipt url into session", "url", chgResult.ReceiptURL)
	}
	return app.RedirectRWeb(ctx, "/payments/receipt", msg)
}