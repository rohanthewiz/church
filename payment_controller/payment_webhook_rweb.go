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

// StripeWebhookRWeb handles Stripe event notifications, currently just
// payment_intent.succeeded. It is the safety net behind the receipt-page redirect:
// givers who close the browser right after paying, and bank-debit style methods
// that confirm now but settle later, still get recorded and emailed a receipt.
// recordPaymentIntent is idempotent by intent id, so the webhook and the redirect
// both firing for the same payment is the normal, harmless case.
//
// Security: there is no session/CSRF here (Stripe is the caller, not a browser).
// Authenticity comes from verifying the Stripe-Signature header against the
// endpoint's signing secret (whsec_..., from the dashboard's webhook config).
func StripeWebhookRWeb(ctx rweb.Context) error {
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
		if _, err = finalizePayment(pi.ID); err != nil {
			logger.LogErr(err, "Stripe webhook: error finalizing payment", "payment_intent", pi.ID)
			// Non-2xx makes Stripe retry with backoff (up to ~3 days) -- exactly what
			// we want for transient DB/network trouble on our side.
			return ctx.Status(http.StatusInternalServerError).WriteJSON(map[string]string{
				"error": "recording failed"})
		}
		logger.Info("Stripe webhook: payment recorded", "payment_intent", pi.ID)

	default:
		// Acknowledge anything else so Stripe doesn't retry event types we don't
		// handle (the dashboard config should only subscribe us to what we need)
		logger.Debug("Stripe webhook: ignoring event type", "type", string(event.Type))
	}

	return ctx.WriteJSON(map[string]string{"received": "true"})
}
