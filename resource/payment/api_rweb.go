package payment

// JSON API (/api/v1) handlers for giving, consumed by the church_mobile
// Flutter app (see ai_docs/plans/2026-0707-mobile-app-flutter-api-plan.md,
// Phase 2 payments).
//
//	app                        server                          Stripe
//	 |-- POST create-intent ---->|                                |
//	 |                           |-- PaymentIntent (metadata) --->|
//	 |<----- client_secret ------|                                |
//	 |-- PaymentSheet.confirm() -------------------------------->|
//	 |                           |<---- webhook: pi.succeeded ----|
//	 |                           |   recordPaymentIntent (idempotent)
//	 |-- GET history ----------->|   charges by the bearer's email
//
// Create-intent is deliberately PUBLIC, like the web giving form — guest
// giving must not require an account. There is no CSRF gate here (CSRF is a
// cookie-session attack; this API is cookie-free), so the abuse control is a
// per-IP rate limit instead. History is personal, hence Bearer-guarded.

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/resource/apitoken"
	"github.com/rohanthewiz/church/resource/apiv1"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/paymentintent"
	"github.com/vattle/sqlboiler/queries/qm"
)

// ---------------------------------------------------------------------------
// Create-intent rate limiting
//
// Same sliding-window shape as the login limiter (resource/apitoken), but
// keyed by IP alone and counting *all* intent creations, not just failures —
// every creation costs a Stripe API call and clutters the dashboard with
// abandoned intents, so success is what we meter. The budget is sized for a
// congregation behind one NAT during offering time (dozens of givers, one or
// two intents each), while still capping a scripted spammer to a trickle.
// ---------------------------------------------------------------------------

const (
	maxIntentsPerIP = 60
	intentWindow    = 15 * time.Minute
)

type intentRateLimiter struct {
	mu     sync.Mutex
	stamps map[string][]time.Time
}

var intentLimiter = &intentRateLimiter{stamps: map[string][]time.Time{}}

// allow reports whether this IP may create another intent right now, and if
// so records the attempt. Check + record are one operation under the lock so
// two concurrent requests can't both squeeze through the last budget slot.
func (l *intentRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.stamps[ip][:0]
	for _, t := range l.stamps[ip] {
		if now.Sub(t) < intentWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) >= maxIntentsPerIP {
		l.stamps[ip] = kept
		return false
	}
	l.stamps[ip] = append(kept, now)
	return true
}

// ---------------------------------------------------------------------------
// POST /api/v1/payments/create-intent
// ---------------------------------------------------------------------------

// createIntentRequest is the JSON body. amount_cents is an integer on purpose:
// the web form takes a dollars string and float-parses it, which forced a
// math.Round dance to avoid shorting gifts by a cent — integer cents from the
// app sidesteps that whole class of bug and matches Stripe's native unit.
type createIntentRequest struct {
	AmountCents int64  `json:"amount_cents"`
	Fullname    string `json:"fullname"`
	Email       string `json:"email"`
	Comment     string `json:"comment"`
}

// APICreateIntentRWeb is the JSON twin of payment_controller's
// CreatePaymentIntentRWeb: creates a PaymentIntent carrying the giver's
// details as metadata and returns its client secret for the app's
// PaymentSheet to confirm. Confirmation, SCA/3DS, and wallets all happen
// between the app and Stripe; completion lands via the Stripe webhook
// (payment_intent.succeeded → recordPaymentIntent), so there is no mobile
// equivalent of the receipt-page redirect and nothing to record here.
//
// 200 → {client_secret, payment_intent_id, amount_cents}
// 400 bad body/amount/name/email, 429 throttled, 503 giving not configured,
// 500 Stripe failure — all {"error": msg}.
func APICreateIntentRWeb(ctx rweb.Context) error {
	// Mirrors app-config's features.giving flag: the app should have hidden
	// the giving UI, but the server still refuses cleanly if called anyway.
	if strings.TrimSpace(config.Options.Stripe.PrivKey) == "" {
		return apiv1.Error(ctx, http.StatusServiceUnavailable, "Giving is not enabled on this server")
	}

	var req createIntentRequest
	if err := json.Unmarshal(ctx.Request().Body(), &req); err != nil {
		return apiv1.Error(ctx, http.StatusBadRequest, "Request body must be JSON")
	}
	req.Fullname = strings.TrimSpace(req.Fullname)
	req.Email = strings.TrimSpace(req.Email)
	req.Comment = strings.TrimSpace(req.Comment)

	if req.AmountCents < MinChargeCents {
		return apiv1.Error(ctx, http.StatusBadRequest, "The minimum giving amount is $0.50")
	}
	// The web form lets fullname slide at intent creation (wallet flows can
	// supply billing details later), but the recorder hard-requires a customer
	// name — a nameless completed gift would fail to save. The app always has
	// a name field, so require it up front and guarantee the fallback exists.
	if req.Fullname == "" {
		return apiv1.Error(ctx, http.StatusBadRequest, "Please provide your full name")
	}
	// Just a sanity check — a garbled address only costs the giver Stripe's
	// emailed receipt; full RFC validation isn't worth false rejections.
	if req.Email != "" && !strings.Contains(req.Email, "@") {
		return apiv1.Error(ctx, http.StatusBadRequest, "Please provide a valid email address")
	}

	// Metered after validation: a giver's typo shouldn't burn rate budget;
	// only requests that would actually reach Stripe count.
	if !intentLimiter.allow(ctx.ClientIP()) {
		logger.Log("Warn", "Payment intent creation rate limited", "ip", ctx.ClientIP())
		return apiv1.Error(ctx, http.StatusTooManyRequests,
			"Too many giving attempts. Please try again later.")
	}

	stripe.Key = config.Options.Stripe.PrivKey

	params := &stripe.PaymentIntentParams{
		Amount:      stripe.Int64(req.AmountCents),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String(TxDescription()),
		// Let Stripe offer whatever methods are enabled on the account
		// (cards, Apple/Google Pay, Link, ...) through the PaymentSheet.
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	if req.Email != "" {
		params.ReceiptEmail = stripe.String(req.Email) // Stripe also emails its own receipt
	}
	// Metadata keys are contract with recordPaymentIntent (payment_controller):
	// the webhook recovers the giver's details from here, so a gift is fully
	// recordable even if the app dies the instant payment confirms.
	params.AddMetadata("customer_name", req.Fullname)
	params.AddMetadata("customer_email", req.Email)
	if req.Comment != "" {
		params.AddMetadata("comment", req.Comment)
	}
	// Distinguishes app gifts from web-form gifts in the Stripe dashboard.
	params.AddMetadata("source", "mobile_app")

	pi, err := paymentintent.New(params)
	if err != nil {
		return apiv1.ServerError(ctx,
			serr.Wrap(err, "Stripe: unable to create payment intent (mobile)",
				"amount_cents", strconv.FormatInt(req.AmountCents, 10), "fullname", req.Fullname),
			"We could not start the payment. Please try again shortly")
	}
	logger.Info("Stripe payment intent created (mobile)", "payment_intent", pi.ID,
		"amount_cents", strconv.FormatInt(req.AmountCents, 10), "customer_name", req.Fullname)

	return ctx.WriteJSON(map[string]any{
		"client_secret":     pi.ClientSecret,
		"payment_intent_id": pi.ID,
		"amount_cents":      req.AmountCents,
	})
}

// ---------------------------------------------------------------------------
// GET /api/v1/payments/history
// ---------------------------------------------------------------------------

// PaymentAPI is the JSON DTO for one recorded gift. Key names are contract
// with church_mobile. Strings serialize as "" (never null) and money stays in
// integer cents — the same Dart-side discipline as the rest of /api/v1.
// payment_token, customer fields, and meta stay server-side: the bearer
// already knows who they are, and Stripe identifiers are none of the app's
// business beyond the receipt link.
type PaymentAPI struct {
	ID                  int64  `json:"id"`
	CreatedAt           string `json:"created_at"` // RFC3339 UTC; "" if the row predates timestamps
	AmountCents         int64  `json:"amount_cents"`
	Description         string `json:"description"`
	Comment             string `json:"comment"`
	Paid                bool   `json:"paid"`
	Refunded            bool   `json:"refunded"`
	AmountRefundedCents int64  `json:"amount_refunded_cents"`
	ReceiptNumber       string `json:"receipt_number"`
	ReceiptURL          string `json:"receipt_url"`
}

func paymentAPIFromModel(c *models.Charge) PaymentAPI {
	createdAt := ""
	if c.CreatedAt.Valid {
		createdAt = c.CreatedAt.Time.UTC().Format(time.RFC3339)
	}
	return PaymentAPI{
		ID:                  c.ID,
		CreatedAt:           createdAt,
		AmountCents:         c.AmountPaid.Int64,
		Description:         c.Description.String,
		Comment:             c.Comment.String,
		Paid:                c.Paid.Bool,
		Refunded:            c.Refunded.Bool,
		AmountRefundedCents: c.AmountRefunded.Int64,
		ReceiptNumber:       c.ReceiptNumber.String,
		ReceiptURL:          c.ReceiptURL.String,
	}
}

// RecentChargesByEmail returns recorded charges whose customer_email matches
// (case-insensitively), newest first. id breaks created_at ties so pagination
// never shuffles rows recorded in the same second.
func RecentChargesByEmail(exec db.Executor, email string, limit, offset int) ([]*models.Charge, error) {
	chgs, err := models.Charges(exec,
		qm.Where("lower(customer_email) = lower(?)", email),
		qm.OrderBy("created_at DESC, id DESC"),
		qm.Limit(limit),
		qm.Offset(offset),
	).All()
	if err != nil {
		return nil, serr.Wrap(err, "Error querying charges by customer email")
	}
	return chgs, nil
}

// APIPaymentHistoryRWeb handles GET /api/v1/payments/history (Bearer-guarded).
// 200 → {"payments": [PaymentAPI...], "has_more": bool}; supports limit/offset.
//
// Gifts are matched to the account by email: the charges table predates user
// accounts entirely (guests give without one), so there is no user_id column
// to join on, and adding one would only cover future gifts. Matching the
// account email shows a member their web-form gifts too. The known tradeoff:
// history shows whatever gifts were *recorded under* that email — someone who
// typed your address into the giving form appears in your history (a modest
// disclosure of amounts/dates, acceptable at church scale; the receipt URL
// was already emailed to that address by Stripe anyway).
func APIPaymentHistoryRWeb(ctx rweb.Context) error {
	tu, ok := apitoken.CurrentUser(ctx)
	if !ok { // only reachable if routed without APIGuard — a wiring bug
		return apiv1.Error(ctx, http.StatusUnauthorized, "Authentication required")
	}

	// An account without an email can match nothing — return the empty page
	// rather than querying, else `customer_email = ''` would sweep up every
	// guest gift recorded without an email.
	email := strings.TrimSpace(tu.Email)
	if email == "" {
		return ctx.WriteJSON(map[string]any{"payments": []PaymentAPI{}, "has_more": false})
	}

	limit, offset := apiv1.ParseLimitOffset(ctx, 20, 100)

	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not load giving history")
	}
	// limit+1 probe: one spare row answers has_more without a COUNT(*) query.
	chgs, err := RecentChargesByEmail(dbH, email, limit+1, offset)
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not load giving history")
	}

	hasMore := false
	if len(chgs) > limit {
		hasMore = true
		chgs = chgs[:limit]
	}
	payments := make([]PaymentAPI, 0, len(chgs)) // len 0: serializes [] never null
	for _, c := range chgs {
		payments = append(payments, paymentAPIFromModel(c))
	}
	return ctx.WriteJSON(map[string]any{"payments": payments, "has_more": hasMore})
}
