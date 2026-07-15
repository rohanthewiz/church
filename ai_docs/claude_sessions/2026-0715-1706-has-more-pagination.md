# Session: has_more Pagination on the Older List Endpoints

**Session ID**: `32bed32d-73d3-4df7-a54e-9a04b5e9e27d`
**Date**: 2026-07-15
**Branch**: master (both repos)

## Goal

Execute the "has_more on the older list endpoints" next-step from the previous
session: retrofit the `has_more` envelope (introduced by payments history)
onto `/api/v1/sermons`, `/articles`, and `/events`, and consume it in the
Flutter app. `/api/v1/feed` was deliberately excluded — it is a fixed-size
aggregator, not a paged list.

## Work Done — server (`church`)

### Sermons + Articles: limit+1 probe

- `APISermonsRWeb` / `APIArticlesRWeb` now query `qm.Limit(limit+1)`, trim the
  spare row, and add `"has_more": bool` to the envelope — same pattern as
  `APIPaymentHistoryRWeb` (no `COUNT(*)`). Existing `limit`/`offset` keys are
  unchanged, so old clients keep working.
- The sermon probe automatically respects the year/teacher/ref filters since
  the spare row comes from the same filtered query — `has_more` speaks for
  the same filter set.

### Events: in-memory length check

- Events page in memory *after* recurrence expansion (SQL offsets would count
  base rows, not occurrences), so the full window is already in hand:
  `has_more = pageEnd < len(events)`. No probe needed.
- Documented caveat: events' `has_more` only speaks for the requested
  `from`/`to` window — more *time* may exist beyond `to` even when false.

### Contract tests

- Each list contract test freezes `has_more` in the envelope and asserts
  false on an unfilled page.
- New true-case per resource: two mock rows at `limit=1` must yield one item
  and `has_more=true` (sermons/articles assert the probe row is trimmed).

## Work Done — Flutter (`church_mobile`)

- `lib/src/models/api_page.dart` — generic `ApiPage<T>{items, hasMore}` with
  `ApiPage.fromJson(json, key, itemFromJson)`. Generic rather than one class
  per resource because the shape is identical and behavior-free;
  `PaymentsPage` predates it and stays (public API of the payments model).
  Missing `has_more` (older server) reads as **false** = exhausted — fails
  closed, no extra fetch.
- `ApiClient.sermons()/articles()/events()` now return `ApiPage<T>`.
- Sermons screen: `_exhausted = !page.hasMore` replaces the short-page
  heuristic — an exactly-full last page no longer costs one wasted empty
  fetch. Articles/events screens remain deliberately single-page and unwrap
  `.items` via a small `_fetch()` helper.
- Tests: sermons envelope parse (`has_more:true` at limit=1) and
  older-server fallback (no `has_more` key → `hasMore=false`, no crash).

## Verification

- Go: full `go test ./...` green (new probe tests included).
- Flutter: `flutter analyze` clean; all 18 tests pass.
- Live end-to-end: built **cema** against the local church module with a
  scratch `go.work` (same technique as last session; cema/ccswm pin a
  published church version). cema on :8088 against DB `church_development`.
  Inserted a second published sermon (id=2 "Walking in Hope"; note
  `updated_by` is NOT NULL — inserts must supply it). Curl confirmed:
  - `?limit=1` → `has_more:true`, one sermon ("Living by Faith")
  - `?limit=1&offset=1` → `has_more:false`, one sermon ("Walking in Hope")
    — exactly-full last page correctly reports no more
  - `?limit=1&teacher=Pastor` → `has_more:true` (probe respects filters)
  - articles/events default pages → `has_more:false`
  Both test sermons left in the dev DB so the app can exercise paging.
  psql on this Mac lives at `/opt/homebrew/opt/postgresql@16/bin/psql`
  (not on PATH).

## Blocked / Next Steps

- **On-device run** (still hardware-blocked): no phone/emulator, no Xcode;
  `flutter run --dart-define=API_BASE=http://<mac-ip>:8088` when a device is
  attached. Background-audio lock-screen controls need a physical device.
- Live Stripe smoke test; browser check of the responsive pass on a running
  site binary.
