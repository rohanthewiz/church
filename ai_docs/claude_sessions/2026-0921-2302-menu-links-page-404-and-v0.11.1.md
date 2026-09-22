# Main menu links, a 404 for unknown pages, and church v0.11.1

Session id: `e54856f6-f4ae-4eaa-b13c-b5bbb8d99872`

The user asked to start the local cema. While testing it, **Articles** in the
main menu gave `500 Internal Server Error` (code `JY09-7KNR`). That turned into
three changes: fixing the menu links, a proper 404 for an unknown page slug,
and a patch release with both sites re-pinned.

## Starting cema

Followed `CEMA_LOCAL_SERVER.md` (local-only, in `.git/info/exclude`):

- Nothing was listening on 8088. The `cema` binary on disk dated from 19:42 on
  2026-09-20, before the `v0.11.0` pin, so it was rebuilt rather than trusted.
- Postgres was up (`pg_isready`), and `dbc migrate status` showed nothing
  pending.
- `GOWORK=off go build -o cema .` (the pinned `v0.11.0`), then
  `env -u DB_TYPE ./cema` as a background task, logging to the session
  scratchpad.
- Boot was clean: `config.TimeZone is America/Chicago`, no `bytdb serving`
  line, no seed-file read. `/` and `/login` returned 200 and `/admin/home` 303.

## The 500 on Articles

The log mapped the error code to
`"/pages/articles" - error: sql: no rows in result set`.

**Cause.** The bootstrapped main menu linked Articles, Sermons and Events to
`/pages/articles`, `/pages/sermons` and `/pages/events`. `/pages/:slug` is
`PageHandlerRWeb`, which looks up a DB page by slug, but bootstrap only ever
creates the `home` page (`select … from pages` showed one row). The links had
been wrong since bootstrap was first added (`3c356b1`), in both
`admin/bootstrap.go` (the menu written to the DB) and the hardwired fallback in
`resource/menu/menu_def.go`.

**Fix (`d6a2b2e`).** The three items now point at the public list routes that
already exist: `/articles`, `/sermons`, `/events`. `admin_links_test.go`
expected the old href and was updated. `bootstrapMenus` rewrites any menu whose
`updated_by` is still `bootstrap`, so cema's main menu picked the change up on
the next boot, with no data migration. A menu an admin has edited is never
overwritten, so such a site keeps its old links until someone changes them in
the menu editor.

## A 404 for an unknown page (`9ab5998`)

A missing slug returned the finder's error, so any stale link or typo under
`/pages/` showed a 500 with an error code.

- **Handler.** `PageHandlerRWeb` checks `errors.Is(err, sql.ErrNoRows)`, logs at
  debug, sets 404, and renders `page.NotFound()` through
  `RenderPageSingleRWeb`, so the visitor keeps the header, nav and footer. Any
  other error is still a 500.
- **Why one check covers both backends.** Postgres and bytdb both reach the
  finder through `database/sql`, whose `Row.Scan` reports a missing row as
  `sql.ErrNoRows` whatever the driver, and SQLBoiler's `One()` passes it
  through. `serr.Wrap` implements `Unwrap`, so `errors.Is` sees through the two
  wraps in `findPageBySlug` and `presenterFromSlug`.
- **`page/not_found.go`.** `NotFound()` builds a hardwired page, like
  `page.Home()`, with one `errormodule` module ("Sorry, we couldn't find that
  page.") in the center column. It is added with `AddModule` rather than
  through `AddModules`, because `errormodule` is not in `modulesRegistry`.
  `Published: true` is required, or `Page.Render` skips the module.
- **Chosen over** a bare 404 body (what rweb gives an unmatched route) and over
  a JSON error like the API handlers, because this is a page a visitor
  navigates to.
- **Test.** The smoke test now wires `/pages/:slug` as `ServeRWeb` does and
  checks the 404 and the message on embedded bytdb. The first run "passed" on
  status alone, because the harness had no `/pages` route and rweb's
  route-not-found also returns 404. The check therefore asserts the body too.
- **Verified live on Postgres:** `/pages/articles` and `/pages/nope` return 404
  inside the nav and footer, `/pages/home` and `/` return 200, and the log has no
  error lines. All church tests pass.

## Release

- church pushed (`313a117..757ecb2`) with CI green, then tagged **`v0.11.1`**
  (annotated) on `757ecb2` and pushed. It's a patch: two bug fixes, no API or
  config change.
- Pinned in each site, outside the workspace:
  `GOWORK=off … go get github.com/rohanthewiz/church@v0.11.1`, then `tidy`,
  `build` and `vet`, all with `GOWORK=off`. cema `f6ffb50`, ccswm `d1239f9`;
  only `go.mod` and `go.sum` changed. Both sites' CI passed on the `pinned` and
  `church-master` jobs.
- ccswm's uncommitted `.gitignore` edit (`.cats-todo`) is still left out.

The local cema was restarted twice on workspace builds (`go build`, not
`GOWORK=off`) to check each fix; that code is identical to `v0.11.1`. The user
stopped it at the end of the session.

## Found, not fixed (on the next list)

- **Calendar** in the main menu links to `/calendar`, the FullCalendar JSON
  feed, so a visitor sees `[]` (N-051).
- `bootstrapMenus` logs "refreshed uncustomized menu" for all three menus on
  every boot. It compares the stored JSONB bytes with freshly marshaled JSON,
  and JSONB normalizes them, so the two never match (N-052).
- The fallback error module in `Page.AddModules` never sets `Published`, so it
  never renders (N-053).

## Gotchas

- An error code on a 500 maps to a line in the site log: grep the log for
  `[ERR: <code>]`.
- In the smoke test harness, a 404 on a route the harness didn't register
  looks like success. Assert on the body as well as the status.
- `gopls` here runs Go 1.25.4 while `go.work` needs 1.26.1, so every edit shows
  `packages.Load` diagnostics. They're noise; `go vet` and `go test` are the
  real check.

## Commits

- church: `d6a2b2e` menu links · `9ab5998` page 404 · `757ecb2` raise
  N-051–N-053 · tag `v0.11.1` · this doc
- cema: `f6ffb50` Pin church to v0.11.1
- ccswm: `d1239f9` Pin church to v0.11.1

## Next

Closed: None. Declined: None. Raised: N-051, N-052, N-053.
Deferred: None. Promoted: None.
Updated: N-050. Full list: `ai_docs/todo/next-list.md`.
