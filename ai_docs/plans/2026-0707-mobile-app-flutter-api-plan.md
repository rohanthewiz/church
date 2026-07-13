# Church Mobile App — Framework Choice + API Plan

Date: 2026-07-07
Companion repo: `church_mobile` (sibling of `church/`, Flutter app)

## Framework decision: Flutter

Chosen over React Native and Kotlin Multiplatform because:

- **Sermon audio is the killer feature.** `just_audio` + `audio_service` give background
  playback, lock-screen controls, and HTTP Range streaming/seek. The server side is already
  mobile-ready: `basectlr.SendAudioFileRWeb` serves 206 Partial Content with `Accept-Ranges`
  and exposes the `Range` CORS headers.
- **Payments align exactly.** `flutter_stripe`'s PaymentSheet consumes a PaymentIntent
  client secret — the same flow the server adopted in the PaymentIntents migration
  (`POST /payments/create-intent`, webhook records completion).
- No existing JavaScript investment anywhere in the stack (server renders HTML via
  `element`), so React Native's main advantage doesn't apply. KMP ≈ double UI work.

Key Flutter packages: `just_audio`, `audio_service`, `flutter_stripe`, `flutter_html`
(article bodies), `firebase_messaging` (push, Phase 3), `dio` or `http` (API client),
`flutter_secure_storage` (token storage).

## Current server API surface (as of 2a8f896)

Everything renders HTML except:

| Endpoint | State |
|---|---|
| `GET /api/v1/sermons` | Thin: no `id`, no pagination/search, `scripture_refs` joined to comma string (`resource/sermon/api_rweb.go`) |
| `GET /calendar` | JSON but FullCalendar-shaped (built for the web widget) |
| `GET /sermon-audio/:year/:filename` | Mobile-ready (Range/206 verified) |
| `POST /payments/create-intent` | Reusable; needs JSON-friendly variant |
| `POST /auth` | Form-post + redirect + cookie — unusable from mobile |

## Target API surface (`/api/v1`)

### Phase 1 — read-only (no auth required)
- `GET /api/v1/sermons` — add `id`, pagination (`limit`/`offset` or cursor), filters
  (year, teacher, scripture book), `scripture_refs` as JSON array, absolute `audio_url`.
- `GET /api/v1/sermons/:id`
- `GET /api/v1/articles?limit&page` + `GET /api/v1/articles/:id` — Summernote HTML bodies;
  app renders with `flutter_html` initially (webview fallback if markup breaks).
- `GET /api/v1/events?from&to` + `GET /api/v1/events/:id` — clean DTO, not FullCalendar shape.
- `GET /api/v1/feed` — one call for app home screen: latest articles + newest sermons +
  upcoming events.
- BlueLetterBible: client-side. App parses `scripture_refs` ("John 3:16") and deep-links to
  `https://www.blueletterbible.org/kjv/john/3/16`. Server work = expose refs as array (done
  in Phase 1 sermons DTO). Later: link refs inside article HTML server-side.

### Phase 2 — auth + payments
- `POST /api/v1/auth/login` → `{token, user}`; `GET /api/v1/auth/me`; `POST /api/v1/auth/logout`.
- **Tokens must be DB-backed** (goose migration + sqlboiler regen), NOT the in-process
  kvstore — kvstore evaporates on deploy; mobile tokens should last weeks.
- `APIGuard` Bearer middleware parallel to `AdminGuardRWeb`.
- **DTO layer required first**: `user.Presenter` currently leaks credential fields
  (README backlog item). Never serialize presenters directly.
- Registration: proper `POST /api/v1/auth/register` with email verification. Do NOT
  resurrect `RegisterUserRWeb` (retired as a security loophole, commented out in
  `auth_controller_rweb.go`).
- Rate-limit / brute-force protection on login (mobile exposes it more).
- `GET /api/v1/app-config` — church name, Stripe publishable key, feature flags.
  — **IMPLEMENTED 2026-07-12** (`resource/apiv1/appconfig.go`): bare-object JSON
  `{church_name, theme, stripe_publishable_key, giving_contacts ([] never null),
  features: {giving, sermon_audio}, server_version}`. Public/unauthenticated,
  reads only config.Options (works even with the DB down). `features.giving` is
  true only when BOTH Stripe keys are configured. Contract tests in
  `resource/apiv1/appconfig_test.go`.
- JSON variant of create-intent + `GET /api/v1/payments/history` (charges table).
  — **IMPLEMENTED 2026-07-12** (`resource/payment/api_rweb.go`):
  `POST /api/v1/payments/create-intent` — public (guest giving), per-IP rate limit,
  JSON body `{amount_cents (int), fullname, email?, comment?}` →
  `{client_secret, payment_intent_id, amount_cents}`; same metadata contract as the
  web flow so the webhook records completion identically (plus `source=mobile_app`).
  `GET /api/v1/payments/history` — Bearer-guarded, charges matched case-insensitively
  on the account email, `{payments: [{id, created_at, amount_cents, description,
  comment, paid, refunded, amount_refunded_cents, receipt_number, receipt_url}],
  has_more}` with limit/offset + limit+1 has_more probe. Contract tests incl. a
  stubbed Stripe backend in `resource/payment/api_rweb_test.go`.

### Phase 3 — chat + push
- New subsystem; nothing exists server-side. `chat_messages` table (+ channels later).
- Send: `POST /api/v1/chat/messages`. Receive: **SSE** via rweb `SetupSSE()` (simpler than
  WebSockets, auto-reconnect, fine at church scale). Gate behind RegisteredUser (role 9+).
- Push is a hard prerequisite for chat adoption: Firebase Cloud Messaging via
  `firebase_messaging`; device-token registration endpoint. Also unlocks "new sermon
  posted" notifications.

## Cross-cutting
- Consistent JSON error shape: `{"error": "...", "code": ...}`.
- HTTPS-only API: Stripe requires it, iOS ATS effectively requires it → deploying the
  Let's Encrypt/TLS work (commit `1635cbf`) precedes real device testing.
- API handlers live with their resource (pattern: `resource/sermon/api_rweb.go`);
  feed aggregation gets its own package.
- Roles recap: SuperAdmin 99 (yes, lowest number wins is NOT the scheme — see
  `AdminGuard`), Admin 1, Publisher 5, Editor 7, RegisteredUser 9.

## Phase 1 implementation notes (IMPLEMENTED this session, 2026-07-07)

Server (`church`, in working tree):
- `resource/apiv1/apiv1.go` — shared `ParseLimitOffset` (with hard caps) + uniform
  JSON error `{"error": msg}`.
- `resource/sermon/api_rweb.go` — rewritten: `SermonAPI` DTO (id, refs as array,
  `audio_url`), list filters `year`/`teacher`/`ref` (all bound params), detail endpoint,
  `RecentSermonsAPI` for the feed. Old `SermonsResp` (refs as comma string, no id) had no
  known consumers; left commented for reference.
- `resource/article/api_rweb.go`, `resource/event/api_rweb.go` — new list/detail +
  feed helpers. Events default to upcoming; `from`/`to` (YYYY-MM-DD) window them.
- `resource/feed/feed_rweb.go` — separate package (avoids import cycle with apiv1);
  sections degrade independently.
- `router_rweb.go` — `/api/v1` group (public reads, outside session middleware);
  Phase 2 auth becomes a Bearer guard on a sub-group.
- Contract decisions: published-only with drafts 404ing identically to missing ids;
  arrays serialize as `[]` never null; `body` only on detail endpoints; `audio_url`
  returned as stored (client resolves relative URLs against its API base).
- Tests: `resource/sermon/api_rweb_test.go` covers DTO mapping; needed the
  `cfg/random_seeds.txt` fixture copied to `resource/sermon/cfg/` (the `resource/auth`
  init() landmine — fixture is cwd-relative per test package).
- NOT yet live-tested against Postgres (was down); smoke-test `/api/v1/feed` etc.
  when next running a site binary.

## Phase 2 auth implementation notes (IMPLEMENTED 2026-07-11)

- **`api_tokens` table** (migration `20260711150000`): SHA-256 hex of the token
  (plaintext exists only in the login response), FK to users with CASCADE,
  `device` label, `last_used_at` touch, fixed 30-day `expires_at`. Hand-written
  SQL in `resource/apitoken` — no SQLBoiler regen, same precedent as
  event_recurrences.
- **Endpoints**: `POST /api/v1/auth/login` (JSON `{username,password,device?}`;
  urlencoded fallback) → `{token, expires_at (RFC3339), user}`; `GET
  /api/v1/auth/me` → `{user}`; `POST /api/v1/auth/logout` → `{"ok":true}`
  (revokes only the presented token). The user DTO (`apitoken.APIUser`) is
  credential-free: id, username, first_name, last_name, email, role, role_name.
- **`APIGuard` is a per-handler decorator, not group middleware** —
  rweb group middleware auto-continues into the handler when middleware returns
  nil without calling Next(), so a 401-writing middleware would double-write the
  body. `api.Get("/auth/me", apitoken.APIGuard(apitoken.APIMeRWeb))`.
- Guard resolves token→user in one JOIN (expiry + `users.enabled` checked in the
  query), stashes `TokenUser` + token hash in ctx; unknown/expired/disabled all
  present as the same 401.
- **Login throttle**: in-process sliding window, 10 failures / 15 min per
  (client IP, username); success clears. Unknown-user and wrong-password answer
  with the identical 401 message (no username oracle).
- `user.AuthUserByUsername` (resource/user) is the login lookup — enabled users
  only, no-rows = found=false, credentials never leave the resource layer except
  into the scrypt comparison.
- `apitoken.RevokeAllForUser` exists for password change / account disable /
  "log out everywhere" (not yet wired to the admin user form).
- Tests: `resource/apitoken/api_contract_test.go` (sqlmock; login shapes,
  throttle, guard rejections, logout revocation targeting the exact hash) +
  `apitest.RequestJSON` helper (method/headers/body). Live end-to-end:
  `go run ./test_scripts/auth_live_check` (verified 2026-07-11 against local PG).
- Flutter TODO: token store (`flutter_secure_storage`), login screen, attach
  `Authorization: Bearer` in api_client.dart, re-login on 401.
  → DONE 2026-07-12; see "Flutter screens" section below.

## Flutter screens (IMPLEMENTED 2026-07-12, church_mobile)

Full screen set landed in `church_mobile` — the app now covers Phases 1+2
end-to-end against the server API:

- **Shell** (`lib/src/screens/shell.dart`): bottom-nav Home / Sermons /
  Events / Give / More on an IndexedStack (tabs stay alive so sermon audio
  keeps playing across tab switches). Articles live inside Home ("See all")
  rather than a sixth tab.
- **Services** (`lib/src/app_services.dart`): plain InheritedWidget
  (`AppScope`) carrying ApiClient + SessionController + a
  `ValueNotifier<AppConfig?>` — deliberately no provider/riverpod.
- **Session** (`lib/src/session/session.dart`): flutter_secure_storage for
  the bearer token (Keychain/EncryptedSharedPreferences) with the user DTO
  cached alongside; login persists *before* activating the token; restore
  trusts the cache instantly then revalidates via /auth/me in the background
  (only a definitive 401 logs out — offline never does); guarded screens
  call `handleUnauthorized()` on 401 so the whole app flips at once.
- **ApiClient** extended: appConfig/login/me/logout/createPaymentIntent/
  paymentsHistory, Bearer header when a token is set, `_postJson`, 401 →
  `ApiException.isUnauthorized`.
- **Screens**: Home (one /feed call, three sections), Sermons
  (infinite-scroll offset paging + year chips), SermonDetail (just_audio
  inline player: Range-seek slider, ±10/30s, 1x-2x speed cycler; scripture
  chips deep-link BlueLetterBible), Articles/ArticleDetail + Events/
  EventDetail (flutter_html bodies, tel/mailto links, month grouping,
  recurring badges), Login (autofill hints, device label), Giving
  (flutter_stripe PaymentSheet: create-intent → initPaymentSheet →
  presentPaymentSheet; guest-friendly, session prefill, quick-amount chips,
  dollars→integer-cents parser, feature-flag fallback showing
  giving_contacts), History (has_more paging, receipt links, 401 teardown),
  More (account, sign in/out, about w/ server_version).
- **Stripe native config**: MainActivity → FlutterFragmentActivity;
  both `styles.xml` themes → Theme.MaterialComponents (stripe_android
  requirement). Publishable key set from app-config at boot (and re-checked
  on the Give tab, which owns the config retry path).
- **Tests**: `test/api_client_test.dart` — MockClient contract tests
  (login shape + JSON body, Bearer-header presence, 401 mapping,
  create-intent integer-cents + omitted empty optionals, has_more envelope,
  app-config parse). `flutter analyze` clean; all tests green.
- Deferred: audio_service (background/lock-screen playback), per-device
  session management UI, calendar month view, push (Phase 3).

## Event recurrence (IMPLEMENTED this session, 2026-07-07)

Supports "every Sunday" (weekly) and "the Nth/last <weekday> of the month"
(monthly, week 1..4 or -1=last) — e.g. "every second Saturday", "last Sunday".

- **Table `event_recurrences`** (migration `20260707130000`): 1:1 with events,
  `ON DELETE CASCADE`, CHECK constraints on freq/weekday/week. Hand-written SQL
  (`resource/event/recurrence_queries.go`) — deliberately NO SQLBoiler model,
  same precedent as sermon_cache_access, so no legacy-toolchain regen.
- **Engine** `resource/event/recurrence.go`: pure `Recurrence.Occurrences(anchor,
  from, to)` — anchored at the event's event_date (occurrences strictly after it;
  the base row represents its own date), optional `until`, date math on yyyymmdd
  ints to dodge timezone drift. `Describe()` renders "Last Sunday of each month".
  Unit tests cover second-Saturday/last-Sunday against hand-verified 2026 dates.
- **`event.WindowedEvents(from, to)`** is the single expansion point shared by
  `/api/v1/events`, the feed, and the website FullCalendar endpoint (which now
  also gets a published=true filter it previously lacked, and paramized date
  parsing). Occurrence entries share the base event's id; `recurring` +
  `recurrence_desc` on all list entries, structured `recurrence` object on detail.
  Listing defaults to a 92-day window (`DefaultWindowDays`) — expansion needs a
  finite horizon; paging is applied post-expansion in memory (base query capped
  at 500 with a logged warning).
- **Admin form** gained Repeats / On <weekday> / Week (monthly) / Repeat-until
  selects; rule syncs in `UpsertEvent` (delete on "None"). Loaded via
  `Presenter.LoadRecurrence` only on single-event edit (avoids N+1 in lists).
- **Verified live**: brew `postgresql@16` started locally, devuser +
  church_development created, ALL migrations applied via pressly goose
  (`~/go/bin/goose`), constraints + cascade validated by SQL, and
  `test_scripts/recurrence_live_check/main.go` seeded weekly/monthly/one-time
  events and printed the correctly expanded Jul-Oct 2026 window.
  NOTE: this local Postgres is fresh (only migrated schema, no content); the
  real dev data was not present on this machine. `brew services stop
  postgresql@16` to turn it off.
- Flutter side mirrored and pushed: `ChurchEvent.recurring/recurrenceDesc` +
  `EventRecurrence` model.

Mobile (`church_mobile`, pushed to github.com/rohanthewiz/church_mobile, private):
- Flutter --empty scaffold (iOS/Android), `http` dep.
- `lib/src/api/api_client.dart` — one method per Phase 1 endpoint, `API_BASE`
  dart-define, uniform ApiException, `resolveMediaUrl` for relative audio URLs.
- `lib/src/models/` — Sermon (incl. `blbUrlFor` BlueLetterBible deep-link parser),
  Article, ChurchEvent (named to avoid dart Event collision), Feed.
- `dart analyze` clean. No UI yet — next: home feed screen + sermon list/player
  (just_audio + audio_service).
