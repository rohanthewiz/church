# Session: Platform Analysis + API Error Shape + Mobile Contract Tests

Session ID: `1f59b765-43b5-46a5-9d61-e22a2c657478`
Date: 2026-07-11

## What was asked

1. Review the church codebase for improvements in architecture, features, and testing,
   with mobile friendliness and interop with `~/projs/go/church/church_mobile` (Flutter)
   as the top concern.
2. Save the analysis, then implement sequencing items 1 and 2 from it.

## Analysis (saved to `ai_docs/fable_platform_analysis.md`)

Four parallel explorations (backend architecture, /api/v1 surface, Flutter app, testing +
mobile-web). Headline findings:

- **Mobile API Phase 1 is implemented and contract-matches the Flutter client** —
  `/api/v1/{sermons,articles,events,feed}` in `resource/*/api_rweb.go` line up with the
  Dart models (snake_case keys, list envelopes, `{"error": ...}` shape, recurrence object).
- **Gaps**: no mobile auth (cookie sessions in in-process kvstore only), giving endpoint is
  form+CSRF+session bound, infra errors leaked as non-JSON 500s, no `has_more` pagination,
  no CORS on the API group.
- **Master layout had no viewport meta tag** — the whole site rendered desktop-width on
  phones. Only one `@media` query existed in the codebase (event form).
- **Testing**: 7 pure unit tests / 151 Go files; global `db.Db()` singleton blocks
  everything DB-adjacent; zero handler-level coverage.
- Architecture smells: `db/connect2.go:18` swallows its init error, duplicated
  `RedirectRWeb`/`IsLoggedInRWeb` helpers (the latter with divergent logic),
  `admin_controller` bypasses the resource layer, fire-and-forget sermon-upload goroutine,
  plaintext password logged on failed login, unguarded `/debug/*` routes, GET-based
  admin deletes.

## What was implemented

### Item 1 — viewport + JSON error consistency
- `template/page.html.go`: added `<meta name="viewport" content="width=device-width, initial-scale=1">`
  (verified it is the only `<head>` in the codebase).
- `resource/apiv1/apiv1.go`: new `ServerError(ctx, err, msg)` — logs server-side, answers
  `{"error": msg}` with 500. Every /api/v1 response, success or failure, is now JSON.
- Wired into all six raw-`serr.Wrap` returns in `resource/{sermon,article,event}/api_rweb.go`.
- `router_rweb.go`: sermon-audio missing file now 404 (was 501, which misleads clients and
  can be proxy-cached); stray `fmt.Println` → `logger.Debug`; dropped unused `fmt` import.

### Item 2 — mobile API contract tests
- `db/testing.go`: `SetHandleForTesting(*sql.DB)` — the global handle is the only test seam
  until queries accept a `boil.ContextExecutor`.
- `resource/apiv1/apitest/`: shared plumbing — in-process rweb router via `Server.Request`
  (no port), go-sqlmock behind the global handle, `GetJSON`/`WantKeys`/`WantError` helpers.
- Contract tests freezing the Flutter contract:
  - `resource/sermon/api_contract_test.go` — envelope, snake_case keys, numeric `id`,
    `scripture_refs` as array, empty arrays as `[]`, body list-omitted/detail-included,
    400/404/500 all in JSON error shape.
  - `resource/article/api_contract_test.go` — same + ISO8601 `created_at`.
  - `resource/event/api_contract_test.go` — envelope, `YYYY-MM-DD` event_date, detail
    recurrence object (`freq`/`weekday`/`week`/`desc`, `until` omitempty).
  - `resource/feed/api_contract_test.go` — all-sections-fail still 200 with three `[]`.
  - `resource/apiv1/apiv1_test.go` — `ParseLimitOffset` defaults/caps/garbage,
    `ServerError` never leaks internal error text.
- New dep: `github.com/DATA-DOG/go-sqlmock v1.5.2`.

## Gotchas learned

- SQLBoiler (vattle v2) emits `SELECT * FROM "sermons"` — not `SELECT "sermons".* ...` —
  so sqlmock regex fragments must match that form.
- `resource/article` and `resource/feed` test binaries needed the `cfg/random_seeds.txt`
  fixture copied into their package dirs (init-time seed loading in `resource/auth/random.go`;
  same workaround sermon/event/auth tests already use). Item 6 of the analysis proposes
  removing this via explicit init.
- rweb `Server.Request(method, url, headers, body)` runs the full router in-process —
  ideal for handler tests without a listener.

## State at session end

- `go build ./...` clean; `go test ./...` green (10 test packages).
- All work uncommitted at time of doc writing; committed together with this doc.

## Next steps (per analysis sequencing)

3. Phase 2 mobile auth: token table, Bearer `APIGuard`, `/api/v1/auth/{login,logout,me}`.
4. Security quartet: plaintext-password log, `/debug/*` guard, POST deletes,
   `connect2.go` swallowed error.
5. Responsive pass (3-column layout + grid media queries, event form as pattern).
6. Executor-injection refactor for queries; auth/payment tests.
