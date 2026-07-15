# Session: Chat + Prayer Wall Modules (Live Discussion, Moderation, Mobile API)

Session ID: `1d358a87-e014-4abb-91d3-1185629bd15d`
Date: 2026-07-15

## Goal

Build a chat module for logged-in users that works either as its own top-level
module or as a live discussion strip at the bottom of another module (Prayer
Wall — also created this session — or comments under a published article).
Chat messages live one day unless an editor marks them keep. Basic rule-based
moderation. Mobile (/api/v1) support throughout.

## What Landed

### resource/chat — the engine (one channel-keyed system, three placements)

- `chat.go` — package doc + `Message` type, `ValidChannel` (`^[a-z0-9][a-z0-9-]{0,62}$`),
  `CanModerate(role)` (editor-or-above; **SuperAdmin=99 breaks the
  lower-is-more-privileged ordering, so it's an explicit special case**),
  `StartRetentionSweep()` — 15-min ticker + immediate startup sweep deleting
  `keep = false` messages older than `RetentionTTL` (24h). Started in `ServeRWeb`.
- `queries.go` — hand-written SQL over `db.Executor` (same precedent as
  apitoken/event_recurrences; no SQLBoiler regen). `RecentMessages` serves both
  the initial window (newest N, reversed to ascending) and `after_id` polling.
- `moderation.go` — transparent rule pipeline in `Moderate(userId, username, body)`:
  trim/collapse → 1000-char cap → per-user sliding-window rate limit (8/30s,
  attempt counts) → duplicate gate (2 min) → banned-word list (word-boundary
  regex; `AddBannedWords` for per-site extension; `ContainsBannedWord` exported
  for the prayer wall) → max 2 links → sustained all-caps lowercased (repair,
  not reject). Word-filter hits log the matched word only, never the message.
- `hub.go` — lazily-created `rweb.SSEHub` per channel (ChannelSize 16, 25s
  heartbeat). `MessageAPI` DTO shared by web widget + mobile. Events ride the
  hub's `{type, data}` JSON envelope on the default `message` SSE event:
  `chat_message` / `chat_delete` / `chat_keep`. `StreamHandler(s *rweb.Server)`
  is a constructor because the hub handler needs the server.
- `widget.go` — `RenderWidget(b, WidgetCfg{Channel, Title, Compact})`:
  self-contained shell + scoped CSS + vanilla-JS hydration (fetch history →
  EventSource live → form-encoded POST). Identity is NOT baked into HTML — the
  messages endpoint's `me` block (`logged_in`, `username`, `can_moderate`)
  drives compose box vs login hint and the ★ keep / ✕ delete tools. All user
  content enters the DOM via `textContent`. Ancient-browser polling fallback.
- `module_chat.go` (`chat`) — top-level module; channel from `Opts.ItemSlug`,
  else slugified title, else `community`.
- `module_chat_discussion.go` (`chat_discussion`) — compact embedded strip;
  `Opts.ItemSlug` is a channel *prefix* combined with `_global["item_id"]`
  (e.g. `article-42`); prefix alone for singleton placements.
- `web_rweb.go` — session-cookie JSON endpoints under `/chat`
  (list/post/keep/:id/delete/:id + `/chat/stream` outside the session group).
  CSRF-light via `Sec-Fetch-Site` (fetch widget can't use the form-token flow).
  Reads public; posting needs login; keep/delete need `CanModerate`.
- `api_rweb.go` — mobile: `GET/POST /api/v1/chat/messages`,
  `POST /api/v1/chat/messages/:id/keep`, `DELETE /api/v1/chat/messages/:id`.
  Reads public, writes `apitoken.APIGuard`ed. `has_more` via limit+1 probe
  (initial window drops the *oldest* probe row; after_id drops the newest).

### resource/prayerwall — durable content + embedded discussion

- `prayerwall.go` — `Request` type, `Validate` (title 120 / body 2000),
  hand-written queries (`InsertRequest`, `ListRequests`, `GetRequest`,
  `SetAnswered` w/ praise-report note, `DeleteRequest`). No retention — wall
  requests persist until removed.
- `module_prayer_wall.go` (`prayer_wall`) — server-rendered wall: submission
  form (logged-in; classic CSRF form posts via `app.GenerateFormToken`),
  request cards (Answered badge + note), editor controls (Mark Answered /
  Reopen / Remove), owner Withdraw (matched by user id), pagination, and
  `chat.RenderWidget` embedded at the bottom (channel `prayer-wall`, or
  `Opts.ItemSlug`). Viewer role resolved at render from `_global["username"]`.
- `web_rweb.go` — `/prayer-requests` POST create, `/answered/:id`, `/delete/:id`
  (all CSRF + flash redirect; delete allows owner-or-editor). Submissions also
  pass `chat.ContainsBannedWord`.
- `api_rweb.go` — `GET/POST /api/v1/prayer-requests`,
  `POST .../:id/answered`, `DELETE .../:id`. DTO has `mine` flag (never leaks
  other users' ids); GET resolves a Bearer token opportunistically for `mine`.

### Wiring

- Migration `db/migrate/20260715090000_CreateChatAndPrayerTables.sql`
  (chat_messages with keep flag + (channel,id) index; prayer_requests).
  **Applied to church_development.**
- `page/modules_registry.go` — registered `chat`, `chat_discussion`,
  `prayer_wall` (no `moduleContentBy` entries — calendar-style, no id options).
- `page/community_pages.go` — prebuilt `PrayerWall()` and `CommunityChat()`
  pages; routes `/prayer-wall` and `/community-chat` in the router.
- `page/article_pages.go` — `ArticleShow()` now appends a `chat_discussion`
  module (prefix `article`) → per-article comments channel.
- `basectlr` — `_global` render params now carry `username` (all three render
  funcs) and `item_id` (single-item pages) so secondary modules can key off
  the viewer and the displayed item.
- `apiv1/appconfig.go` — features now advertise `chat: true, prayer_wall: true`.
- Retention sweep started in `ServeRWeb` next to the idrive cleanup.

## Testing / Verification

- Contract tests: `resource/chat/api_contract_test.go`,
  `resource/prayerwall/api_contract_test.go` (apitest + sqlmock; both packages
  needed the `cfg/random_seeds.txt` fixture). Moderation unit tests in
  `resource/chat/moderation_test.go`. Full `go test ./...` green.
- **Real bug caught by tests**: `apiv1.Error`/`webError` return *nil* after
  successfully writing a denial, so guard helpers checking `errResp != nil`
  fell through — a 403'd keep request executed anyway (double-written body).
  Fixed with explicit `ok bool` returns in `requireAPIModerator` /
  `requireModerator`. This generalizes the earlier APIGuard lesson: never
  infer "denied" from the error value of a response-writing helper.
- Live smoke test: built cema via go.work, ran on **:8090** from a scratch dir
  with a port-edited config copy (the user's live cema on :8088 was left
  untouched). Verified: app-config flags; anonymous list/post 401s; page HTML
  embeds widgets with correct channels (`community`, `prayer-wall`,
  `article-1` — proving the `_global item_id` derivation); SSE headers;
  full authed flow (API login → post 201 → SSE listener received
  `chat_message` + `chat_keep` broadcasts → banned word 422 → member keep 403
  → editor keep ok); web-session flow (form login 303 → `me.can_moderate` →
  web post → CSRF-tokened prayer-request post → editor controls rendered).
  Smoke rows cleaned from the DB afterward.
- Seeded test users remain for future testing (seeder:
  `test_scripts/seed_chat_test_users/main.go`, idempotent): `chat-tester`
  (role 9) and `chat-editor` (role 7), password [redacted — see seeder].

## Notes / Follow-ups

- The user's long-running cema instance on :8088 predates this work — rebuild
  and restart it to serve the new features.
- Flutter side has NOT consumed the new endpoints yet (screens for chat +
  prayer wall are a next milestone; SSE via `/chat/stream` or `after_id`
  polling both work for the app).
- Moderation deny-list is a mild starter — extend per site with
  `chat.AddBannedWords` at bootstrap.
- Chat reads are public by design (like article comments); only posting needs
  login. Revisit if a members-only wall/chat is wanted.
