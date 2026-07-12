package payment

// Contract tests for the /api/v1 giving endpoints. Like the other API
// contract tests, these freeze the JSON shapes church_mobile hard-maps —
// key names, envelopes, and the uniform {"error": msg} failure shape.
//
// The create-intent success path runs against a stubbed Stripe backend
// (httptest server swapped in via stripe.SetBackend), so the full
// handler → stripe-go → HTTP → response pipeline is exercised without the
// network. History tests use the guard + sqlmock, mirroring the apitoken
// contract tests.

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/resource/apitoken"
	"github.com/rohanthewiz/church/resource/apiv1/apitest"
	"github.com/rohanthewiz/rweb"
	stripe "github.com/stripe/stripe-go/v86"
)

// withConfig installs a test EnvConfig and restores the prior one afterward
// (config.Options is process-global; same pattern as the apiv1 tests).
func withConfig(t *testing.T, cfg *config.EnvConfig) {
	t.Helper()
	prev := config.Options
	config.Options = cfg
	t.Cleanup(func() { config.Options = prev })
}

func givingConfig() *config.EnvConfig {
	cfg := &config.EnvConfig{}
	cfg.CopyrightOwner = "Community Church"
	cfg.Stripe.PubKey = "pk_test_123"
	cfg.Stripe.PrivKey = "sk_test_456"
	return cfg
}

// paymentsAPIServer wires the endpoints exactly as router_rweb.go does.
func paymentsAPIServer() *rweb.Server {
	s := apitest.NewServer()
	api := s.Group("/api/v1")
	api.Post("/payments/create-intent", APICreateIntentRWeb)
	api.Get("/payments/history", apitoken.APIGuard(APIPaymentHistoryRWeb))
	return s
}

var jsonHdr = []rweb.Header{{Key: "Content-Type", Value: "application/json"}}

func bearer(token string) []rweb.Header {
	return []rweb.Header{{Key: "Authorization", Value: "Bearer " + token}}
}

// stubStripe points stripe-go at an in-process server for the test's duration
// and hands the handler each request Stripe would have received.
func stubStripe(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	ts := httptest.NewServer(handler)
	prev := stripe.GetBackend(stripe.APIBackend)
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(stripe.APIBackend,
		&stripe.BackendConfig{URL: stripe.String(ts.URL)}))
	t.Cleanup(func() {
		stripe.SetBackend(stripe.APIBackend, prev)
		ts.Close()
	})
}

// ---------------------------------------------------------------------------
// POST /api/v1/payments/create-intent
// ---------------------------------------------------------------------------

func TestAPICreateIntentGivingDisabled(t *testing.T) {
	cfg := givingConfig()
	cfg.Stripe.PrivKey = "" // matches app-config's features.giving=false
	withConfig(t, cfg)

	status, doc := apitest.RequestJSON(t, paymentsAPIServer(),
		"POST", "/api/v1/payments/create-intent", jsonHdr,
		`{"amount_cents": 5000, "fullname": "Kim Lee"}`)
	apitest.WantError(t, status, 503, doc)
}

func TestAPICreateIntentValidation(t *testing.T) {
	withConfig(t, givingConfig())
	s := paymentsAPIServer()
	// None of these may reach Stripe — no stub is installed, so an escaped
	// request would fail loudly trying api.stripe.com (or trip the auth error).

	cases := []struct{ name, body string }{
		{"malformed JSON", `{"amount_cents": `},
		{"below Stripe minimum", `{"amount_cents": 49, "fullname": "Kim Lee"}`},
		{"zero amount", `{"fullname": "Kim Lee"}`},
		{"negative amount", `{"amount_cents": -5000, "fullname": "Kim Lee"}`},
		{"missing fullname", `{"amount_cents": 5000}`},
		{"garbled email", `{"amount_cents": 5000, "fullname": "Kim Lee", "email": "not-an-email"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, doc := apitest.RequestJSON(t, s,
				"POST", "/api/v1/payments/create-intent", jsonHdr, tc.body)
			apitest.WantError(t, status, 400, doc)
		})
	}
}

func TestAPICreateIntentContract(t *testing.T) {
	withConfig(t, givingConfig())

	var gotStripeReq *http.Request
	stubStripe(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("could not parse Stripe request form: %v", err)
		}
		gotStripeReq = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "pi_test_1",
			"object": "payment_intent",
			"client_secret": "pi_test_1_secret_abc",
			"status": "requires_payment_method"
		}`))
	})

	status, doc := apitest.RequestJSON(t, paymentsAPIServer(),
		"POST", "/api/v1/payments/create-intent", jsonHdr,
		`{"amount_cents": 1500, "fullname": "Kim Lee", "email": "kim@example.com", "comment": "Tithe"}`)
	if status != 200 {
		t.Fatalf("status = %d, want 200 (doc: %v)", status, doc)
	}

	apitest.WantKeys(t, doc, "client_secret", "payment_intent_id", "amount_cents")
	if cs, _ := doc["client_secret"].(string); cs != "pi_test_1_secret_abc" {
		t.Errorf("client_secret = %q, want the stub's secret", cs)
	}
	if id, _ := doc["payment_intent_id"].(string); id != "pi_test_1" {
		t.Errorf("payment_intent_id = %q, want pi_test_1", id)
	}
	if amt, _ := doc["amount_cents"].(float64); int64(amt) != 1500 {
		t.Errorf("amount_cents = %v, want 1500", doc["amount_cents"])
	}

	// What actually went to Stripe: exact cents, and the metadata contract
	// recordPaymentIntent recovers giver details from.
	if gotStripeReq == nil {
		t.Fatal("handler never called Stripe")
	}
	form := gotStripeReq.Form
	if got := form.Get("amount"); got != "1500" {
		t.Errorf("Stripe amount = %q, want 1500", got)
	}
	if got := form.Get("metadata[customer_name]"); got != "Kim Lee" {
		t.Errorf("metadata[customer_name] = %q, want Kim Lee", got)
	}
	if got := form.Get("metadata[customer_email]"); got != "kim@example.com" {
		t.Errorf("metadata[customer_email] = %q", got)
	}
	if got := form.Get("metadata[comment]"); got != "Tithe" {
		t.Errorf("metadata[comment] = %q, want Tithe", got)
	}
	if got := form.Get("receipt_email"); got != "kim@example.com" {
		t.Errorf("receipt_email = %q", got)
	}
	if got := form.Get("metadata[source]"); got != "mobile_app" {
		t.Errorf("metadata[source] = %q, want mobile_app", got)
	}
}

func TestAPICreateIntentStripeFailure(t *testing.T) {
	withConfig(t, givingConfig())
	stubStripe(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error": {"type": "card_error", "message": "boom"}}`))
	})

	// A Stripe-side failure must still come back as the uniform JSON error
	// shape (never an HTML error page, never Stripe's own error passthrough).
	status, doc := apitest.RequestJSON(t, paymentsAPIServer(),
		"POST", "/api/v1/payments/create-intent", jsonHdr,
		`{"amount_cents": 5000, "fullname": "Kim Lee"}`)
	apitest.WantError(t, status, 500, doc)
}

// ---------------------------------------------------------------------------
// GET /api/v1/payments/history
// ---------------------------------------------------------------------------

// tokenUserCols matches APIGuard's JOIN select-list ordering.
var tokenUserCols = []string{"id", "username", "first_name", "last_name", "email_address", "role"}

// expectGuardPass primes the mock for a successful Bearer lookup as the given
// email's user.
func expectGuardPass(mock sqlmock.Sqlmock, email string) {
	mock.ExpectQuery(regexp.QuoteMeta("FROM api_tokens t")).
		WillReturnRows(sqlmock.NewRows(tokenUserCols).
			AddRow(int64(7), "kim", "Kim", "Lee", email, 9))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE api_tokens SET last_used_at")).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

// chargeCols is the subset of charges columns the DTO reads; sqlboiler binds
// returned columns by name, so a partial select-list is fine for tests.
var chargeCols = []string{"id", "created_at", "customer_name", "customer_email",
	"description", "comment", "receipt_number", "receipt_url", "payment_token",
	"paid", "amount_paid", "refunded", "amount_refunded"}

func chargeRow(rows *sqlmock.Rows, id int64, when time.Time, amtCents int64) *sqlmock.Rows {
	return rows.AddRow(id, when, "Kim Lee", "kim@example.com",
		"Community Church Donation", "Tithe", "1234-5678", "https://pay.stripe.com/receipts/x",
		"pi_test_1", true, amtCents, false, int64(0))
}

func TestAPIPaymentHistoryRequiresAuth(t *testing.T) {
	apitest.MockDB(t) // no expectations: an anonymous request must not touch the DB
	status, doc := apitest.RequestJSON(t, paymentsAPIServer(),
		"GET", "/api/v1/payments/history", nil, "")
	apitest.WantError(t, status, 401, doc)
}

func TestAPIPaymentHistoryContract(t *testing.T) {
	mock := apitest.MockDB(t)
	expectGuardPass(mock, "kim@example.com")
	rows := sqlmock.NewRows(chargeCols)
	chargeRow(rows, 42, time.Date(2026, 7, 5, 15, 4, 5, 0, time.UTC), 5000)
	// The email match must be the bound parameter — the case-insensitive
	// matching contract lives in the SQL, so pin it here.
	mock.ExpectQuery(regexp.QuoteMeta(`lower(customer_email) = lower($1)`)).
		WithArgs("kim@example.com").
		WillReturnRows(rows)

	status, doc := apitest.RequestJSON(t, paymentsAPIServer(),
		"GET", "/api/v1/payments/history", bearer("sometoken"), "")
	if status != 200 {
		t.Fatalf("status = %d, want 200 (doc: %v)", status, doc)
	}

	apitest.WantKeys(t, doc, "payments", "has_more")
	if hasMore, _ := doc["has_more"].(bool); hasMore {
		t.Error("has_more must be false when the page wasn't filled")
	}
	payments, _ := doc["payments"].([]any)
	if len(payments) != 1 {
		t.Fatalf("payments length = %d, want 1 (doc: %v)", len(payments), doc)
	}
	p, _ := payments[0].(map[string]any)
	apitest.WantKeys(t, p, "id", "created_at", "amount_cents", "description", "comment",
		"paid", "refunded", "amount_refunded_cents", "receipt_number", "receipt_url")
	if amt, _ := p["amount_cents"].(float64); int64(amt) != 5000 {
		t.Errorf("amount_cents = %v, want 5000", p["amount_cents"])
	}
	if created, _ := p["created_at"].(string); created != "2026-07-05T15:04:05Z" {
		t.Errorf("created_at = %q, want RFC3339 UTC", p["created_at"])
	}
	// Server-side-only fields must not leak into the DTO
	for _, banned := range []string{"payment_token", "customer_email", "customer_name", "meta"} {
		if _, present := p[banned]; present {
			t.Errorf("field %q must not be serialized to the app", banned)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPIPaymentHistoryHasMore(t *testing.T) {
	mock := apitest.MockDB(t)
	expectGuardPass(mock, "kim@example.com")
	// limit=2 → the query probes for 3; returning 3 means another page exists
	rows := sqlmock.NewRows(chargeCols)
	base := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	for i := range int64(3) {
		chargeRow(rows, 100-i, base.Add(-time.Duration(i)*time.Hour), 1000+i)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`lower(customer_email) = lower($1)`)).
		WithArgs("kim@example.com").
		WillReturnRows(rows)

	status, doc := apitest.RequestJSON(t, paymentsAPIServer(),
		"GET", "/api/v1/payments/history?limit=2", bearer("sometoken"), "")
	if status != 200 {
		t.Fatalf("status = %d, want 200 (doc: %v)", status, doc)
	}
	if hasMore, _ := doc["has_more"].(bool); !hasMore {
		t.Error("has_more must be true when the probe row came back")
	}
	payments, _ := doc["payments"].([]any)
	if len(payments) != 2 {
		t.Errorf("payments length = %d, want the requested limit 2", len(payments))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPIPaymentHistoryNoAccountEmail(t *testing.T) {
	mock := apitest.MockDB(t)
	// Guard passes but the account has no email: must answer the empty page
	// WITHOUT querying charges (an '' match would sweep up guest gifts
	// recorded with no email).
	expectGuardPass(mock, "")

	status, doc := apitest.RequestJSON(t, paymentsAPIServer(),
		"GET", "/api/v1/payments/history", bearer("sometoken"), "")
	if status != 200 {
		t.Fatalf("status = %d, want 200 (doc: %v)", status, doc)
	}
	payments, ok := doc["payments"].([]any)
	if !ok {
		t.Fatalf(`payments must be a JSON array (never null), got %v`, doc["payments"])
	}
	if len(payments) != 0 {
		t.Errorf("payments must be empty for an email-less account, got %v", payments)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
