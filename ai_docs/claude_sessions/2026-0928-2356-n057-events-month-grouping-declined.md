# N-057: month sub-grouping for the events list — declined

Session ID: `94221085-fab0-4a83-8d9b-13d285c765ba`
Date: 2026-09-28

(Earlier in this session: `2026-0928-2339-n004-unique-index-recommendation`,
`2026-0928-2347-n005-theme-vars-material-form`,
`2026-0928-2351-n009-postgres-import-webhook-tests`,
`2026-0928-2354-n021-mobile-can-moderate`,
`2026-0928-2355-n022-chat-moderate-seed-check`.)

## Question

N-057 (optional, from `2026-0928-0100-sermons-group-by-month`): set
`GroupByMonth: true` on the events list's "Event Date" column
(`resource/event/module_events_list.go:90`), as sermons have. No code
changed.

## Findings

- Mechanically a one-liner: the column is already `grid.ColDate` with
  `GroupBy: true`, same as sermons' "Date Preached".
- **The default-open month would be wrong for events.** The grid's collapse
  default opens the first year and the first month of each open year
  (`grid/assets.go:277`). List modules sort `DESC` unless the placement sets
  Ascending (`module/module_presenter.go:69`, events query
  `event_date <order>`). Sermons are all past, so first = latest month =
  what visitors want. Events include future dates, so first = the
  furthest-future month; a September visitor would see e.g. December open and
  the current month collapsed. Year-only grouping (today) leaves the whole
  current year open.
- **The item's own trigger isn't met:** no evidence of long year groups, and
  the list pages server-side, which keeps groups short. Live event volume
  couldn't be checked (site unreachable, church_mobile N-001).

## Outcome

Declined → Non-goals. The way back, if ever wanted: make the grid open the
month containing today (or the nearest upcoming one) instead of the first
month — a small JS change in `grid/assets.go` that sermons would tolerate
(their current month is normally their first) — then flip the flag. Roadmap
was offered as the alternative; recorded as declined per the recommendation.

## Next

Closed: None. Declined: N-057. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
