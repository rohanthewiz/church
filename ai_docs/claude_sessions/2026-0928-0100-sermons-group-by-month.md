# Sermons list: group by month, not just year

Session ID: `75d975e9-526f-4fa1-a37c-6976f865cd5f`
Date: 2026-09-28

## Goal

The sermons list's grouping toggle grouped rows by year only. The user asked
for month grouping as well.

## Design

- The grouping lives in the shared `grid` package (used by the sermons and
  events lists), so month grouping was added there as an opt-in column flag,
  `Column.GroupByMonth`, alongside the existing `GroupBy`. Events keep the flat
  year grouping; sermons (weekly, ~50 rows a year) turn months on.
- Nested, not a separate mode: one toggle ("Group by Month") produces
  year rows, each holding month rows, each holding its sermons.
- The server stamps each row with `data-month="yyyy-mm"` (next to the existing
  `data-year`) and flags the wrapper with `data-group-month="1"`, so the JS
  never parses dates. The month regexp only accepts the ISO `yyyy-mm` prefix
  (`config.DisplayDateFormat`), because a month can't be read from an
  arbitrary format without locale ambiguity. Rows without a month still group
  by year and land in an "Other" month.
- Collapse defaults apply per level: the first year is open, and the first
  month of each open year is open. Collapse state shares one map; year keys
  (`2026`) and month keys (`2026-09`) can't collide.
- Month rows reuse the year-row class (so they inherit the tint and caret
  rotation) plus `ch-grid-month-row` for indentation and a lighter weight.

## Changes

- `grid/grid.go`: `GroupByMonth` field, `yearMonthRe`, `groupByMonth()`,
  `data-group-month` wrapper attr, `data-month` row attr, button label
  "Group by Month" when on.
- `grid/assets.go`: JS `bucket()`, `groupHeader()`, `monthLabel()` and the
  nested render branch; CSS for `.ch-grid-month-row` (desktop and ≤640px).
- `resource/sermon/module_sermons_list.go`: "Date Preached" column sets
  `GroupByMonth: true`.
- `grid/grid_test.go`: `TestRenderMonthGrouping` (off by default; attrs and
  label present when on).
- `arch_test_scripts/grid_preview/main.go`: preview mirrors sermons with
  month grouping on.

## Verification

- `go build ./...`, `go vet`, `go test ./grid ./resource/sermon
  ./resource/event` pass; the extracted grid JS passes `node --check`.
- Behavior checked in headless Chrome (`--dump-dom` on the grid preview with
  a probe script): grouping on gives year → month nesting with the newest
  year and month open; opening a second month and collapsing a year work;
  toggling off restores the flat list.
- Chrome extension couldn't open `file://`, and the served preview tab went
  unresponsive after a click, so the headless probe replaced the interactive
  check. Not yet seen in the running app.

## Next

Closed: None. Declined: None. Raised: N-057.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
