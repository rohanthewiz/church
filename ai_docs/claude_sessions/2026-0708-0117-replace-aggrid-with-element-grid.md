# Session: Replace AG Grid with our own element-based grid component

- **Session ID**: `e979f4a0-18b6-438e-bf6e-c92f37012ffd`
- **Date**: 2026-07-08
- **Branch**: master

## Goal

AG Grid had good features but poor layout behavior. Replace it with a home-grown
component built on `github.com/rohanthewiz/element` with feature parity plus:
pagination, multi-column sorting, lazy loading, data filtering, and year grouping.

## What was done

### New package: `grid/`

- `grid/grid.go` — `Column`/`Cell`/`Grid` types and an element-based `Render(b)`
  (satisfies element's Component interface). Cell constructors `grid.Text`,
  `grid.Link`, `grid.EditLink`, `grid.DeleteLink`, `grid.HTML` replace the old
  AG Grid `label|href` string convention. All user text is HTML-escaped at
  render time (element writes strings verbatim). Rows are stamped with
  `data-year` server-side (regex on the GroupBy date column) so the JS groups
  without date parsing.
- `grid/assets.go` — `grid.CSS` and `grid.JS` string constants, inlined into
  every page by `template/page.html.go` (same slot where the AG Grid bundle +
  renderer shims used to load). The JS is vanilla (no jQuery), initializes every
  `.ch-grid` on the page independently, and provides:
  - multi-column sort: click header cycles asc/desc/off; shift-click adds
    secondary sorts (priority number shown in the header indicator)
  - per-column filter inputs + quick "search all columns" box with
    "(filtered from N)" count
  - client-side pagination with page-size selector (10/25/50/100)
  - lazy row attachment: all rows for the server page are rendered once, but
    only the visible page's `<tr>` nodes are attached to the tbody
  - collapsible year grouping (first group open, like sermon-cleanup);
    pagination suspends while grouped
  - delete links: `href="#"` + `data-url`, confirm via SweetAlert2 when present
    (`swal`) else `window.confirm`; inert without JS
  - popup cells (`Column.Popup`): click shows full content via swal
  - progressive enhancement: `.ch-grid-jsonly` controls hidden until init adds
    `.ch-grid-ready`; without JS it's a plain readable table
- `grid/grid_test.go` — unit tests: structure, escaping, server pager links,
  empty message, raw HTML + popup cells.
- Server-side paging: `Grid.Limit/Offset` render relative `?limit=&offset=`
  Prev/Next links (no-JS friendly). These params already reach the main module
  via `basectlr.RenderPageListRWeb` → `Presenter.SetLimitAndOffset`.
- Theming: all colors are CSS custom properties on `.ch-grid` (`--chg-*`), so a
  site theme can re-skin grids by overriding the variables.

### Migrated modules (Render() bodies rewritten; constructors/queries unchanged)

- `page/module_pages_list.go`
- `resource/sermon/module_sermons_list.go` (Date Preached: `GroupBy: true`)
- `resource/event/module_events_list.go` (Event Date: `GroupBy: true`)
- `resource/article/module_article_list.go` (Summary = `grid.HTML`, clamped
  in-row, full content in popup — no more base64 through JSON)
- `resource/menu/module_menus_list.go`
- `resource/user/module_users_list.go`

Small cleanups while migrating:
- users list: the old duplicate "Enabled" admin column collapsed to one
- pages list: "Page URL" is now a real link (was text + click popup)
- old rowDef structs and inline gridOptions JS deleted entirely

### Template

- `template/page.html.go`: dropped `/assets/js/ag-grid.min.js` and the agrid JS
  constant injection; now injects `grid.CSS` + `grid.JS` inline. SweetAlert2
  stays (used for confirms/popups). jQuery stays (other modules use it).

### Left in place

- `agrid/ag_grid.go` — now completely unreferenced; kept per convention of not
  deleting code. `dist/js/ag-grid.min.js` can be dropped from deploys later.
- `arch_test_scripts/grid_preview/` — Go harness that renders a 60-row sample
  grid to a standalone HTML file for browser preview.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` all pass.
- jsdom functional test against the real generated HTML: 20/20 checks pass
  (init, paging, page-size, numeric/date sort, multi-sort indicators, quick +
  column filters, empty state, grouping/collapse/expand, ungroup restores
  paging, declined delete does not navigate).
- Interactive demo published as an artifact (light/dark themed via the
  `--chg-*` override hook):
  https://claude.ai/code/artifact/9247710e-bc5a-40a4-990e-dc7da9d95c7f
- Not yet done: click-through of `/admin/sermons` and `/sermons` against a live
  Postgres/Redis stack.

## Key decisions

- Built the grid inside church (`church/grid`) rather than bumping the element
  dependency: church pins element v0.5.6 whose `components.Table` is minimal;
  the richer table lives only in the local element working copy (~14 commits
  ahead, not wired in). The grid uses `element.Builder` directly.
- Data stays embedded in the page (exactly what AG Grid did via inline JSON);
  "lazy" means lazy DOM attachment + server limit/offset links, not AJAX. The
  admin lists need drafts/edit/delete URLs which the public JSON API v1
  deliberately doesn't expose, so no new endpoints were added.
- Modeled the collapsible year groups and inline-assets pattern on
  `resource/sermoncleanup/` (the existing pure-element table precedent).

## Follow-ups / ideas

- Verify the lists in the running app; tune `.ch-grid-scroll` max-height
  (`calc(100vh - 240px)`) if page chrome differs per site.
- Consider upstreaming the grid into the element library's components once
  proven here.
- Remove `agrid/` and the ag-grid bundle from `dist/` after a deploy or two.
- Optional: AJAX "load more" instead of full-page server pager links.
