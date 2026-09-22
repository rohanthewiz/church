package payment_controller

// Charge recording and giving history against real databases, on both
// backends (internal/testdb). webhook_test.go covers the recorder's control
// flow with sqlmock, which proves the SQL it sends but not that a database
// accepts it and hands the values back intact: the boolean and bigint
// columns, meta, and case-insensitive email matching.
//
// No test here gives a charge an email through recordPaymentIntent, because
// a first recording then sends a receipt through Gmail. History rows are
// inserted directly instead.

import (
	"fmt"
	"sync"
	"testing"
	"time"

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

// tokens lists each charge's payment token, for failure messages.
func tokens(cs []*models.Charge) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.PaymentToken)
	}
	return out
}
