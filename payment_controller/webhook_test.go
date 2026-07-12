package payment_controller

// Stripe webhook tests: signature gate behavior and the recorder's
// idempotency-by-intent-id. Signed payloads are produced with stripe-go's own
// test helper, so verification runs the real HMAC path — nothing is stubbed
// except the DB (sqlmock via apitest.MockDB; recordPaymentIntent reaches the
// handle through db.Db() at its boundary).
//
// The payment_intent.succeeded happy path is NOT driven through the HTTP
// handler here: it re-retrieves the intent from Stripe's API (by design — the
// handler never trusts the webhook body), which cannot run offline. The
// recording logic behind it is covered directly via recordPaymentIntent.

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/resource/apiv1/apitest"
	"github.com/rohanthewiz/rweb"
	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
)

const testSigningSecret = "whsec_test_secret"

// withStripeConfig installs a minimal EnvConfig and restores the prior one.
func withStripeConfig(t *testing.T, webhookSecret string) {
	t.Helper()
	prev := config.Options
	cfg := &config.EnvConfig{}
	cfg.Stripe.WebhookSecret = webhookSecret
	config.Options = cfg
	t.Cleanup(func() { config.Options = prev })
}

func newWebhookServer() *rweb.Server {
	s := apitest.NewServer()
	s.Post("/webhooks/stripe", StripeWebhookRWeb)
	return s
}

func TestWebhookAnswers503WhenSecretUnconfigured(t *testing.T) {
	withStripeConfig(t, "") // empty secret = webhook processing disabled

	resp := newWebhookServer().Request("POST", "/webhooks/stripe", nil,
		strings.NewReader(`{"type":"payment_intent.succeeded"}`))
	// 503 (not 400) so Stripe retries until the config is fixed — events must
	// not be silently dropped during a misconfiguration window.
	if resp.Status() != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 (body: %s)", resp.Status(), resp.Body())
	}
}

func TestWebhookRejectsBadSignature(t *testing.T) {
	withStripeConfig(t, testSigningSecret)

	resp := newWebhookServer().Request("POST", "/webhooks/stripe",
		[]rweb.Header{{Key: "Stripe-Signature", Value: "t=1,v1=deadbeef"}},
		strings.NewReader(`{"type":"payment_intent.succeeded"}`))
	if resp.Status() != http.StatusBadRequest {
		t.Errorf("forged signature: status = %d, want 400 (body: %s)", resp.Status(), resp.Body())
	}
}

func TestWebhookAcksUnhandledEventTypes(t *testing.T) {
	withStripeConfig(t, testSigningSecret)
	apitest.MockDB(t) // no expectations: unhandled events must never touch the DB

	// ConstructEvent rejects events whose api_version doesn't match the
	// stripe-go binding, so the test event must claim the library's version.
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: []byte(`{"id":"evt_test_1","object":"event","api_version":"` + stripe.APIVersion +
			`","type":"charge.refunded","data":{"object":{}}}`),
		Secret: testSigningSecret,
	})
	resp := newWebhookServer().Request("POST", "/webhooks/stripe",
		[]rweb.Header{{Key: "Stripe-Signature", Value: signed.Header}},
		strings.NewReader(string(signed.Payload)))

	// 200 acknowledges the event so Stripe does not retry types we ignore.
	if resp.Status() != http.StatusOK {
		t.Errorf("status = %d, want 200 (body: %s)", resp.Status(), resp.Body())
	}
	if !strings.Contains(string(resp.Body()), `"received":"true"`) {
		t.Errorf("body should ack receipt, got %s", resp.Body())
	}
}

// The recorder must be idempotent by PaymentIntent id: the receipt-page
// redirect and the webhook both fire for the same payment in normal operation,
// and only one charge row may result.
func TestRecordPaymentIntentIsIdempotent(t *testing.T) {
	withStripeConfig(t, testSigningSecret)
	mock := apitest.MockDB(t)

	pi := &stripe.PaymentIntent{
		ID:             "pi_test_777",
		AmountReceived: 12500,
		Description:    "Tithe",
		// customer_name from metadata (no LatestCharge in this scenario);
		// no email anywhere → the receipt-email step is skipped entirely.
		Metadata: map[string]string{"customer_name": "Kim Lee"},
	}

	// First delivery: idempotency lookup misses → INSERT.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "charges" WHERE (payment_token = $1)`)).
		WithArgs("pi_test_777").
		WillReturnRows(sqlmock.NewRows([]string{"id"})) // no rows
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "charges"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	if _, err := recordPaymentIntent(pi); err != nil {
		t.Fatalf("first recording failed: %v", err)
	}

	// Second delivery of the same intent: lookup hits → load row → UPDATE.
	// No INSERT expectation — an attempted insert fails ExpectationsWereMet.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "charges" WHERE (payment_token = $1)`)).
		WithArgs("pi_test_777").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "charges" WHERE (id = $1)`)).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "customer_name", "payment_token"}).
			AddRow(int64(1), "Kim Lee", "pi_test_777"))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "charges"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := recordPaymentIntent(pi); err != nil {
		t.Fatalf("second recording failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
