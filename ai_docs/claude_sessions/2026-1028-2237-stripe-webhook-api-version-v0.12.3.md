# Stripe webhook: accept events at any API version; tag v0.12.3

Session: `0f86c11f-53c1-4bbd-a541-972521b78add` (run from ccswm; its doc
`2026-1028-2216-stripe-giving-smoke-test` has the full test-mode run)

## Ask

ccswm's N-007, the first run of web giving against Stripe (test mode),
found that `/webhooks/stripe` rejected every event (ccswm N-008). The user
chose to fix it in church, then tag `v0.12.3` and re-pin both sites.

## Finding

`webhook.ConstructEvent` (stripe-go v86) rejects any event whose
`api_version` is not on the library's release train, `dahlia`
(`2026-06-24.dahlia`). The check compares only the part after the dot
(`isCompatibleAPIVersion`). Stripe renders each event at the endpoint's
pinned version, or the account default if none is pinned:
- CCSWM's account default is 2018-11-08 (no train), so it fails.
- `stripe listen --latest` sends 2026-09-30.endive, which also fails.

The handler answered 400 on both `payment_intent.succeeded` and
`charge.refunded`. A production endpoint not pinned to `*.dahlia` would
never have recorded refunds or closed-tab gifts. Every stripe-go upgrade to
a new train would have broken it again.

## Change (`payment_controller/payment_webhook.go`)

- `webhook.ConstructEventWithOptions(..., ConstructEventOptions{IgnoreAPIVersionMismatch: true})`.
  The signature check is unchanged, and a zero `Tolerance` still means the
  default 300s window. v86's `webhook` subpackage has no
  `WithIgnoreAPIVersionMismatch`; that option belongs to the root
  package's `ConstructEvent`.
- The event body is decoded into `eventIDs` (`id`, and `payment_intent` as
  `*stripe.PaymentIntent`, which decodes a bare id string), not into a full
  `stripe.PaymentIntent` / `stripe.Charge`. The handler only ever used the
  ids and re-fetches the intent through the API at the library's version.
  Decoding only ids answers stripe-go's warning that foreign-version
  objects may be incorrectly deserialized.

## Tests (`payment_controller/webhook_test.go`)

- `signedEventAt(version, typ, obj)` (signedEvent at a chosen version), and
  `refusingStripeAPI(t)`, the fake Stripe server extracted from
  `TestWebhookRefundReRetrievesAndRetriesOnFailure`.
- `TestWebhookAcceptsOtherAPIVersions`,
  `TestWebhookOtherAPIVersionStillNeedsSignature` and
  `TestWebhookRefundAtOtherAPIVersionReRetrieves` run at 2018-11-08 and
  2026-09-30.endive. The refund body is 2018-shaped (a `refunds` list and a
  card `source`).
- With the handler change stashed, the two acceptance tests fail with the
  same 400. With the change, `go test ./...` passes, and so do
  `payment_controller` and `resource/payment` with `CHURCH_TEST_PG_DSN` and
  `CHURCH_TEST_PG_REQUIRED` set (bytdb and Postgres).
- End to end on ccswm, built through go.work against this checkout, with
  real `stripe listen` deliveries at 2018-11-08:
  - `payment_intent.succeeded` → 200.
  - A partial refund, then the rest, each → 200; the row went to 177, then
    777/`refunded`.

## Release

Tagged `v0.12.3` and re-pinned ccswm and cema (N-019).

## Next

Closed: N-065. Declined: None. Raised: N-065. Deferred: None. Promoted: None.
Moved: None. Updated: N-019. Full list: `ai_docs/todo/next-list.md`.
