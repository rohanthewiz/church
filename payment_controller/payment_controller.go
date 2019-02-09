package payment_controller

import (
	"errors"
	"fmt"
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/logger"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"strconv"
)

func init() {
	stripe.Key = config.Options.Stripe.PrivKey // Todo! create env var override //os.Getenv("STRIPE_PRIV_KEY")
}

func NewPayment(c echo.Context) error {
	pg, err := page.PaymentForm()
	if err != nil { c.Error(err); return err }
	_ = c.HTMLBlob(200, base.RenderPageNew(pg, c))
	return  nil
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
	stripeToken := c.FormValue("stripeToken")
	logger.Log("Info", fmt.Sprintf("Stripe token: '%s'", stripeToken))
	strAmount := c.FormValue("amount")
	amt, err := strconv.ParseFloat(strAmount, 64)
	if err != nil {
		logger.LogErr(err, "Unable to parse donation amount")
		c.Error(err)
		return err
	}
	// Make the Charge
	chgParams := &stripe.ChargeParams{
		Amount: stripe.Int64(int64(amt * 10.0)),
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String("Test charge"),
	}
	err = chgParams.SetSource(stripeToken)
	if err != nil {
		logger.LogErr(err, "Stripe: unable to set token source", "token", stripeToken)
		c.Error(err)
		return err
	}
	ch, err := charge.New(chgParams)
	if err != nil {
		logger.LogErr(err, "Stripe: unable to charge donation amount: " + strAmount, "token", stripeToken)
		c.Error(err)
		return err
	}
	logger.Log("Info", "Stripe payment charged", "charge", fmt.Sprintf("%#v", ch))

	msg := "Created"
	//if efs.Id != "0" && efs.Id != "" {
	//	msg = "Updated"
	//}
	app.Redirect(c, "/", "Payment " + msg)
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
