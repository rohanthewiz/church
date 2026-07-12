# Church Platform Analysis — Architecture, Features, Testing, Mobile

Date: 2026-07-11
Scope: `church` (Go backend/CMS framework) + `church_mobile` (Flutter app).
Focus: improvement opportunities, with mobile friendliness and mobile-app interop as the top concern.

## TLDR

The mobile foundation is in better shape than expected — the `/api/v1` JSON API (Phase 1)
is implemented and its contract matches the Flutter client's models exactly. The big mobile
gaps are: no auth/token system (sessions are cookie-based and stored in-process, so they
can't serve mobile), no mobile-usable giving endpoint, and — the single most impactful web
finding — **the master layout emits no viewport meta tag**, so the server-rendered site
renders as a shrunken desktop page on every phone. Testing is nearly absent (7 pure unit
tests across 151 Go files) and the global DB singleton is what's blocking more.

---

## Mobile interop with church_mobile (top concern)

**Where it stands.** The Flutter app is a Phase-1 scaffold: a clean, well-commented API
client and models (`lib/src/api/api_client.dart`, `lib/src/models/`), but `main.dart` still
renders "Hello World!" — no screens, no state management, no audio player. On the Go side,
all seven endpoints the app consumes exist and are wired in `router_rweb.go:114-121`:
sermons list/detail (with `year`/`teacher`/`ref` filters), articles, events (with recurrence
expansion), and the `/api/v1/feed` home-screen aggregate. The contract was cross-checked:
snake_case keys, list envelopes (`{"sermons":[...]}`), the `{"error": "..."}` shape, and the
recurrence object (`freq`/`weekday`/`week`/`until`/`desc`) all line up between
`resource/*/api_rweb.go` and the Dart models. Good discipline already present: dedicated API
DTOs decoupled from SQLBoiler models, empty arrays serialized as `[]` not `null`,
hard-capped `limit`.

**Interop gaps, in priority order:**

1. **No mobile auth.** `POST /auth` (`auth_controller/auth_controller_rweb.go:33`) is
   form + cookie + 303-redirect; there is no token endpoint and no Bearer handling anywhere.
   Sessions live in the in-process `core/kvstore` — every deploy invalidates all sessions,
   and it cannot back durable mobile tokens. Phase 2 of the existing plan
   (`ai_docs/plans/2026-0707-mobile-app-flutter-api-plan.md`) already calls for DB-backed
   tokens + an `APIGuard`. This is the prerequisite for any personalized mobile feature.
2. **Giving isn't mobile-usable.** `POST /payments/create-intent` returns JSON but reads
   urlencoded form values, requires the web CSRF token, and sits inside the session
   middleware group (`router_rweb.go:141-148`). Mobile needs a JSON, token-authenticated
   variant, plus a `GET /api/v1/app-config` endpoint for the Stripe publishable key and
   feature flags.
3. **Error-shape leaks.** Handlers use `apiv1.Error` → `{"error": msg}` for 400/404, but
   infrastructure failures `return serr.Wrap(err)` raw (e.g. `resource/sermon/api_rweb.go:109`),
   so a DB outage returns a non-JSON 500 the Dart client reports as generic "Request failed".
   Related: the audio endpoint returns HTTP **501** for a missing file (`router_rweb.go:167`)
   — should be 404.
4. **Pagination has no `total`/`has_more`** — the client can only detect the end of a list
   by receiving a short page. Adding `has_more` (fetch limit+1) makes infinite scroll reliable.
5. **Client hardening (Flutter side):** every `id` is a hard `as int` cast (throws on
   drift); no request timeouts; a `SocketException` offline propagates uncaught; no caching
   layer. `api_client.dart:30` still has the TODO for the production base URL; iOS ATS will
   require HTTPS.
6. **No CORS on `/api/v1`.** Irrelevant for the native app, but forecloses a future PWA/web
   consumer. The audio route already has good CORS + Range support
   (`basectlr/send_file_rweb.go:84-90`) — mirror on the API group when needed.

**Suggested next mobile milestones:** (a) auth tokens table + `POST /api/v1/auth/login|logout`,
`GET /api/v1/auth/me` with a Bearer `APIGuard`; (b) `app-config` endpoint + JSON
create-intent for giving; (c) on the Flutter side, pick state management, build the
feed/sermons screens, and add `just_audio` — the backend's Range-enabled audio streaming is
already ready for it.

## Mobile friendliness of the web site

- **Add `<meta name="viewport" content="width=device-width, initial-scale=1">` to
  `template/page.html.go` (head block).** It is missing entirely, so phones render at
  ~980px and scale down. This one line is the highest-impact change in this review.
- Exactly **one media query exists in the codebase** — the event form
  (`resource/event/module_event_form.go:91`), which correctly collapses its CSS grid to one
  column under 640px. It is a good template; the three-column `#left-side`/`#main`/`#right-side`
  layout and the rest of the modules have no responsive rules at all. (Caveat: the primary
  `app.css`/`bootstrap_scoped.css` live in the gitignored `dist/` build output and could not
  be inspected — but without the viewport tag they can't help anyway.)
- The home-grown grid (`grid/assets.go`) is halfway there: flexbox toolbar, `overflow:auto`
  scroll container for wide tables — but no media queries, 0.75-0.92em fonts, and small tap
  targets (pager buttons, caret). A `@media (max-width:640px)` block bumping touch targets
  would go a long way.
- The JS payload is heavy for mobile: jQuery **2.1.4** from CDN, fullcalendar, slick,
  summernote, moment.js all load in the master layout. jQuery is already dropped from the
  grid — continuing that trend (and loading summernote/bootstrap only on admin pages, which
  is partially done) would help first paint on phones.

## Architecture

The core design is solid — the resource-package layout (presenter / queries / modules / api
per domain), the module registry + JSONB page composition, and the dedicated API DTO layer
are all clean. The issues are mostly discipline leaks:

- **`db/connect2.go` has a real bug**: line 18 discards the `serr.Wrap` result, so `InitDB2`
  silently swallows its init error. The file is also a copy-paste of `connect.go` — either
  de-duplicate or fix and document why two singletons exist.
- **Duplicated helpers**: `RedirectRWeb` exists in both `app` and `auth_controller`;
  `IsLoggedInRWeb` exists in both `app` (`application_controller_rweb.go:54`) and `basectlr`
  (`base_controller_rweb.go:79`) *with different logic* (one checks the `isAdmin` flag, the
  other `sess.Username`) — a latent auth-display bug. Consolidate into `app`.
- **Layering violation**: `admin_controller_rweb.go:21-68` imports `db`+`models` and inserts
  events directly, bypassing the resource layer every other controller uses. Also, the
  `api_rweb.go` files duplicate query logic instead of reusing their package's `*_queries.go`
  — one query path per resource keeps HTML and JSON views consistent.
- **Fat write handlers**: `UpsertSermonRWeb` (`sermon_controller_rweb.go:80-199`) does form
  binding, multipart file I/O, *and* spawns a fire-and-forget goroutine that sleeps 45s then
  uploads to IDrive — error only logged, closure captures the outer `err`, work lost on
  restart. Extract file handling into `resource/sermon` and replace the goroutine with a
  small managed upload queue with retry.
- **Security-adjacent items worth fixing soon**: failed logins **log the plaintext password**
  (`auth_controller_rweb.go:45-46`); `/debug/set|show|clear` routes are unguarded and always
  registered (`router_rweb.go:83-100`); admin deletes are `GET`s (CSRF-trivial,
  prefetch-unsafe) despite CSRF machinery already existing via `app.VerifyFormToken`; panic
  recovery in `basectlr` is disabled outside production (`base_controller_rweb.go:16-23`).
- Smaller: stray `fmt.Println`s in production paths (router, sermon queries/presenter, menu
  controller) bypass the structured logger; the registry filters admin modules by
  substring-matching type names (`modules_registry.go:114-118`) — an explicit flag would be
  sturdier; vestigial Redis: config commented out and kvstore in use, yet `cema/main.go:53`
  still calls `roredis.InitRedis`.

## Testing

7 test files for 151 Go files, all pure unit tests — zero `httptest`, zero DB mocking, zero
handler coverage. What exists is good (recurrence math, TLS hot-reload, kvstore concurrency,
scrypt, grid rendering, the sermon DTO mapper), but the highest-risk code is untested:
auth/login flow, payments (`ChargePresenter.Upsert`, Stripe webhook), module rendering, and
every query.

The structural blocker is that **`db.Db()` is a package-level singleton** called from inside
every query function (`resource/sermon/sermon_queries.go:17,75,...`), so nothing DB-adjacent
can be tested without a live Postgres. The fix is mechanical: have query functions accept a
`boil.ContextExecutor` parameter (callers pass `db.Db()`), which unlocks both `sqlmock` and
transaction-rollback tests. After that, the best-value additions in order:

1. `httptest` handler tests for `/api/v1/*` — this is the **mobile contract**; table-driven
   tests asserting the exact JSON shapes catch any drift the Flutter app's hard `as int`
   casts would otherwise turn into runtime crashes.
2. Auth flow tests: login success/failure, cookie issuance, `AdminGuardRWeb` redirect.
3. Stripe webhook signature/idempotency tests.
4. Move `init()`-time random-seed loading behind an explicit call, removing the
   committed-fixture hack the existing tests need.

## Feature opportunities

Beyond the mobile Phase 2/3 items (tokens, giving, app-config, push/chat per the plan doc):
`has_more` pagination, a search endpoint (sermon `teacher`/`ref` filters exist;
articles/events have none), RSS/podcast feed for sermons (audio + metadata already exist —
a `/podcast.xml` is nearly free and gets sermons into podcast apps), and image
variants/resizing for mobile bandwidth (article/sermon images currently ship at full size).

## Sequencing recommendation

1. Viewport meta tag + wrap API infra errors in the JSON error shape (an afternoon,
   outsized payoff). — **DONE 2026-07-11**: viewport added to `template/page.html.go`;
   `apiv1.ServerError` added and wired into all /api/v1 handlers; sermon-audio missing
   file now 404 (was 501).
2. API contract tests via `httptest` (locks the mobile interface before it grows). —
   **DONE 2026-07-11**: in-process contract tests (rweb `Server.Request` + go-sqlmock via
   `db.SetHandleForTesting`) in `resource/{sermon,article,event,feed}/api_contract_test.go`
   and `resource/apiv1/apiv1_test.go`; shared plumbing in `resource/apiv1/apitest`.
3. Phase 2 auth: token table, Bearer guard, `/api/v1/auth/*` — unblocks everything
   personalized on mobile. — **DONE 2026-07-11**: `api_tokens` table (migration
   `20260711150000`, SHA-256-hashed tokens, 30-day TTL), `resource/apitoken`
   (hand-written SQL per the recurrence precedent), `POST /api/v1/auth/login`
   (JSON + form fallback, per-IP+username failed-login throttle),
   `GET /auth/me`, `POST /auth/logout`, `APIGuard` Bearer decorator, contract
   tests + live smoke (`test_scripts/auth_live_check`).
4. The security quartet: plaintext-password log, `/debug/*` guard, POST-based deletes,
   `connect2.go` swallowed error. — **DONE 2026-07-11**: password removed from the
   failed-login log; `/debug/*` behind AdminGuard; all six admin deletes are POST +
   CSRF form token (grid delete links now submit a POST form via `data-csrf`);
   `InitDB2` returns its error.
5. Responsive pass: media queries for the 3-column layout and grid, using the event form as
   the pattern. — **DONE 2026-07-12**: framework-level `template.ResponsiveCSS`
   (template/responsive_css.go, inlined after app.css so every site gets it without a
   stylus rebuild) stacks #left-side/#main/#right-side inside the #mid scroller under
   768px with main content first (flex order), and neutralizes the site CSS's
   min-width:800px; grid/assets.go gained a 640px block (16px control fonts to stop
   iOS focus-zoom, 44px touch targets, wrapping toolbar).
6. Executor-injection refactor for queries, then auth/payment tests. — **DONE 2026-07-12**:
   `db.Executor` interface (db/executor.go; method-set-identical to vattle's
   boil.Executor so it passes to generated models without adapters). All query/presenter
   functions in resource/{sermon,article,event,user,payment,apitoken,menu}, page, and
   core/idrive's sermon cache now take the executor as first param; db.Db() is fetched
   only at boundaries (handlers, module getData, background services, bootstrap).
   New tests: web auth flow (login success/failure/oracle-free unknown user, session
   cookie passes AdminGuard, anonymous + stale-cookie redirects) in auth_controller;
   ChargePresenter.Upsert insert/update/validation + FindChargeIdByPaymentToken in
   resource/payment (direct sqlmock injection — no global swap); webhook signature
   gate (503 unconfigured / 400 forged / 200 ack with a real HMAC-signed payload) and
   recordPaymentIntent idempotency in payment_controller.

Also done 2026-07-12: `GET /api/v1/app-config` (mobile milestone (b) part 1) —
resource/apiv1/appconfig.go returns church_name, theme, stripe_publishable_key,
giving_contacts (never null), features {giving: both Stripe keys present,
sermon_audio: idrive enabled}, server_version; DB-free by design; contract tests in
resource/apiv1/appconfig_test.go. Remaining for milestone (b): JSON create-intent.
