# Giving report reconciliation (N-017) and `charge.refunded` webhook

Session ID: `39631dbd-ce51-4e39-b079-12c5a961ddaa`
Date: 2026-09-27

## Goal

Work next-list item N-017: check the giving report and the summary CSV against
real charge data on Postgres, and against Stripe for one month, with
`time_zone` set. Then, at the user's request, add `charge.refunded` handling
to the Stripe webhook.

## Findings

- No local database holds real charges. `church_development` and
  `cema_prodcopy` have 0 rows. `church_test` has the 9 synthetic rows seeded
  for N-013, including two month-boundary probes (Feb 28 19:40 CST and
  Apr 30 23:30 CDT).
- cema's `cfg/options.yml` has `time_zone: America/Chicago` but Stripe keys
  set to `'TODO'`, so no Stripe account is reachable from here.
- `psql` isn't on PATH. Use `/opt/homebrew/opt/postgresql@16/bin/psql`.
  Postgres's own `TimeZone` is America/Chicago.
- **Real gap:** the webhook handled only `payment_intent.succeeded`. A refund
  issued in the Stripe dashboard never updated `charges.amount_refunded`, so
  the report's Refunded and Net figures were wrong for refunded gifts.
- In the PaymentIntents flow, `charges.payment_token` holds the PaymentIntent
  id and `meta` holds `{"stripe_charge_id": ...}` (payment_recorder.go).
  Rows from the old Charges API are keyed by card token.

## Changes

### `test_scripts/giving_reconcile/main.go` (new, read-only)

- **Postgres check:** for every year from the earliest charge to now, it
  compares `payment.LoadGivingYear`, `WriteGivingSummaryCSV` and
  `WriteGivingCSV` (both parsed back) with month totals computed in SQL. The
  SQL uses `to_char(created_at AT TIME ZONE $tz, 'YYYY-MM')`, so Postgres's
  zone database is independent of Go's. It applies the same rules as
  `GivingTotals`: paid-only money, refunds capped to `[0, amount_paid]`,
  unpaid charges counted as Pending. It also checks each CSV Date against
  Postgres-formatted local time and checks the Total row.
- **Stripe check (`-stripe-month YYYY-MM`, `STRIPE_SECRET_KEY`):** it lists
  succeeded PaymentIntents (with `data.latest_charge` expanded) over the
  month, widened by ±2 days, and matches them to rows by `payment_token`. It
  reports:
  - gifts missing on either side
  - amount or refund drift
  - a payment that succeeded in Stripe but shows unpaid locally
  - gifts whose month differs between Stripe's `created` and our
    `created_at`

  It then prints Stripe, charges-table and report totals for the month.
- Safety: `SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY` on a
  single-connection pool.
- It sets `time.Local = loc`, mirroring what `config.applyTimeZone` does in
  production.
- Results on `church_test`: all checks pass in America/Chicago, UTC and
  Pacific/Auckland, and the boundary gifts move months correctly.
- Negative control: a temporary copy with the SQL forced to UTC while the
  report ran in Chicago failed on exactly the boundary months. The copy was
  deleted.
- The Stripe half compiles and passes vet but has never been run: there is
  no key.

### `payment_controller/payment_webhook.go`

- A new `case stripe.EventTypeChargeRefunded` reads only
  `charge.payment_intent.id` from the body and calls
  `finalizePayment(piID)`. That fetches the PaymentIntent again with
  `latest_charge` and upserts through `recordPaymentIntent`.
- Why this design:
  - It keeps the handler's existing rule: only an id is taken from the body.
  - Retries and out-of-order deliveries end up correct, because each
    delivery writes Stripe's current state.
  - It reuses `recordMu` and the payment_token duplicate check.
  - An existing row takes the update path: no email, and `created_at` is
    kept. The model's Update touches only `updated_at`.
  - A refund for a gift that was never recorded records it (sending a
    receipt email).
- A charge with no PaymentIntent (the old Charges API) gets a warning log and
  a 200 acknowledgement, so Stripe stops retrying.
- A recording failure answers 500 so Stripe retries.

### `payment_controller/webhook_test.go`

- `TestWebhookAcksUnhandledEventTypes` now uses `customer.created`; it
  previously used `charge.refunded` as its "unhandled" example.
- New `signedEvent` and `postSigned` helpers.
- `TestWebhookRefundWithoutPaymentIntentIsAcked`: returns 200 and leaves the
  database untouched.
- `TestWebhookRefundReRetrievesAndRetriesOnFailure`: stripe-go's API backend
  is pointed at an `httptest` server (`MaxNetworkRetries: 0`, and it answers
  400 because stripe-go retries 5xx). The test asserts the request goes to
  `/v1/payment_intents/pi_test_refund` with `expand[0]=latest_charge`, that
  the payload isn't written, and that the handler answers 500. The first
  draft of this test reached the real Stripe API and got a 401; the
  `httptest` server removed that network dependency.
- `go test ./...` passes.

## Owner steps

1. Subscribe each site's Stripe webhook endpoint to `charge.refunded` in the
   Stripe dashboard. The code does nothing until then.
2. Run the reconcile tool against production:

   ```
   STRIPE_SECRET_KEY=rk_live_... go run ./test_scripts/giving_reconcile \
     -dsn "$PROD_DATABASE_URL" -tz America/Chicago -stripe-month 2026-08
   ```

   A restricted key that can read PaymentIntents and Charges is enough.
3. Refunds issued before the subscription show up as `refunded` mismatches.
   Resend those `charge.refunded` events from the dashboard, or fix them by
   hand.

## Next

Closed: None. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: N-017 (reconcile tool built and passing locally; `charge.refunded`
now handled; owner's production run and endpoint subscription remain).
Full list: `ai_docs/todo/next-list.md`.
