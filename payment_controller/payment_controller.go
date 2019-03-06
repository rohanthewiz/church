package payment_controller

import (
	"errors"
	"fmt"
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/config"
	ctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/payment"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/logger"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"strconv"
)

func NewPayment(c echo.Context) error {
	pg, err := page.PaymentForm()
	if err != nil { c.Error(err); return err }
	_ = c.HTMLBlob(200, base.RenderPageNew(pg, c))
	return  nil
}

func PaymentReceipt(c echo.Context) (err error) {
	pg, err := page.PaymentReceipt(c.(*ctx.CustomContext).Session.LastGivingReceiptURL)
	if err != nil {
		logger.LogErr(err, "Error obtaining payment receipt")
		c.Error(err)
		return err
	}
	_ = c.HTMLBlob(200, base.RenderPageNew(pg, c))
	return
}

func UpsertPayment(c echo.Context) error {
	csrf := c.FormValue("csrf")
	// Check token valid against Redis
	if !app.VerifyFormToken(csrf) { // Todo better logging here
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		logger.LogErr(err, "CSRF failed")
		c.Error(err)
		return err
	}
	paymentToken := c.FormValue("stripeToken")
	strAmount := c.FormValue("amount")
	fullname := c.FormValue("fullname")
	logger.Log("Info", fmt.Sprintf("Stripe token: '%s'", paymentToken))
	amt, err := strconv.ParseFloat(strAmount, 64)
	if err != nil {
		logger.LogErr(err, "Unable to parse donation amount")
		c.Error(err)
		return err
	}
	// Make the Charge
	stripe.Key = config.Options.Stripe.PrivKey // Todo! create env var override //os.Getenv("STRIPE_PRIV_KEY")
	chgParams := &stripe.ChargeParams{
		Amount: stripe.Int64(int64(amt * 100.0)), // Todo! Verify amount is expressed as cents
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String("Test charge"),
	}
	err = chgParams.SetSource(paymentToken)
	if err != nil {
		logger.LogErr(err, "Stripe: unable to set token source", "token", paymentToken)
		c.Error(err)
		return err
	}
	chgResult, err := charge.New(chgParams)
	if err != nil {
		logger.LogErr(err, "Stripe: unable to charge donation amount: " + strAmount, "token", paymentToken)
		c.Error(err)
		return err
	}
	logger.Log("Info", "Stripe payment charged", "charge", fmt.Sprintf("%#v", chgResult))

	// Record the charge in local DB
	chg := payment.ChargePresenter{}
	chg.CustomerName = fullname
	chg.AmtPaid = chgResult.Amount  // *chgParams.Amount
	// chg.CustomerName = ?
	chg.Description = *chgParams.Description
	chg.PaymentToken = paymentToken
	chg.Captured = chgResult.Captured
	chg.Paid = chgResult.Paid
	chg.Refunded = chgResult.Refunded
	chg.AmtRefunded = chgResult.AmountRefunded
	cust := chgResult.Customer
	if cust != nil {
		chg.CustomerId = cust.ID
	}
	chg.ReceiptNumber = chgResult.ReceiptNumber
	chg.ReceiptURL = chgResult.ReceiptURL

	updateOp, err := chg.Upsert()
	if err != nil {
		logger.LogErr(err, "Error saving charge/payment record")
		c.Error(err)
		return err
	}

	msg := "Thank you! Your payment of $" + strAmount + " processed successfully"
	if updateOp { msg = "Payment Updated" }
	logger.Log("Info", "Charge " + msg, "customer_name", chg.CustomerName, "amount_paid (cents)", strAmount,
			"receipt_number", chg.ReceiptNumber)
	err = session.SetLastDonationURL(c, chg.ReceiptURL) // store in session so can be picked up by the receipt page
	if err != nil {
		logger.LogErr(err, "Unable to set last donation receipt url into session",
			"url", chgResult.ReceiptURL)
	} else {
		logger.Log("Info", "Saved receipt url into session", "url", chgResult.ReceiptURL)
	}
	app.Redirect(c, "/payments/receipt", msg)
	return nil
}

//func ListPayments(c echo.Context) error {
//	pg, err := page.PaymentsList()
//	if err != nil { c.Error(err); return err }
//	c.HTMLBlob(200, base.RenderPageList(pg, c))
//	return  nil
//}
//
//func EditPayment(c echo.Context) error {
//	pg, err := page.PaymentForm()
//	if err != nil { c.Error(err); return err }
//	c.HTMLBlob(200, base.RenderPageSingle(pg, c))
//	return  nil
//}

//efs := payment.Presenter{}
//efs.Id = c.FormValue("payment_id")
//efs.Username = strings.TrimSpace(c.FormValue("username"))
//efs.EmailAddress = strings.TrimSpace(c.FormValue("email_address"))
//efs.Firstname = strings.TrimSpace(c.FormValue("firstname"))
//efs.Lastname = strings.TrimSpace(c.FormValue("lastname"))
//efs.Summary = c.FormValue("user_summary")
//efs.Password = c.FormValue("password")  // do not trim space!
//efs.PasswordConfirmation = c.FormValue("password_confirm")  // do not trim space!
//efs.UpdatedBy = c.(*ctx.CustomContext).Username
//role, err := strconv.ParseInt(c.FormValue("role"), 10, 64)
//if err != nil { logger.LogErr(err, "Error converting role"); return err }
//efs.Role = int(role)
//if c.FormValue("enabled") == "on" {
//	efs.Enabled = true
//}
//
//err = efs.UpsertPayment()
//if err != nil {
//	logger.LogErr(err, "Error in payment upsert", "payment_presenter", fmt.Sprintf("%#v", efs))
//	c.Error(err)
//	return err
//}

//func DeletePayment(c echo.Context) error {
//	err := payment.DeletePaymentById(c.Param("id"))
//	msg := "Payment with id: " + c.Param("id") + " deleted"
//	if err != nil {
//		msg = "Error attempting to delete payment with id: " + c.Param("id")
//		logger.LogErrAsync(err, "when", "deleting payment")
//	}
//	app.Redirect(c, "/admin/payments", msg)
//	return nil
//}
