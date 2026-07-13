# Session: Flutter screens — full church_mobile app (Phases 1+2 UI)

Session ID: `24108146-5744-4e41-8bbf-9fd487878b2e`
Date: 2026-07-12

## What was asked

"Please continue with the Flutter screens" — the next step from the previous
session (JSON create-intent + payments history). All work landed in the
sibling repo `church_mobile`; this repo only gets the plan-doc update and
this session doc.

## What was built (church_mobile)

### Dependencies added
`flutter_secure_storage`, `just_audio`, `flutter_html`, `flutter_stripe`,
`url_launcher`, `intl` (via `flutter pub add`; stripe pinned itself at 13.0.0).

### Native config for flutter_stripe (Android)
- `MainActivity` now extends **FlutterFragmentActivity** — PaymentSheet is a
  native AndroidX fragment and crashes under plain FlutterActivity.
- Both `values/styles.xml` and `values-night/styles.xml` themes reparented to
  **Theme.MaterialComponents\*** (stripe_android inflates Material widgets
  against the host activity theme; the Material lib arrives transitively).
- iOS untouched (no Xcode on this machine; flutter_stripe needs iOS 13+,
  which the current Flutter template already targets).

### Services layer
- `lib/src/app_services.dart` — `AppServices` (ApiClient + SessionController
  + `ValueNotifier<AppConfig?>` for app-config) threaded via a plain
  `AppScope` InheritedWidget. Deliberately no provider/riverpod.
- `lib/src/session/session.dart` — `SessionController` (ChangeNotifier):
  - Token in flutter_secure_storage (Keychain / EncryptedSharedPreferences),
    user DTO cached alongside as JSON for instant cold-start rendering.
  - `login()` persists to storage **before** activating the token (a crash
    mid-write can't leave a live token the next launch doesn't know about).
  - `restore()` trusts the cache immediately, then revalidates via /auth/me
    in the background; only a definitive 401 tears down — offline never
    logs the user out. `logout()` revokes server-side best-effort.
  - `handleUnauthorized()` — screens that hit 401 mid-session call this so
    the whole app flips to logged-out at once.

### ApiClient extensions (lib/src/api/api_client.dart)
- New: `appConfig()`, `login()` (returns LoginResult; does NOT set the token
  itself — session's job), `me()`, `logout()`, `createPaymentIntent()`
  (integer cents; empty optionals omitted from the JSON body, presence =
  intent), `paymentsHistory()` (has_more envelope).
- Mutable `token` field; `Authorization: Bearer` on every request when set.
- `_postJson` + shared `_decode`; `ApiException.isUnauthorized`.

### Screens (lib/src/screens/)
- `shell.dart` — bottom nav **Home / Sermons / Events / Give / More** on an
  IndexedStack (tabs stay alive → sermon audio survives tab switches).
  Articles live inside Home ("See all"), not a sixth tab.
- `home_screen.dart` — one GET /feed paints three sections; AppBar title from
  app-config church_name; pull-to-refresh.
- `sermons_screen.dart` — infinite scroll (offset paging, prefetch at 200px
  from bottom, tail row doubles as page-retry) + year filter chips.
  Note: sermons endpoint predates has_more; exhaustion inferred from a short
  page until the server retrofit lands.
- `sermon_detail_screen.dart` — just_audio inline player + scripture
  ActionChips deep-linking BlueLetterBible + summary + HTML body.
- `articles_screen.dart` / `article_detail_screen.dart` — single-page list
  (church archives are small), flutter_html body.
- `events_screen.dart` / `event_detail_screen.dart` — month-grouped upcoming
  list, recurring badges w/ recurrence_desc tooltip, undated events sink to a
  "Date TBA" group; detail has tappable tel/mailto/url contact rows.
- `login_screen.dart` — website credentials, autofill hints, device label
  ("iPhone/iPad"/OS name) stored with the token, server 401/429 messages
  surfaced verbatim; pops true on success.
- `giving_screen.dart` — mirrors the web PaymentIntents flow:
  create-intent → initPaymentSheet → presentPaymentSheet (card/3DS/wallets
  all Stripe UI; completion recorded by the existing webhook — no client
  recording call). Guest-friendly; prefills name/email from session (never
  clobbers typing); quick-amount chips; "$25.50"-tolerant dollars→int-cents
  parser (no float math on money); client-side 50¢ floor mirroring the
  server; StripeException.Canceled treated as a normal exit; feature-flag
  fallback renders giving_contacts when the server has no Stripe key.
  This tab owns the app-config retry (Stripe can't init without the
  publishable key).
- `history_screen.dart` — has_more-driven "Load more", receipt_url launches
  externally, refund badging, 401 → session teardown + pop.
- `more_screen.dart` — account header, Giving history, sign in/out (confirm
  dialog), About w/ server_version from app-config.

### Widgets / audio
- `widgets/async_view.dart` — the one loading/error/data switch + ErrorRetry
  (ApiException messages are user-safe; anything else gets a generic line)
  + EmptyNote.
- `widgets/html_body.dart` — flutter_html for Summernote bodies (unified
  scrolling beats a webview for editor-produced markup); relative link/image
  URLs resolved against API base; links open externally.
- `audio/sermon_player.dart` — just_audio: stream-driven position/duration,
  play/pause/buffering morphing button, ±10/30s, 1x→1.25→1.5→2x speed
  cycler, inline load-error card with retry (dead legacy mediasave URLs).
  Per-screen player only — audio_service/lock-screen is the next layer.

### Patterns settled
- Screen fetches: `Future? _future` + `_future ??= api.x()` in build (AppScope
  needs an inherited-widget context — initState is too early); retry replaces
  the future via setState. Paged screens kick the first load from a
  post-frame callback (setState-during-build hazard).
- main.dart boot: session restore awaited (two keystore reads, prevents
  logged-out flash); app-config fire-and-forget with Stripe.publishableKey
  set on arrival; Give tab re-checks both.

### Tests (test/api_client_test.dart)
MockClient contract tests — Dart mirror of the server's api contract tests:
login response shape + JSON body sent, Bearer header present exactly when a
token is set, 401 → isUnauthorized with server message, create-intent sends
integer cents & omits empty optionals, history parses has_more, app-config
parse. 6/6 green.

## Verification
- `flutter analyze` — no issues.
- `flutter test` — 6/6 pass.
- `flutter build apk --debug` — **built successfully** (231s first Gradle
  run), proving the Dart + stripe native config compiles for a real target.
- NOT verified: iOS (no Xcode installed), live run against a server
  (`flutter run --dart-define=API_BASE=http://<mac-ip>:8000`), live Stripe
  smoke test.

## Docs / memory
- Plan doc `ai_docs/plans/2026-0707-mobile-app-flutter-api-plan.md`: Phase 2
  Flutter TODO marked done; new "Flutter screens (IMPLEMENTED 2026-07-12)"
  section.
- Memory `mobile-interop-priority` refreshed (Flutter screens done; toolchain
  gotchas: no Xcode, Android cmdline-tools/licenses warnings).

## Not done / next steps
- audio_service — background/lock-screen playback on top of the existing
  just_audio instance.
- Live smoke test against a running site binary + live Stripe test keys.
- Server side still queued: wire `apitoken.RevokeAllForUser` to password
  change/user disable; has_more retrofit on sermons/articles/events lists.
- Per-device session management UI (device labels are already stored).
- Phase 3: chat + push (firebase_messaging).
