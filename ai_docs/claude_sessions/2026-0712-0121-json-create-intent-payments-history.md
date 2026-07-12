# Session: JSON Create-Intent + Payments History API (mobile milestone (b) part 2)

Session ID: `38f9a55a-216d-4002-af6a-9061c1c01700`
Date: 2026-07-12

## What was asked

Continue with the JSON create-intent milestone from the previous session —
the JSON variant of the giving flow for the church_mobile Flutter app, plus
`GET /api/v1/payments/history` (milestone (b) part 2 in
`ai_docs/plans/2026-0707-mobile-app-flutter-api-plan.md`).

## POST /api/v1/payments/create-intent (implemented)

- **`resource/payment/api_rweb.go`** — `APICreateIntentRWeb`, the JSON twin of
  payment_controller's `CreatePaymentIntentRWeb`.
- **Public by design**, like the web giving form: guest giving must not require
  an account. No CSRF gate (CSRF is a cookie-session attack; this API is
  cookie-free) — abuse control is a **per-IP sliding-window rate limit**
  (60 creations / 15 min, sized for a congregation behind one NAT during
  offering). Metered *after* validation so a giver's typo doesn't burn budget;
  only requests that would reach Stripe count. Check+record are one operation
  under the lock.
- Body `{amount_cents (int), fullname, email?, comment?}` →
  `{client_secret, payment_intent_id, amount_cents}`. Integer cents on
  purpose — the web form's dollars-string float parse forced a math.Round
  dance; cents-as-int is Stripe's native unit and dodges that bug class.
- **Metadata contract preserved**: `customer_name`, `customer_email`,
  `comment` — exactly what `recordPaymentIntent` (payment_controller) reads —
  plus `source=mobile_app` to distinguish app gifts in the Stripe dashboard.
  Completion is recorded by the existing webhook (`payment_intent.succeeded`);
  there is no mobile equivalent of the receipt-redirect leg, so zero new
  recording code.
- `fullname` is required (web version lets it slide for wallet flows, but the
  recorder hard-requires a customer name; the app always has a name field).
  Email gets only an "@" sanity check. Amount `>= MinChargeCents` (50).
- Errors: 400 body/amount/name/email, 429 throttled, 503 when the Stripe
  private key is unconfigured (mirrors app-config's `features.giving` flag),
  500 (uniform JSON shape) on Stripe failure.

## GET /api/v1/payments/history (implemented)

- Same file — `APIPaymentHistoryRWeb`, wrapped in `apitoken.APIGuard`.
- **Email matching**: charges are matched case-insensitively on the account
  email (`lower(customer_email) = lower($1)`). The charges table has no
  user_id column — it predates accounts and guests give without one — and
  email matching also surfaces a member's web-form gifts. Documented
  tradeoff: history shows gifts *recorded under* that email, whoever typed it
  (modest disclosure, acceptable at church scale).
- An account with no email answers the empty page **without querying** —
  otherwise `customer_email = ''` would sweep up guest gifts recorded with no
  email.
- Envelope `{payments: [...], has_more}` — **first endpoint with the
  has_more pagination shape** (limit+1 probe, no COUNT). limit/offset via
  `apiv1.ParseLimitOffset` (default 20, cap 100). Order
  `created_at DESC, id DESC` so same-second rows never shuffle across pages.
- DTO `PaymentAPI`: id, created_at (RFC3339 UTC, "" if null), amount_cents,
  description, comment, paid, refunded, amount_refunded_cents,
  receipt_number, receipt_url. Deliberately omits payment_token, customer
  fields, meta. Strings "" never null; array `[]` never null.
- Query `RecentChargesByEmail(exec, email, limit, offset)` follows the
  executor-first convention.

## Supporting changes

- **`apitoken.CurrentUser(ctx)`** (resource/apitoken/api_rweb.go) — exported
  accessor for the guard-resolved TokenUser; the context key stays unexported
  so nothing outside the package can spoof the guard's stash. APIMeRWeb now
  uses it.
- **`resource/payment/giving.go`** — `TxDescription()` and `MinChargeCents`
  moved from payment_controller (controllers may import resources, never the
  reverse). payment_controller call sites updated.
- **`router_rweb.go`** — both routes on the `/api/v1` group; history wrapped
  per-handler in APIGuard (the rweb group-middleware gotcha).

## Tests (resource/payment/api_rweb_test.go)

- **Stubbed Stripe backend**: httptest server swapped in via
  `stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(...))` —
  the success path exercises the real stripe-go pipeline and asserts exactly
  what Stripe receives (amount=1500, metadata contract, receipt_email,
  source=mobile_app). New reusable pattern for API tests.
- Also: 503 giving-disabled; six 400 validation cases (run with NO stub
  installed, so an escaped request fails loudly); Stripe failure → uniform
  JSON 500; history contract keys + banned-fields check; has_more probe
  (limit=2, 3 rows → 2 items + true); 401 without token (no DB touch);
  empty-email guard (no charges query expected).
- Guard mocking mirrors apitoken's contract tests (`FROM api_tokens t` rows +
  `last_used_at` touch). Charge rows use a partial column list — sqlboiler
  binds by name.

## Verification

- `go build ./...`, `go vet ./...` clean; `go test ./...` — all 14 packages
  green.
- NOT yet driven against live Stripe test keys — smoke-test create-intent
  when next running a site binary.

## Docs / memory

- `ai_docs/fable_platform_analysis.md` — milestone (b) marked COMPLETED with
  detail; `ai_docs/plans/2026-0707-mobile-app-flutter-api-plan.md` — Phase 2
  payments line marked IMPLEMENTED with both shapes.
- Memory `mobile-interop-priority` refreshed.

## Not done / next steps

- Wire `apitoken.RevokeAllForUser` to password change / user disable.
- `has_more` pagination retrofit on the older list endpoints (sermons,
  articles, events) — history sets the shape.
- Flutter side: token store (flutter_secure_storage), login screen,
  feed/sermon screens, just_audio, giving via flutter_stripe PaymentSheet
  consuming `client_secret`.
- Browser check of the responsive layout on a running site binary; live
  Stripe smoke test of create-intent.
- Conventions: query code takes `db.Executor` first; test packages
  transitively importing resource/auth need the `cfg/random_seeds.txt`
  fixture; Stripe can be stubbed in tests via httptest + stripe.SetBackend.
