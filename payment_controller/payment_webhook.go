package payment_controller

import (
	"encoding/json"
	"net/http"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
)

// StripeWebhook handles Stripe event notifications: payment_intent.succeeded
// and charge.refunded. (The endpoint's dashboard config must subscribe to
// both.) payment_intent.succeeded is the safety net behind the receipt-page redirect:
// givers who close the browser right after paying, and bank-debit style methods
// that confirm now but settle later, still get recorded and emailed a receipt.
// recordPaymentIntent is idempotent by intent id, so the webhook and the redirect
// both firing for the same payment is the normal, harmless case.
//
// Security: there is no session/CSRF here (Stripe is the caller, not a browser).
// Authenticity comes from verifying the Stripe-Signature header against the
// endpoint's signing secret (whsec_..., from the dashboard's webhook config).
func StripeWebhook(ctx rweb.Context) error {
	signingSecret := config.Options.Stripe.WebhookSecret
	if signingSecret == "" {
		// Misconfiguration: the route is mounted but the secret isn't set.
		// 503 (not 400) so Stripe keeps retrying until config is fixed --
		// events aren't silently lost in the meantime.
		logger.Log("Warn", "Stripe webhook called but stripe.webhook_secret is not configured")
		return ctx.Status(http.StatusServiceUnavailable).WriteJSON(map[string]string{
			"error": "webhook not configured"})
	}

	event, err := webhook.ConstructEvent(
		ctx.Request().Body(), ctx.Request().Header("Stripe-Signature"), signingSecret)
	if err != nil {
		// Bad or missing signature - not a genuine Stripe call (or wrong secret).
		// 400 tells Stripe the payload was rejected.
		logger.LogErr(err, "Stripe webhook signature verification failed")
		return ctx.Status(http.StatusBadRequest).WriteJSON(map[string]string{
			"error": "signature verification failed"})
	}

	switch event.Type {
	case stripe.EventTypePaymentIntentSucceeded:
		var pi stripe.PaymentIntent
		if err = json.Unmarshal(event.Data.Raw, &pi); err != nil {
			logger.LogErr(err, "Stripe webhook: unable to unmarshal payment intent",
				"event_id", event.ID)
			return ctx.Status(http.StatusBadRequest).WriteJSON(map[string]string{
				"error": "bad event payload"})
		}
		// Only the intent id is taken from the payload; finalizePayment re-retrieves
		// the intent from Stripe with latest_charge expanded (the webhook payload does
		// not expand it, and re-fetching also means we never act on a spoofed body).
		if _, _, err = finalizePayment(pi.ID); err != nil {
			logger.LogErr(err, "Stripe webhook: error finalizing payment", "payment_intent", pi.ID)
			// Non-2xx makes Stripe retry with backoff (up to ~3 days) -- exactly what
			// we want for transient DB/network trouble on our side.
			return ctx.Status(http.StatusInternalServerError).WriteJSON(map[string]string{
				"error": "recording failed"})
		}
		logger.Info("Stripe webhook: payment recorded", "payment_intent", pi.ID)

	case stripe.EventTypeChargeRefunded:
		// Refunds are issued from the Stripe dashboard, after the gift was
		// recorded, so without this event charges.amount_refunded stays 0 and
		// the giving report overstates net giving. Stripe sends
		// charge.refunded for every refund, full or partial, carrying the
		// charge's cumulative amount_refunded.
		//
		// Rather than writing the payload's refund fields, the charge is mapped
		// to its PaymentIntent and re-recorded through finalizePayment:
		//
		//	charge.refunded ──► charge.payment_intent (id only)
		//	                ──► finalizePayment ──► re-retrieve PI + latest_charge
		//	                ──► recordPaymentIntent ──► upsert by PI id (no email on update)
		//
		// Why this path:
		//   - Same trust model as above: only an id comes from the body; the
		//     amounts come from the Stripe API.
		//   - Out-of-order and retried deliveries converge. Each delivery
		//     writes Stripe's current state, not the state in the event, so a
		//     late retry of the first partial refund can't overwrite the second.
		//   - recordMu and the payment_token idempotency gate already cover the
		//     race with the redirect and payment_intent.succeeded.
		//   - A refund for a gift we never recorded (redirect and webhook both
		//     missed) records the gift, refund included, instead of dropping it.
		//     That takes the insert path, so the giver also gets the receipt
		//     email; its receipt link shows the refund.
		// latest_charge is the refunded charge: a PaymentIntent succeeds through
		// at most one charge, and only a succeeded charge can be refunded.
		var chg stripe.Charge
		if err = json.Unmarshal(event.Data.Raw, &chg); err != nil {
			logger.LogErr(err, "Stripe webhook: unable to unmarshal charge",
				"event_id", event.ID)
			return ctx.Status(http.StatusBadRequest).WriteJSON(map[string]string{
				"error": "bad event payload"})
		}
		if chg.PaymentIntent == nil || chg.PaymentIntent.ID == "" {
			// A charge made before the PaymentIntents flow (legacy Charges API).
			// Those rows are keyed by card token, not charge id, so there is no
			// reliable row to update. Ack so Stripe doesn't retry forever; the
			// log line is the prompt to fix the row by hand.
			logger.Log("Warn", "Stripe webhook: refund on a charge with no payment intent - not recorded",
				"charge", chg.ID, "event_id", event.ID)
			break
		}
		if _, _, err = finalizePayment(chg.PaymentIntent.ID); err != nil {
			logger.LogErr(err, "Stripe webhook: error recording refund",
				"charge", chg.ID, "payment_intent", chg.PaymentIntent.ID)
			// Retry, as above: a missed refund is a wrong giving report.
			return ctx.Status(http.StatusInternalServerError).WriteJSON(map[string]string{
				"error": "recording failed"})
		}
		logger.Info("Stripe webhook: refund recorded",
			"charge", chg.ID, "payment_intent", chg.PaymentIntent.ID)

	default:
		// Acknowledge anything else so Stripe doesn't retry event types we don't
		// handle (the dashboard config should only subscribe us to what we need)
		logger.Debug("Stripe webhook: ignoring event type", "type", string(event.Type))
	}

	return ctx.WriteJSON(map[string]string{"received": "true"})
}
