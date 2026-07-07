# Session: Architecture Review + Stripe PaymentIntents Migration

Session ID: `31ae9fa2-1b2c-465e-b39a-73a0cca46973`
Date: 2026-07-06

## What happened

1. Full architectural review of the `church` framework and its consumers (`cema`, `ccswm`), with a future mobile app in mind (three parallel Explore agents: framework core, multi-site usage, API readiness).
2. Payments deep-dive: found the legacy Charges API wasn't recording the customer name on Stripe transactions.
3. **Implemented Option B**: migrated payments to the PaymentIntents API + Payment Element (stripe-go v55 → v86), with bug fixes and a `payment_intent.succeeded` webhook. Committed as `c82322c` (church) and `50b3046` (cema, sample-config docs).

## Architectural review — key findings (reference for future work)

### Framework (`church`)
- **Half-finished Echo→RWeb migration is the dominant issue**: two full HTTP stacks (`church.Serve()` in router.go, `church.ServeRWeb()` in router_rweb.go), every controller duplicated (13 Echo files vs 17 `_rweb.go`). cema calls `ServeRWeb()`, **ccswm still calls the legacy Echo `Serve()`** and is pinned to church v0.1.15 (June 2025) while cema is on v0.10.1 — a year of drift.
- **Pervasive package-level globals** (config.Options, db handle, module registry, kvstore, view.PgFrame, S3/IDrive clients) — untestable, can't multi-instance. Lifecycle (seeding, module registration) is buried inside `ServeRWeb()`.
- **Sessions are now in-process memory** (`core/kvstore`), not Redis (stale comments say Redis). Restart logs everyone out; blocks horizontal scaling and durable mobile tokens.
- **Security gaps**: `AdminGuardRWeb` only checks logged-in-ness — the 5-level role model (SuperAdmin 99 / Admin 1 / Publisher 5 / Editor 7 / RegisteredUser 9) is NOT enforced in middleware; raw SQL condition strings from query params (e.g. `resource/calendar/fullcalendar_events_rweb.go:24`) are an injection risk; `RegisterUserRWeb` marked "security loophole" but still routed.
- Module interface is render-only (`Render() string`); data fetching lives in `resource/*/queries.go` (`QuerySermons/QueryEvents/QueryArticles` all take condition/order/limit/offset and return `[]Presenter`) — that query layer is the right foundation for a JSON API, not the module path.

### Multi-site (cema/ccswm)
- Sites are ~90% copy-paste forks: identical package.json/lockfile, identical ~85-file vendored Bootstrap SCSS tree, near-identical main.go (with manual "comment this out on master" edits). Real customization is only `cfg/options.yml`, ~6 stylus files, logo images.
- Live credentials sit in untracked `cema/cfg/options.yml` (IDrive keys, gmail app password, FTP/PG passwords — [redacted]); **prod section points at the dev database**; consider env-var overrides + key rotation.

### Mobile API readiness
- One real JSON endpoint exists: `GET /api/v1/sermons` (`resource/sermon/api_rweb.go`) with a proper DTO (`SermonsResp`, ISO dates, json tags) — the template to copy.
- Presenters have exported fields but no json tags; `user.Presenter` leaks Password/EncryptedSalt/ResetPasswordToken — needs a DTO, never direct serialization.
- `AudioLink` is relative (`/sermon-audio/...`); no public base-URL config; audio is proxied through the server from IDrive e2 — verify Range support in `basectlr.SendAudioFileRWeb` for mobile seek.
- Recommended sequence: migrate ccswm to RWeb → delete Echo layer → fix role enforcement + parameterize SQL → extract lifecycle from ServeRWeb → build /api/v1 + bearer tokens + durable kvstore backend.

## Payments migration (implemented this session)

### New flow (PaymentIntents + Payment Element, stripe-go v86)
- `POST /payments/create-intent` (`payment_controller_rweb.go:CreatePaymentIntentRWeb`): CSRF check → validate amount (min 50¢) → create PI with name/email/comment as **metadata** + `receipt_email` + automatic payment methods → return client secret as JSON.
- Frontend (`pack/src/module_payment_form.js`, regenerated via `go run pack/packer.go`): Payment Element in **deferred-intent mode** (`mode:'payment'`, amount synced from the form input via `elements.update`). Billing details fields set to `'never'` since our form collects name/email; they're passed in `confirmPayment`'s `payment_method_data.billing_details` — **this is what puts the giver's name on the Stripe transaction**.
- `GET /payments/receipt` doubles as the Stripe `return_url`: reads `?payment_intent=`, calls `finalizePayment` → retrieves PI with `latest_charge` expanded → `recordPaymentIntent`.
- `POST /webhooks/stripe` (`payment_webhook_rweb.go`): signature-verified (`webhook.ConstructEvent`), handles `payment_intent.succeeded`, takes only the intent id from the payload and re-retrieves from Stripe. 503 when secret unconfigured, 400 on bad signature, 500 on record failure (triggers Stripe retry ~3 days). Mounted outside session middleware.
- Shared recorder `payment_controller/payment_recorder.go`: `recordPaymentIntent` is **idempotent by PI id stored in the `payment_token` column** (`payment.FindChargeIdByPaymentToken`); first caller (webhook or redirect, whichever lands first) inserts + emails the receipt; the other updates. `txDescription()` resolves per site.

### Bug fixes rolled in
- `int64(math.Round(amt * 100))` — bare cast truncated $32.57 → $32.56 (both flows).
- `receipt_number` now stores the real Stripe receipt number; the Stripe **charge id** moves to `meta` as `{"stripe_charge_id": ...}` (was wrongly stored as receipt number).
- Hardcoded `const txDescription = "CCSWM Donation"` (mislabeled cema gifts!) → config `stripe.tx_description`, fallback `copyright_owner + " Donation"`.
- Receipt email now synchronous on first record (was fire-and-forget goroutine); composition shared by legacy + new flows (`sendReceiptEmail`).
- Legacy rweb token handler preserved as a commented block; Echo twin (`UpsertPayment`) migrated to v86 and kept live for old-pinned ccswm builds.

### New config keys (documented in cema `cfg/options-sample.yml`)
```yaml
stripe:
  tx_description: "My Church Donation"   # dashboard/receipt label
  webhook_secret: "whsec_..."            # signs /webhooks/stripe
```

### Activation / verification checklist (NOT yet done — needs live keys)
1. Set `tx_description` in cema's real `options.yml` (currently inherits fallback).
2. Stripe dashboard → Developers → Webhooks → add `https://<domain>/webhooks/stripe`, subscribe `payment_intent.succeeded`, copy `whsec_` into options.yml.
3. Test with `pk_test_`/`sk_test_` keys + card 4242 4242 4242 4242; confirm giver's name shows on the payment in the dashboard and local `charges` row has receipt number + charge id in meta.
4. Local webhook test: `stripe listen --forward-to localhost:8000/webhooks/stripe`, then `stripe trigger payment_intent.succeeded`.

## Commits
- church `c82322c` — Migrate payments to Stripe PaymentIntents API with webhook support (12 files, +731/−184)
- cema `50b3046` — Document new Stripe config keys in sample options

## Verified
- `go build ./...` + `go vet` clean (two pre-existing vet warnings unrelated: calendar json tag, echo middleware unkeyed fields).
- cema builds against the local framework via temporary `go mod edit -replace` (reverted after).
- Delayed-settlement note: `finalizePayment` also records `processing` intents (bank debits); the webhook later updates the same row on `succeeded`.
