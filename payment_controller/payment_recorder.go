package payment_controller

// PaymentIntents-flow support shared by the RWeb (and any future) controllers.
// Kept free of echo/rweb imports so both HTTP layers, and later a webhook handler
// or mobile API endpoint, can call into it.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/payment"
	gmail "github.com/rohanthewiz/gmail_send"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	stripe "github.com/stripe/stripe-go/v86"
)

// txDescription labels the charge in Stripe's dashboard and on receipts.
// Resolved at call time (not a package const) because this framework serves
// multiple church sites from one binary family -- the old hardcoded
// "CCSWM Donation" const was branding every site's gifts with one church's name.
func txDescription() string {
	if desc := strings.TrimSpace(config.Options.Stripe.TxDescription); desc != "" {
		return desc
	}
	if owner := strings.TrimSpace(config.Options.CopyrightOwner); owner != "" {
		return owner + " Donation"
	}
	return "Donation"
}

// chargeMeta is what we persist in the charges.meta JSON column.
// The Stripe charge id lives here (not in receipt_number, which previously and
// incorrectly held the charge id) so refunds/disputes can be reconciled against Stripe.
type chargeMeta struct {
	StripeChargeId string `json:"stripe_charge_id,omitempty"`
}

// recordPaymentIntent persists a completed (or at least confirmed) PaymentIntent to the
// local charges table and emails the giver a receipt. It is idempotent by PaymentIntent id:
// the receipt page redirect and a future webhook may both call this for the same intent, and
// browser refreshes of the receipt page certainly will. Only the first call inserts and
// emails; subsequent calls take the update path and skip the email.
//
//	Stripe redirect / webhook
//	          |
//	   retrieve PI (with latest_charge expanded)
//	          |
//	   recordPaymentIntent ---- lookup by payment_token (= PI id)
//	          |                       |
//	     insert + email          update, no email
func recordPaymentIntent(pi *stripe.PaymentIntent) (receiptURL string, err error) {
	if pi == nil {
		return "", serr.New("nil PaymentIntent passed to recordPaymentIntent")
	}

	// The latest charge carries the receipt fields and the billing details the
	// customer entered in the Payment Element. It is only populated when the
	// retrieve call expanded "latest_charge".
	chg := pi.LatestCharge

	pres := payment.ChargePresenter{}

	// Prefer what the customer typed into the payment sheet (billing details);
	// fall back to the form fields we stashed in metadata at intent creation.
	// Both sources exist because wallets (Apple/Google Pay) supply billing details
	// while our form fields are always present in metadata.
	if chg != nil && chg.BillingDetails != nil && strings.TrimSpace(chg.BillingDetails.Name) != "" {
		pres.CustomerName = chg.BillingDetails.Name
	} else {
		pres.CustomerName = pi.Metadata["customer_name"]
	}
	if chg != nil && chg.BillingDetails != nil && strings.TrimSpace(chg.BillingDetails.Email) != "" {
		pres.CustomerEmail = chg.BillingDetails.Email
	} else if pi.ReceiptEmail != "" {
		pres.CustomerEmail = pi.ReceiptEmail
	} else {
		pres.CustomerEmail = pi.Metadata["customer_email"]
	}
	pres.Comment = pi.Metadata["comment"]
	pres.Description = pi.Description

	// PaymentToken column is repurposed to hold the PaymentIntent id under this flow.
	// (Card tokens no longer exist here; the column doubles as our idempotency key.)
	pres.PaymentToken = pi.ID

	pres.AmtPaid = pi.AmountReceived
	if pi.Customer != nil {
		pres.CustomerId = pi.Customer.ID
	}

	if chg != nil {
		pres.Captured = chg.Captured
		pres.Paid = chg.Paid
		pres.Refunded = chg.Refunded
		pres.AmtRefunded = chg.AmountRefunded
		// ReceiptNumber now stores the actual Stripe receipt number.
		// The charge id goes into Meta -- previously the charge id was
		// (wrongly) stored as the receipt number.
		pres.ReceiptNumber = chg.ReceiptNumber
		pres.ReceiptURL = chg.ReceiptURL
		receiptURL = chg.ReceiptURL

		metaBytes, jerr := json.Marshal(chargeMeta{StripeChargeId: chg.ID})
		if jerr != nil {
			logger.LogErr(jerr, "Unable to marshal charge meta", "stripe_charge_id", chg.ID)
		} else {
			pres.Meta = string(metaBytes)
		}
	}

	dbH, err := db.Db()
	if err != nil {
		return receiptURL, serr.Wrap(err, "Could not obtain DB handle to record charge",
			"payment_intent", pi.ID)
	}

	// Idempotency gate: if we already recorded this intent, route to the update path
	// and remember that we must not re-send the receipt email.
	existingId, alreadyRecorded, err := payment.FindChargeIdByPaymentToken(dbH, pi.ID)
	if err != nil {
		// A lookup failure shouldn't abort recording -- worst case we insert a
		// duplicate row, which is preferable to losing the record of a real charge.
		logger.LogErr(err, "Charge idempotency lookup failed - proceeding with insert",
			"payment_intent", pi.ID)
	}
	if alreadyRecorded {
		pres.Id = fmt.Sprintf("%d", existingId)
	}

	_, err = pres.Upsert(dbH)
	if err != nil {
		return receiptURL, serr.Wrap(err, "Error saving charge record",
			"payment_intent", pi.ID, "customer_name", pres.CustomerName)
	}
	logger.Info("Charge recorded", "payment_intent", pi.ID, "already_recorded", fmt.Sprintf("%t", alreadyRecorded))

	if !alreadyRecorded {
		sendReceiptEmail(pres)
	}
	return receiptURL, nil
}

// sendReceiptEmail thanks the giver and includes the Stripe receipt link.
// Failures are logged, not returned -- the charge succeeded and is recorded;
// a receipt email problem should never surface as a payment error to the giver.
func sendReceiptEmail(pres payment.ChargePresenter) {
	if strings.TrimSpace(pres.CustomerEmail) == "" {
		logger.Log("Warn", "No customer email on charge - skipping receipt email",
			"payment_intent", pres.PaymentToken)
		return
	}

	strAmt := fmt.Sprintf("%0.2f", float64(pres.AmtPaid)/100.0)

	msg := `<body><p>Thank you ` + pres.CustomerName + `, for your investment into the Kingdom!</p>
<p>The Lord bless you and keep you. The Lord make His face to shine upon you.</p>
<p>
Description: ` + pres.Description + `<br>
Comment: ` + pres.Comment + `<br>
Amount: ` + strAmt + `<br>
Receipt Number: ` + pres.ReceiptNumber + `<br>
Receipt Link: <a href="` + pres.ReceiptURL + `">Online Receipt</a><br>
</p>
</body>`

	gcfg := gmail.GSMTPConfig{
		AccountEmail: config.Options.Gmail.Account,
		Word:         config.Options.Gmail.Word, // Can use an app password here (Enable MFA then setup app password)
		FromName:     config.Options.Gmail.FromName,
		Subject:      "Giving Receipt",
		ToAddrs:      []string{pres.CustomerEmail},
		BCCs:         config.Options.Gmail.BCCs,
		Body:         msg,
	}
	if err := gmail.GmailSend(gcfg); err != nil {
		logger.LogErr(err, "Unable to send receipt email", "to", pres.CustomerEmail)
	}
}
