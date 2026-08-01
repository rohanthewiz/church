# Session: Chat + Prayer Wall Flutter Screens (church_mobile)

- Session ID: `27d26784-d74f-4bd9-8a50-c90a911b4842`
- Date: 2026-08-01 11:24
- Repos touched: `~/projs/go/church/church_mobile` only (1 commit on master).
  Core `church` untouched except this doc — the server side shipped complete
  on 2026-07-15 (see `2026-0715-1832-chat-prayerwall-modules.md`).
- Context in: `ai_docs/claude_sessions/2026-0801-0956-admin-ui-stripe-mobile-polish.md`

## Goal

Build the chat and prayer-wall screens in church_mobile against the already-
finished server API (endpoints, SSE, feature flags all live; `deleteJson` /
any-2xx groundwork landed last session).

## Server contract consumed (nothing changed server-side)

- `GET/POST /api/v1/chat/messages` (+`/:id/keep`, `DELETE /:id`) — list is
  public, pages **forward-only by `after_id` cursor** (no before_id exists;
  deliberate given 24h retention). POST 422 = moderation verdict, member-safe.
- `GET /chat/stream?channel=X` — rweb SSEHub: JSON envelope
  `{"type": chat_message|chat_delete|chat_keep, "data": {...}}` on standard
  `message` events, `:keepalive` comment every 25s, **no id:/Last-Event-ID**.
- `GET/POST /api/v1/prayer-requests` (+`/:id/answered`, `DELETE /:id`) —
  limit/offset paging; `mine` computed server-side from the opportunistic
  Bearer read; answered body `{"answered": bool, "note": string}`.
- Moderation gate = `chat.CanModerate(role)`: editor-or-above, SuperAdmin 99
  is the inverted-scale exception. Channels are hardcoded: `community`,
  `prayer-wall`, `article-<id>` (no discovery endpoint).

## Commit — church_mobile master

### Models

- `lib/src/models/chat_message.dart` — mirrors `resource/chat/hub.go`
  MessageAPI; `withKeep()` because the SSE keep event carries only {id, keep}.
- `lib/src/models/prayer_request.dart` — mirrors RequestAPI; `withAnswered()`
  because POST /:id/answered returns only {ok, id, answered}. Doc note:
  `mine` is only valid for the session it was fetched under.
- `lib/src/models/user.dart` — **`canModerate` getter: the ONE place the app
  branches on numeric role** (`role == 99 || 1..7`), mirroring
  chat.CanModerate. UI affordance only — every endpoint re-checks. Class doc
  updated (previously claimed the app never branches on role).

### ApiClient (`lib/src/api/api_client.dart`)

Eight methods: `chatMessages` (channel/limit/afterId; afterId omitted from
the query entirely when 0 — presence signals intent), `postChatMessage`,
`setChatKeep`, `deleteChatMessage`, `prayerRequests`, `postPrayerRequest`,
`setPrayerAnswered` (always sends both keys so answered=false clears a stale
note), `deletePrayerRequest`, plus `chatStreamUri()`. Chat + prayer lists
reuse the generic `ApiPage<T>` envelope (keys `messages`/`prayer_requests`).

### SSE client (`lib/src/api/chat_stream.dart`, new)

- Hand-rolled over package:http (no new dependency): `sseFrames()` is a
  **pure** byte-stream → frame parser (subset of the SSE grammar the hub
  emits: comments dropped, `event:`/`data:` fields, blank-line dispatch,
  blank line with no data dispatches nothing, CRLF tolerated).
- `ChatStreamConnection` — single-use by design (reconnect policy belongs to
  the screen, which owns backoff + backfill). 60s idle watchdog via
  `Stream.timeout` (heartbeats every 25s ⇒ 60s of silence = half-open TCP);
  connect itself also timeout-capped. `close()` for deterministic teardown.

### ChatScreen (`lib/src/screens/chat_screen.dart`, new)

- Data flow: REST is source of truth, SSE is the wake-up call. Initial load
  = newest window; every (re)connect runs an **after_id backfill loop**
  (stream has no replay); `_upsert` dedupes by id (own POST echoes via SSE).
  Missed deletes/keep-toggles are NOT backfillable — the AppBar refresh does
  a full-window swap; documented in-code.
- Reverse ListView anchors newest at bottom (no scroll bookkeeping; readers
  scrolled into history aren't yanked). `_hasOlder` renders a "Earlier
  messages have been cleared or are not shown" top row.
- Composer session-gated in place (ListenableBuilder on SessionController —
  sign-in bar ↔ TextField swap without leaving the screen). Draft is kept on
  moderation 422; the server's member-safe reason goes to a snackbar
  verbatim. maxLength 1000 = server cap.
- Long-press sheet: Copy (all), Pin/Unpin + Delete (canModerate only).
- Reconnect: exponential 2s→30s cap; double-schedule guard (an errored
  stream fires onError AND onDone); `WidgetsBindingObserver` reconnects
  immediately on app resume with fresh backoff; "Reconnecting…" banner.
- 401 anywhere → `session.handleUnauthorized()` (app-wide teardown).

### PrayerWallScreen (`lib/src/screens/prayer_wall_screen.dart`, new)

- HistoryScreen accumulate-paging pattern (manual list + load-more/retry
  tail + page-0-swap refresh) — chosen over AsyncView because rows mutate in
  place (answered toggles, withdrawals).
- **Refetches on every session change** (giving-screen listener pattern,
  captured via post-frame, removed via stored ref): `mine` flags are
  per-fetch server-side, so rows loaded signed-out are wrong once signed in.
- FAB → sign-in first if needed (LoginScreen pops true) → compose bottom
  sheet (isScrollControlled + viewInsets; title ≤120 / body ≤2000 mirroring
  server Validate; 422 renders inline, draft kept; pops the created request
  → inserted at top, server lists newest first).
- Card actions (menu only when mine || canModerate): Mark answered… (dialog
  with optional praise note, prefilled with existing), Mark not answered
  (clears note), Withdraw (mine) / Remove (moderator).
- AppBar forum icon (when features.chat) → ChatScreen on channel
  `prayer-wall` — same pairing as the web wall's discussion strip.

### Navigation + gating (`more_screen.dart`)

- More tab: Community Chat + Prayer Wall entries at the top, gated on
  app-config `features.chat` / `features.prayer_wall` (Give-tab convention).
- Tab listens to `Listenable.merge([session, config])` so late-arriving
  config reveals the entries without a rebuild elsewhere.

### Tests (36 total, all pass; `flutter analyze` clean)

- `test/api_client_test.dart` — new `chat` + `prayer wall` groups: envelope
  parsing, after_id present/absent, POST body shapes, 422 reason verbatim,
  keep boolean body, DELETE tolerating empty 204, answered-clears-note.
- `test/chat_stream_test.dart` (new) — sseFrames: hub frame + keepalive
  ignore, **reassembly across chunks split mid-field**, multi-frame +
  multi-line data join, empty-dispatch rule, CRLF.
- Gotcha: MockClient's `http.Response` defaults to Latin-1 — canned bodies
  with non-ASCII (em dash in a moderation message) need
  `content-type: application/json; charset=utf-8`.

### README

- Roadmap Phase 3 marked shipped (chat + prayer wall); FCM push noted open.

## Verification done

- `flutter analyze` — no issues. `flutter test` — 36/36 pass.
- Contract fidelity by construction: handlers read line-by-line
  (`resource/chat/api_rweb.go`, `resource/prayerwall/api_rweb.go`,
  `hub.go`, rweb `sse_hub.go` wire format) before writing the Dart mirrors.

## Follow-ups / next steps

- **On-device smoke against a live server** — SSE path is unit-tested but
  not yet exercised end-to-end from the app (seeded users `chat-tester` /
  `chat-editor` + seeder in `test_scripts/seed_chat_test_users` from the
  0715 session; emulator API base `http://10.0.2.2:<port>`).
- FCM push (chat is foreground-only); Apple Pay entitlements; physical-
  device audio; live Stripe smoke — all still open from prior sessions.
- If deep history in chat ever matters, the server needs a `before_id`
  branch in RecentMessages (the app deliberately doesn't fake it).
- Channel list is hardcoded; a discovery endpoint would unlock per-article
  discussion screens in the app (`article-<id>` channels already work
  server-side).
