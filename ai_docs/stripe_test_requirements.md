# Stripe Test Payment — Requirements & Runbook

Everything needed to run the live Stripe (test-mode) smoke test of mobile
giving end-to-end: Flutter app (Give tab) → church server → Stripe →
webhook → local `charges` row → `/api/v1/payments/history`.

Prepared 2026-07-15. All tooling below is already in place on this Mac;
the only missing piece is the Stripe test keys.

## The one thing still needed

**Stripe test-mode API keys** from the dashboard
(https://dashboard.stripe.com → Developers → API keys, with the
**Test mode** toggle ON):

- `pk_test_...` — publishable key
- `sk_test_...` — secret key

No webhook secret is needed from the dashboard for local testing — the
Stripe CLI mints one (see below).

## What's already in place

| Piece | State |
|---|---|
| Env overrides for Stripe secrets | `church/config/env_overrides.go` honors `STRIPE_PUB_KEY`, `STRIPE_PRIV_KEY`, `STRIPE_WEBHOOK_SECRET` (env wins over yaml) |
| Stripe CLI | v1.43.8 installed via Homebrew; `stripe listen --api-key sk_test_...` forwards webhooks without interactive login |
| cema binary | Builds against the local church module via scratch `go.work` in `cema/`; serves :8088, DB `church_development`, needs local redis |
| Android emulator | AVD `Medium_Phone_API_36.1`; app installed with `API_BASE=http://10.0.2.2:8088` (emulator alias for Mac localhost) |
| Webhook endpoint | `POST /webhooks/stripe` (router_rweb.go); signature-verified against `Stripe.WebhookSecret`, rejects if unset |
| Giving feature flag | `/api/v1/app-config` reports `features.giving=true` only when **both** pub and priv keys are non-empty (`resource/apiv1/appconfig.go`) — the Flutter Give tab keys off this |

Key config knowledge:

- cema's `cfg/options.yml` currently has `stripe: pub_key: 'TODO'` /
  `priv_key: 'TODO'`. The file is gitignored, so keys *may* be pasted
  there instead of using env vars — but env vars are preferred.
- The local `charges` row and payments history come from the **webhook**,
  not from the PaymentSheet result — without webhook forwarding the
  payment succeeds on Stripe's side but never lands in the local DB.

## Runbook

1. **Start redis** (sessions) if not running:

   ```bash
   redis-server --daemonize yes --dir /Users/ro/projs/go/church/cema
   ```

2. **Start webhook forwarding** and capture the signing secret. The CLI
   prints `whsec_...` on startup (stable per machine+key):

   ```bash
   stripe listen --api-key sk_test_XXX \
     --forward-to localhost:8088/webhooks/stripe
   ```

3. **Start cema with all three secrets** (from `cema/`; rebuild first if
   the church module changed — `go build -o cema .` with the scratch
   `go.work` present):

   ```bash
   STRIPE_PUB_KEY=pk_test_XXX \
   STRIPE_PRIV_KEY=sk_test_XXX \
   STRIPE_WEBHOOK_SECRET=whsec_XXX \
   ./cema
   ```

4. **Sanity-check the config surface** — the Give tab is driven by this:

   ```bash
   curl -s http://localhost:8088/api/v1/app-config
   # expect: "stripe_publishable_key":"pk_test_...", "features":{"giving":true,...}
   ```

5. **Launch the emulator + app** (app-config is fetched at startup, so
   restart the activity if the app was already running):

   ```bash
   flutter emulators --launch Medium_Phone_API_36.1
   adb shell am start -n com.rohanthewiz.church_mobile/.MainActivity
   ```

   If the app isn't installed (fresh AVD), from `church_mobile/`:

   ```bash
   flutter run -d emulator-5554 --dart-define=API_BASE=http://10.0.2.2:8088
   ```

6. **Drive the payment**: Give tab → enter an amount → PaymentSheet →
   test card `4242 4242 4242 4242`, any future expiry, any CVC, any ZIP.
   Other useful test cards: `4000 0000 0000 9995` (declined —
   insufficient funds), `4000 0025 0000 3155` (requires 3DS challenge).

## Verification checklist

- [ ] PaymentSheet completes with success UI in the app
- [ ] `stripe listen` terminal shows `payment_intent.succeeded` forwarded → `200`
- [ ] Local charge row exists:
  `/opt/homebrew/opt/postgresql@16/bin/psql -U devuser church_development -c "select id, amount, currency, meta from charges order by id desc limit 3;"`
  — expect receipt number + charge id in `meta` (per the 2026-0706
  PaymentIntents session doc), and the giver's name on the payment in
  the Stripe test dashboard
- [ ] `GET /api/v1/payments/history` (Bearer token, account email matching
  the charge email) returns the payment with `has_more` envelope
- [ ] Guest (no-login) giving also works — create-intent is public and
  per-IP rate-limited

## Gotchas observed on this Mac (2026-07-15 emulator session)

- `flutter run` can exit code 2 with "Error connecting to the service
  protocol" even though the APK installed and the app is running — check
  `adb shell pidof com.rohanthewiz.church_mobile` before assuming failure.
- The app's first fetch can race the emulator's wifi/DHCP at cold boot
  ("Could not reach the server") — relaunch the activity once the network
  is up.
- `psql` is not on PATH: `/opt/homebrew/opt/postgresql@16/bin/psql`.
- `adb` is not on PATH: `~/Library/Android/sdk/platform-tools/adb`.
