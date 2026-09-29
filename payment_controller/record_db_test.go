package payment_controller

// Charge recording and giving history against real databases, on both
// backends (internal/testdb). webhook_test.go covers the recorder's control
// flow with sqlmock, which proves the SQL it sends but not that a database
// accepts it and hands the values back intact: the boolean and bigint
// columns, meta, and case-insensitive email matching.
//
// No test here gives a charge an email through recordPaymentIntent, because
// a first recording then sends a receipt through Gmail. History rows are
// inserted directly instead. TestWebhookRoundTripOnDB drives the same
// recorder through the HTTP handler and a fake Stripe API.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/internal/testdb"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/resource/payment"
	stripe "github.com/stripe/stripe-go/v86"
)

func TestRecordPaymentIntentOnDB(t *testing.T) {
	testdb.Each(t, func(t *testing.T) {
		withStripeConfig(t, "")
		dbH, err := db.Db()
		if err != nil {
			t.Fatal(err)
		}

		pi := &stripe.PaymentIntent{
			ID:             "pi_db_1",
			AmountReceived: 12500,
			Description:    "Tithe",
			Metadata:       map[string]string{"customer_name": "Kim Lee", "comment": "for missions"},
			LatestCharge: &stripe.Charge{
				ID: "ch_db_1", Captured: true, Paid: true,
				ReceiptNumber: "1234-5678", ReceiptURL: "https://pay.stripe.com/receipts/x",
			},
		}

		// The webhook and the receipt redirect race by design; recordMu must
		// leave exactly one row however many deliveries arrive at once.
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := recordPaymentIntent(pi); err != nil {
					errs <- err
				}
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatalf("recording failed: %v", err)
		}

		var (
			n                            int
			name, comment, receipt, meta string
			amt, refundedAmt             int64
			captured, paid, refunded     bool
		)
		row := dbH.QueryRow(`SELECT count(*) FROM charges WHERE payment_token = $1`, pi.ID)
		if err := row.Scan(&n); err != nil || n != 1 {
			t.Fatalf("rows for %s = %d (err %v), want 1", pi.ID, n, err)
		}
		readBack := func() {
			t.Helper()
			err := dbH.QueryRow(`SELECT customer_name, comment, receipt_number, meta, amount_paid,
				amount_refunded, captured, paid, refunded FROM charges WHERE payment_token = $1`, pi.ID).
				Scan(&name, &comment, &receipt, &meta, &amt, &refundedAmt, &captured, &paid, &refunded)
			if err != nil {
				t.Fatal(err)
			}
		}
		readBack()
		if name != "Kim Lee" || comment != "for missions" || receipt != "1234-5678" ||
			meta != `{"stripe_charge_id":"ch_db_1"}` || amt != 12500 || !captured || !paid || refunded {
			t.Fatalf("recorded charge = %q %q %q %q %d captured=%v paid=%v refunded=%v",
				name, comment, receipt, meta, amt, captured, paid, refunded)
		}

		// A later delivery (e.g. after a refund) takes the update path: the
		// same row changes, no second row appears.
		pi.LatestCharge.Refunded = true
		pi.LatestCharge.AmountRefunded = 2500
		if _, err := recordPaymentIntent(pi); err != nil {
			t.Fatal(err)
		}
		_ = dbH.QueryRow(`SELECT count(*) FROM charges`).Scan(&n)
		readBack()
		if n != 1 || !refunded || refundedAmt != 2500 {
			t.Fatalf("after re-delivery: rows=%d refunded=%v amount_refunded=%d", n, refunded, refundedAmt)
		}
	})
}

func TestRecentChargesByEmailOnDB(t *testing.T) {
	testdb.Each(t, func(t *testing.T) {
		dbH, err := db.Db()
		if err != nil {
			t.Fatal(err)
		}
		base := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
		for i, email := range []string{"Kim@Example.com", "kim@example.com", "other@example.com", "KIM@EXAMPLE.COM"} {
			_, err := dbH.Exec(`INSERT INTO charges (created_at, customer_name, customer_email, payment_token, paid, amount_paid)
				VALUES ($1, 'Kim', $2, $3, true, $4)`, base.AddDate(0, 0, i), email, fmt.Sprintf("pi_hist_%d", i), int64(100*(i+1)))
			if err != nil {
				t.Fatal(err)
			}
		}

		got, err := payment.RecentChargesByEmail(dbH, "kim@example.com", 2, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Newest first, matched without regard to case, other donors excluded
		if len(got) != 2 || got[0].PaymentToken != "pi_hist_3" || got[1].PaymentToken != "pi_hist_1" {
			t.Fatalf("first page = %v", tokens(got))
		}
		got, err = payment.RecentChargesByEmail(dbH, "kim@example.com", 2, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].PaymentToken != "pi_hist_0" || got[0].AmountPaid.Int64 != 100 {
			t.Fatalf("second page = %v", tokens(got))
		}
	})
}

// The full webhook round trip on both backends: signed event → StripeWebhook
// → finalizePayment → Stripe API → recordPaymentIntent → charges row. The
// Stripe API is a local httptest server (as in webhook_test.go), so no
// test-mode keys or network are needed. It serves the intent's CURRENT state,
// which the test changes between deliveries the way Stripe's would change:
//
//	delivery                         fake API serves        expect
//	payment_intent.succeeded         succeeded, no refund   200, row inserted
//	payment_intent.succeeded (again) same                   200, still 1 row
//	charge.refunded                  refunded 1500          200, row updated
//	payment_intent.succeeded (pi_2)  requires_payment_...   500, no row
//
// The event bodies carry deliberately wrong amounts: the handler takes only
// the intent id from them, so a recorded payload value would show up as a
// wrong amount here. No email is set anywhere, so no receipt is sent.
func TestWebhookRoundTripOnDB(t *testing.T) {
	testdb.Each(t, func(t *testing.T) {
		withStripeConfig(t, testSigningSecret)
		config.Options.Stripe.PrivKey = "sk_test_fake" // finalizePayment installs it as stripe.Key
		prevKey := stripe.Key
		t.Cleanup(func() { stripe.Key = prevKey })
		dbH, err := db.Db()
		if err != nil {
			t.Fatal(err)
		}

		// Current Stripe-side state per intent id, as the API would return it
		// with latest_charge expanded. Guarded because the handler's Stripe
		// call runs on the request path while the test mutates between calls.
		var (
			mu    sync.Mutex
			state = map[string]string{}
			hits  = map[string]int{}
		)
		const piPath = "/v1/payment_intents/"
		fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			id := strings.TrimPrefix(r.URL.Path, piPath)
			body, ok := state[id]
			w.Header().Set("Content-Type", "application/json")
			if !strings.HasPrefix(r.URL.Path, piPath) || !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"no such intent"}}`))
				return
			}
			hits[id]++
			_, _ = w.Write([]byte(body))
		}))
		defer fake.Close()
		prevBackend := stripe.GetBackend(stripe.APIBackend)
		stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(stripe.APIBackend,
			&stripe.BackendConfig{URL: stripe.String(fake.URL), MaxNetworkRetries: stripe.Int64(0)}))
		t.Cleanup(func() { stripe.SetBackend(stripe.APIBackend, prevBackend) })

		intent := func(id, status string, refunded bool, amtRefunded int64) string {
			return fmt.Sprintf(`{"id":%q,"object":"payment_intent","status":%q,"amount_received":5000,
				"description":"Offering","metadata":{"customer_name":"Ana Ruiz","comment":"building fund"},
				"latest_charge":{"id":"ch_%s","object":"charge","captured":true,"paid":true,
				"refunded":%t,"amount_refunded":%d,"receipt_number":"RT-1",
				"receipt_url":"https://pay.stripe.com/receipts/rt"}}`,
				id, status, id, refunded, amtRefunded)
		}
		setState := func(id, body string) { mu.Lock(); state[id] = body; mu.Unlock() }
		deliver := func(typ, obj string, wantStatus int) {
			t.Helper()
			resp := postSigned(signedEvent(typ, obj))
			if resp.Status() != wantStatus {
				t.Fatalf("%s: status = %d, want %d (body: %s)", typ, resp.Status(), wantStatus, resp.Body())
			}
		}
		type row struct {
			n                   int
			name, comment, meta string
			amt, amtRefunded    int64
			paid, refunded      bool
		}
		read := func(id string) (r row) {
			t.Helper()
			if err := dbH.QueryRow(`SELECT count(*) FROM charges WHERE payment_token = $1`, id).Scan(&r.n); err != nil {
				t.Fatal(err)
			}
			if r.n == 0 {
				return r
			}
			err := dbH.QueryRow(`SELECT customer_name, comment, meta, amount_paid, amount_refunded, paid, refunded
				FROM charges WHERE payment_token = $1`, id).
				Scan(&r.name, &r.comment, &r.meta, &r.amt, &r.amtRefunded, &r.paid, &r.refunded)
			if err != nil {
				t.Fatal(err)
			}
			return r
		}

		// 1. First delivery inserts from the API's values, not the payload's.
		setState("pi_rt_1", intent("pi_rt_1", "succeeded", false, 0))
		succeeded := `{"id":"pi_rt_1","object":"payment_intent","status":"succeeded","amount_received":999999}`
		deliver("payment_intent.succeeded", succeeded, http.StatusOK)
		r := read("pi_rt_1")
		if r.n != 1 || r.name != "Ana Ruiz" || r.comment != "building fund" || r.amt != 5000 ||
			!r.paid || r.refunded || r.meta != `{"stripe_charge_id":"ch_pi_rt_1"}` {
			t.Fatalf("after first delivery: %+v", r)
		}

		// 2. Stripe redelivers (at-least-once): still one row.
		deliver("payment_intent.succeeded", succeeded, http.StatusOK)
		if r = read("pi_rt_1"); r.n != 1 {
			t.Fatalf("after redelivery: %d rows, want 1", r.n)
		}

		// 3. A dashboard refund: the event's amount_refunded is wrong on
		// purpose; the row must take the API's 1500.
		setState("pi_rt_1", intent("pi_rt_1", "succeeded", false, 1500))
		deliver("charge.refunded",
			`{"id":"ch_pi_rt_1","object":"charge","amount_refunded":1,"payment_intent":"pi_rt_1"}`, http.StatusOK)
		if r = read("pi_rt_1"); r.n != 1 || r.amtRefunded != 1500 || r.amt != 5000 {
			t.Fatalf("after refund: %+v", r)
		}

		// 4. An intent that is not money in motion gets no row, and a 500 so
		// Stripe retries in case the state is about to change.
		setState("pi_rt_2", intent("pi_rt_2", "requires_payment_method", false, 0))
		deliver("payment_intent.succeeded", `{"id":"pi_rt_2","object":"payment_intent"}`,
			http.StatusInternalServerError)
		if r = read("pi_rt_2"); r.n != 0 {
			t.Fatalf("incomplete intent was recorded: %+v", r)
		}

		mu.Lock()
		defer mu.Unlock()
		if hits["pi_rt_1"] != 3 || hits["pi_rt_2"] != 1 {
			t.Errorf("Stripe fetches = %v, want one per delivery", hits)
		}
	})
}

// tokens lists each charge's payment token, for failure messages.
func tokens(cs []*models.Charge) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.PaymentToken)
	}
	return out
}
