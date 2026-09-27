// Read-only reconciliation of the admin giving report (N-017).
//
// Two independent checks, both side-effect free:
//
//  1. Postgres: for every year holding charges, the Go report
//     (payment.LoadGivingYear) and both CSV exports are compared against
//     totals Postgres computes itself. The month cut is done in SQL with
//     `created_at AT TIME ZONE <tz>`, which uses Postgres's own zone database,
//     so a Go-side time-zone mistake cannot hide by being repeated in the
//     check.
//
//     Go report ──► WriteGivingSummaryCSV ──► parse ─┐
//     ──► WriteGivingCSV        ──► parse ─┼──► diff per month + total
//     SQL: GROUP BY to_char(created_at AT TIME ZONE tz, 'YYYY-MM') ─┘
//
//  2. Stripe (optional, one month): succeeded PaymentIntents for the month are
//     listed from the Stripe API and matched to charges rows by
//     payment_token (= PaymentIntent id, see payment_recorder.go). Reports
//     gifts missing on either side, amount / refund drift, and gifts whose
//     month differs between Stripe's `created` and our `created_at`.
//
// Usage (from the church module root):
//
//	go run ./test_scripts/giving_reconcile -dsn "$DATABASE_URL" -tz America/Chicago
//	STRIPE_SECRET_KEY=rk_live_... go run ./test_scripts/giving_reconcile \
//	    -dsn "$DATABASE_URL" -tz America/Chicago -stripe-month 2026-08
//
// A restricted key with read access to PaymentIntents and Charges is enough.
// -tz should be the site's configured time_zone; the report is cut in the
// process-local zone, which config.applyTimeZone sets from time_zone.
package main

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/rohanthewiz/church/resource/payment"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/paymentintent"
)

var failures int

func fail(format string, args ...any) {
	failures++
	fmt.Printf("FAIL  "+format+"\n", args...)
}

func pass(format string, args ...any) { fmt.Printf("pass  "+format+"\n", args...) }

// monthRow is one month of totals, in the summary CSV's terms (cents).
type monthRow struct {
	Gifts, Pending       int
	Gross, Refunded, Net int64
	Rows                 int      // per-gift CSV rows (paid + pending)
	Dates                []string // per-gift CSV Date column, in order
}

func main() {
	dsn := flag.String("dsn", "postgres://devuser:secret@localhost:5432/church_test?sslmode=disable", "Postgres DSN (read-only use)")
	tzName := flag.String("tz", "America/Chicago", "site time_zone (IANA)")
	stripeMonth := flag.String("stripe-month", "", "YYYY-MM: also reconcile this month against Stripe (needs STRIPE_SECRET_KEY)")
	flag.Parse()

	loc, err := time.LoadLocation(*tzName)
	if err != nil {
		fmt.Println("bad -tz:", err)
		os.Exit(2)
	}
	// The report cuts months in now.Location(); production gets that from
	// time.Local via config.applyTimeZone. Mirror that here.
	time.Local = loc
	now := time.Now().In(loc)

	dbH, err := sql.Open("postgres", *dsn)
	if err != nil {
		fmt.Println("open:", err)
		os.Exit(2)
	}
	defer dbH.Close()

	// Guard against accidental writes: every statement below is a SELECT, and
	// the session is additionally marked read-only.
	if _, err := dbH.Exec("SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY"); err != nil {
		fmt.Println("set read only:", err)
		os.Exit(2)
	}
	// Single connection so the SET above applies to every query.
	dbH.SetMaxOpenConns(1)

	earliest, err := payment.EarliestGivingYear(dbH, loc, now.Year())
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	fmt.Printf("tz=%s  years %d..%d\n", loc, earliest, now.Year())
	for y := earliest; y <= now.Year(); y++ {
		checkYear(dbH, *tzName, y, now)
	}

	if *stripeMonth != "" {
		checkStripe(dbH, loc, *stripeMonth, now)
	}

	if failures > 0 {
		fmt.Printf("\n%d check(s) FAILED\n", failures)
		os.Exit(1)
	}
	fmt.Println("\nall checks passed")
}

// sqlMonths computes month totals for one site-local year entirely in
// Postgres, mirroring GivingTotals' rules: only paid charges count toward
// money; refunds are capped to [0, amount_paid] (refundedCents); unpaid
// charges count as Pending.
func sqlMonths(dbH *sql.DB, tz string, year int) (map[string]*monthRow, error) {
	rows, err := dbH.Query(`
		SELECT to_char(created_at AT TIME ZONE $1, 'YYYY-MM') AS m,
		       count(*) FILTER (WHERE paid IS TRUE),
		       count(*) FILTER (WHERE paid IS NOT TRUE),
		       coalesce(sum(coalesce(amount_paid,0)) FILTER (WHERE paid IS TRUE), 0),
		       coalesce(sum(GREATEST(0, LEAST(coalesce(amount_refunded,0), coalesce(amount_paid,0))))
		                FILTER (WHERE paid IS TRUE), 0)
		FROM charges
		WHERE created_at IS NOT NULL
		  AND extract(year FROM created_at AT TIME ZONE $1) = $2
		GROUP BY 1`, tz, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*monthRow{}
	for rows.Next() {
		var k string
		r := &monthRow{}
		if err := rows.Scan(&k, &r.Gifts, &r.Pending, &r.Gross, &r.Refunded); err != nil {
			return nil, err
		}
		r.Net = r.Gross - r.Refunded
		r.Rows = r.Gifts + r.Pending
		out[k] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Local timestamps of each gift, in the report's order (created_at, id),
	// formatted by Postgres for comparison with the CSV Date column.
	drows, err := dbH.Query(`
		SELECT to_char(created_at AT TIME ZONE $1, 'YYYY-MM'),
		       to_char(created_at AT TIME ZONE $1, 'YYYY-MM-DD HH24:MI')
		FROM charges
		WHERE created_at IS NOT NULL
		  AND extract(year FROM created_at AT TIME ZONE $1) = $2
		ORDER BY created_at, id`, tz, year)
	if err != nil {
		return nil, err
	}
	defer drows.Close()
	for drows.Next() {
		var k, d string
		if err := drows.Scan(&k, &d); err != nil {
			return nil, err
		}
		out[k].Dates = append(out[k].Dates, d)
	}
	return out, drows.Err()
}

// parseCents reads a CSV decimal ("1234.50") back into cents without float
// rounding.
func parseCents(s string) int64 {
	if s == "" {
		return 0
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	w, _ := strconv.ParseInt(whole, 10, 64)
	f, _ := strconv.ParseInt((frac + "00")[:2], 10, 64)
	c := w*100 + f
	if neg {
		c = -c
	}
	return c
}

func readCSV(b []byte) [][]string {
	b = bytes.TrimPrefix(b, []byte("\uFEFF"))
	recs, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		fail("CSV parse: %v", err)
		return nil
	}
	return recs
}

func checkYear(dbH *sql.DB, tz string, year int, now time.Time) {
	fmt.Printf("\n== %d\n", year)
	g, err := payment.LoadGivingYear(dbH, strconv.Itoa(year), now)
	if err != nil {
		fail("LoadGivingYear(%d): %v", year, err)
		return
	}
	want, err := sqlMonths(dbH, tz, year)
	if err != nil {
		fail("SQL month totals %d: %v", year, err)
		return
	}

	// Summary CSV → per-month rows as a treasurer would read them.
	var sb bytes.Buffer
	if err := payment.WriteGivingSummaryCSV(&sb, g); err != nil {
		fail("WriteGivingSummaryCSV: %v", err)
		return
	}
	summary := map[string]*monthRow{}
	for _, r := range readCSV(sb.Bytes())[1:] {
		gifts, _ := strconv.Atoi(r[1])
		pending, _ := strconv.Atoi(r[5])
		summary[r[0]] = &monthRow{Gifts: gifts, Gross: parseCents(r[2]),
			Refunded: parseCents(r[3]), Net: parseCents(r[4]), Pending: pending}
	}

	// Per-gift CSV → rows, net and dates regrouped by its Month column.
	var gb bytes.Buffer
	if err := payment.WriteGivingCSV(&gb, g); err != nil {
		fail("WriteGivingCSV: %v", err)
		return
	}
	perGift := map[string]*monthRow{}
	for _, r := range readCSV(gb.Bytes())[1:] {
		m := perGift[r[1]]
		if m == nil {
			m = &monthRow{}
			perGift[r[1]] = m
		}
		m.Rows++
		m.Net += parseCents(r[6])
		m.Dates = append(m.Dates, r[0])
		if !strings.HasPrefix(r[0], r[1]) {
			fail("%s: gift dated %s filed under month %s", r[1], r[0], r[1])
		}
	}

	// Every month SQL saw must be a report month; every report month is
	// checked (empty months must be zero in SQL, i.e. absent).
	for k := range want {
		if _, ok := summary[k]; !ok {
			fail("%s: SQL has %d charge(s) but the summary CSV has no such month", k, want[k].Rows)
		}
	}
	var total monthRow
	for _, m := range g.Months {
		k := m.Key()
		w := want[k]
		if w == nil {
			w = &monthRow{}
		}
		total.Gifts += w.Gifts
		total.Pending += w.Pending
		total.Gross += w.Gross
		total.Refunded += w.Refunded
		total.Net += w.Net

		s := summary[k]
		if s.Gifts != w.Gifts || s.Pending != w.Pending || s.Gross != w.Gross ||
			s.Refunded != w.Refunded || s.Net != w.Net {
			fail("%s summary: got gifts=%d gross=%d refunded=%d net=%d pending=%d, SQL gifts=%d gross=%d refunded=%d net=%d pending=%d",
				k, s.Gifts, s.Gross, s.Refunded, s.Net, s.Pending, w.Gifts, w.Gross, w.Refunded, w.Net, w.Pending)
		} else if w.Rows > 0 {
			pass("%s summary: %d gift(s), net %s, %d pending", k, w.Gifts, cents(w.Net), w.Pending)
		}

		p := perGift[k]
		if p == nil {
			p = &monthRow{}
		}
		if p.Rows != w.Rows || p.Net != w.Net {
			fail("%s per-gift CSV: rows=%d net=%d, SQL rows=%d net=%d", k, p.Rows, p.Net, w.Rows, w.Net)
		}
		if strings.Join(p.Dates, "|") != strings.Join(w.Dates, "|") {
			fail("%s per-gift CSV dates differ from Postgres local time:\n      csv %v\n      sql %v", k, p.Dates, w.Dates)
		}
	}

	t := summary["Total"]
	if t == nil || t.Gifts != total.Gifts || t.Gross != total.Gross || t.Refunded != total.Refunded ||
		t.Net != total.Net || t.Pending != total.Pending {
		fail("%d Total row %+v, SQL %+v", year, t, total)
	} else {
		pass("%d Total: %d gift(s), gross %s, refunded %s, net %s, %d pending",
			year, t.Gifts, cents(t.Gross), cents(t.Refunded), cents(t.Net), t.Pending)
	}
}

func cents(c int64) string { return fmt.Sprintf("$%d.%02d", c/100, c%100) }

// dbCharge is the subset of a charges row the Stripe comparison needs.
type dbCharge struct {
	id               int64
	token            string
	created          time.Time
	paid             bool
	amount, refunded int64
}

// checkStripe reconciles one site-local month against Stripe.
//
// The Stripe query window is widened by two days on each side. Our
// created_at is stamped when the receipt redirect or webhook records the
// intent, normally seconds after Stripe's `created` but possibly later (a
// delayed webhook). Widening lets a gift that straddles midnight on the 1st
// be matched and reported as a month shift instead of as missing.
func checkStripe(dbH *sql.DB, loc *time.Location, month string, now time.Time) {
	fmt.Printf("\n== Stripe %s\n", month)
	key := os.Getenv("STRIPE_SECRET_KEY")
	if key == "" {
		fail("STRIPE_SECRET_KEY is not set")
		return
	}
	stripe.Key = key

	start, err := time.ParseInLocation("2006-01", month, loc)
	if err != nil {
		fail("bad -stripe-month %q: %v", month, err)
		return
	}
	end := start.AddDate(0, 1, 0)
	inMonth := func(t time.Time) bool { return !t.Before(start) && t.Before(end) }

	// Stripe side: succeeded intents only; others never reach charges as paid.
	params := &stripe.PaymentIntentListParams{
		CreatedRange: &stripe.RangeQueryParams{
			GreaterThanOrEqual: start.AddDate(0, 0, -2).Unix(),
			LesserThan:         end.AddDate(0, 0, 2).Unix(),
		},
	}
	params.Limit = stripe.Int64(100)
	params.AddExpand("data.latest_charge")
	stripePIs := map[string]*stripe.PaymentIntent{}
	it := paymentintent.List(params)
	for it.Next() {
		pi := it.PaymentIntent()
		if pi.Status == stripe.PaymentIntentStatusSucceeded {
			stripePIs[pi.ID] = pi
		}
	}
	if err := it.Err(); err != nil {
		fail("Stripe list: %v", err)
		return
	}

	// DB side: the same widened window.
	rows, err := dbH.Query(`
		SELECT id, payment_token, created_at, coalesce(paid,false),
		       coalesce(amount_paid,0), coalesce(amount_refunded,0)
		FROM charges
		WHERE created_at >= $1 AND created_at < $2
		ORDER BY created_at, id`, start.AddDate(0, 0, -2), end.AddDate(0, 0, 2))
	if err != nil {
		fail("DB query: %v", err)
		return
	}
	defer rows.Close()
	dbByToken := map[string]dbCharge{}
	for rows.Next() {
		var c dbCharge
		if err := rows.Scan(&c.id, &c.token, &c.created, &c.paid, &c.amount, &c.refunded); err != nil {
			fail("DB scan: %v", err)
			return
		}
		dbByToken[c.token] = c
	}

	var sGifts, dGifts int
	var sGross, sRef, dGross, dRef int64
	var ids []string
	for id := range stripePIs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		pi := stripePIs[id]
		sCreated := time.Unix(pi.Created, 0).In(loc)
		var sRefunded int64
		if pi.LatestCharge != nil {
			sRefunded = pi.LatestCharge.AmountRefunded
		}
		if inMonth(sCreated) {
			sGifts++
			sGross += pi.AmountReceived
			sRef += sRefunded
		}
		c, ok := dbByToken[id]
		if !ok {
			if inMonth(sCreated) {
				fail("%s (%s, %s) succeeded in Stripe but has no charges row", id, sCreated.Format("2006-01-02 15:04"), cents(pi.AmountReceived))
			}
			continue
		}
		if inMonth(sCreated) != inMonth(c.created.In(loc)) {
			fail("%s month shift: Stripe %s, charges.created_at %s", id,
				sCreated.Format("2006-01-02 15:04"), c.created.In(loc).Format("2006-01-02 15:04"))
		}
		if !inMonth(sCreated) && !inMonth(c.created.In(loc)) {
			continue
		}
		if !c.paid {
			fail("%s succeeded in Stripe but charges row %d is unpaid (counted as Pending)", id, c.id)
		}
		if c.amount != pi.AmountReceived {
			fail("%s amount: charges %s, Stripe %s", id, cents(c.amount), cents(pi.AmountReceived))
		}
		if c.refunded != sRefunded {
			// Expected when a refund was issued after the gift was recorded:
			// the webhook handles only payment_intent.succeeded, so later
			// refunds never reach charges.amount_refunded.
			fail("%s refunded: charges %s, Stripe %s", id, cents(c.refunded), cents(sRefunded))
		}
	}

	for tok, c := range dbByToken {
		if !inMonth(c.created.In(loc)) {
			continue
		}
		if c.paid {
			dGifts++
			dGross += c.amount
			dRef += min(max(c.refunded, 0), c.amount)
		}
		if _, ok := stripePIs[tok]; !ok && c.paid {
			fail("charges row %d (%s, token %q) has no succeeded PaymentIntent in Stripe", c.id, cents(c.amount), tok)
		}
	}

	fmt.Printf("      Stripe:   %d gift(s), gross %s, refunded %s, net %s\n", sGifts, cents(sGross), cents(sRef), cents(sGross-sRef))
	fmt.Printf("      charges:  %d gift(s), gross %s, refunded %s, net %s\n", dGifts, cents(dGross), cents(dRef), cents(dGross-dRef))

	// And the report's own row for that month, so the three agree end to end.
	g, err := payment.LoadGivingYear(dbH, strconv.Itoa(start.Year()), now)
	if err != nil {
		fail("LoadGivingYear: %v", err)
		return
	}
	for _, m := range g.Months {
		if m.Key() == month {
			fmt.Printf("      report:   %d gift(s), gross %s, refunded %s, net %s\n",
				m.Totals.Gifts, cents(m.Totals.Gross), cents(m.Totals.Refunded), cents(m.Totals.Net()))
			if m.Totals.Gifts == sGifts && m.Totals.Gross == sGross && m.Totals.Refunded == sRef {
				pass("%s report matches Stripe", month)
			} else {
				fail("%s report differs from Stripe (see lines above)", month)
			}
		}
	}
}
