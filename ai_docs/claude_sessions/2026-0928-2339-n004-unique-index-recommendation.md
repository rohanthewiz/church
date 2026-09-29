# N-004: unique index on charges(payment_token) — recommendation

Session ID: `94221085-fab0-4a83-8d9b-13d285c765ba`
Date: 2026-09-28

## Goal

N-004 (a `UNIQUE INDEX on charges(payment_token)` as a DB-level backstop to
the charge-recording mutex) had been left as "the owner's call" on
2026-09-21. This session reviewed the code and deployment to give that call a
concrete recommendation. No code changed.

## Findings

- `recordMu` (`payment_controller/payment_recorder.go`) serializes the
  lookup-then-upsert in `recordPaymentIntent` process-wide. It covers every
  writer as long as each database has exactly one app process writing it.
- That holds for every deployment today:
  - **bytdb** is embedded and single-writer; `deploy/k8s/sites/*.yaml` pin
    `replicas: 1` with `Recreate` (the README warns two pods corrupt the file).
  - **Postgres** sites run one app process per database.
- The index only adds protection when several app processes share one
  Postgres — a shape that doesn't exist.

## Costs of adding the index

1. **Postgres migration** fails on any live site holding duplicate or
   repeated empty tokens (legacy data unknown; the dev DB has no charges). A
   skip-if-dirty `DO` block would avoid the failure but leave the index
   silently absent.
2. **bytdb cutover** (newly found): `test_scripts/pg_to_bytdb` brings the
   destination up through the production schema bootstrap (`db.InitDB` →
   `ensureBytDBSchema`) and aborts on any failed insert. A unique index in the
   bytdb `charges` tableDef would block a site's Postgres→bytdb migration on
   those same duplicates.
3. **Existing bytdb files** would never get it: `ensureBytDBSchema` only
   creates missing tables, so index-level upgrade machinery would be needed.
4. **Recorder change**: with the index, the losing insert returns a unique
   violation instead of writing a duplicate; `recordPaymentIntent` would need
   a violation → lookup + update fallback, or the receipt page shows an error.

## Recommendation

Close N-004 as won't-do. Reopen if a site ever runs more than one app process
against a single Postgres (e.g. replicas behind managed Postgres). The fix
then is the index plus the unique-violation fallback, preceded by a
duplicate check (`test_scripts/giving_reconcile` already has the Postgres
connection for it).

The recommendation was recorded in N-004's text; the item stays in Open until
the owner decides.

## Next

Closed: None. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: N-004. Full list: `ai_docs/todo/next-list.md`.
