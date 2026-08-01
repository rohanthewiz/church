# Session: Chat + Prayer Wall Emulator Smoke Test (church_mobile)

- Session ID: `27d26784-d74f-4bd9-8a50-c90a911b4842` (same session as
  `2026-0801-1124-chat-prayer-wall-flutter-screens.md` — continued)
- Date: 2026-08-01 11:49
- Repos touched: none (no code changes; this doc is the only commit).
  Verified `church_mobile@745886c` against `church@0104433` (cema rebuilt
  locally via go.work, binary is gitignored).

## Goal

Live end-to-end smoke of the new chat + prayer-wall Flutter screens on the
Android emulator against a real server — the one gap called out when the
screens shipped (SSE path was unit-tested but never exercised from the app).

## Server recipe (reusable)

- Rebuilt cema (`cd ~/projs/go/church/cema && go build`) — its `go.work`
  pins `../church`, so the local module rides along.
- Scratch run dir (in the session scratchpad): `cfg/` copy with
  `sed 's/port: 8088/port: 8090/'`, `cfg/random_seeds.txt` copied,
  `dist` symlinked, then `APP_ENV=development DB_TYPE=postgres ./cema`.
- **`DB_TYPE=postgres` was load-bearing**: cema's options.yml has no `db:`
  block, so the binary now defaults to bytdb (empty DB — no users). The
  seeded smoke users live in PG `church_development`. Seeder re-run first
  (idempotent): `go run test_scripts/seed_chat_test_users/main.go` from the
  church root (resource/auth needs `cfg/random_seeds.txt` relative to cwd).
  Users: `chat-tester` (role 9), `chat-editor` (role 7), password in seeder.
- Redis no longer needed (sessions moved to in-process core/kvstore).
- Curl-verified before app work: app-config advertises
  `features.chat/prayer_wall: true`; chat + prayer list endpoints answer.

## App build

- `flutter build apk --debug --dart-define=API_BASE=http://10.0.2.2:8090`,
  `adb install -r`, launched on AVD `Medium_Phone_API_36.1`.
- First screenshot was black — emulator display asleep; `input keyevent
  KEYCODE_WAKEUP` + `wm dismiss-keyguard` before screencaps.

## What was verified (all pass, screenshots in session scratchpad)

Chat (`community` channel):
- More tab shows the two feature-gated entries (flags from app-config).
- Empty state renders the 24h-retention wording; composer replaced by the
  "Sign in to join the conversation" bar when signed out; **no reconnect
  banner** (SSE connected on first try).
- curl POST as chat-editor → message **appeared live in the app via SSE**,
  no refresh (left bubble, display name, time). An independent
  `curl -sN /chat/stream?channel=community` Monitor captured the exact
  `{"type","data"}` envelopes concurrently (chat_message/chat_keep/
  chat_delete) — wire format matches the Dart parser 1:1.
- Signed in as chat-tester from the chat screen; LoginScreen popped and the
  composer **swapped in place** (session listener) with history intact.
- Posted from the app: 201 + right-aligned own bubble, composer cleared,
  **exactly one copy** — POST-echo vs SSE-echo dedupe by id held.
- curl keep (id/keep) as editor → **pin icon patched onto the bubble live**
  (`chat_keep` handler, `withKeep` local patch).
- Long-press as role-9 member → sheet offers **only "Copy text"** (no
  Pin/Delete) — `User.canModerate` affordance gate confirmed.

Prayer wall:
- Empty state + `Request prayer` FAB + app-bar forum icon (chat pairing).
- Compose sheet: 0/120 + 0/2000 counters, sheet rises above the keyboard
  (viewInsets), posted → card shows title/body/**"You"** (mine flag) +
  snackbar; owner menu contains **only "Withdraw"** (no moderator items).
- curl mark-answered as editor (with note) → pull-to-refresh rendered the
  tinted "Answered — …" badge with the note.
- Withdraw from the app: confirm dialog (title quoted) → card gone; server
  list confirmed empty.

## Cleanup

- Chat smoke messages deleted via moderator DELETE (both `chat_delete`
  broadcasts observed on the stream monitor); prayer row removed by the
  in-app withdraw; editor token revoked via /auth/logout; :8090 server and
  SSE monitor stopped; emulator shut down (`adb emu kill`).
- PG left with only the two seeded users (as the 0715 session left it).

## Gotchas worth keeping (also in auto-memory)

- `adb shell input text` needs `%s` for spaces — a quoted sentence silently
  truncates at the first space.
- The soft keyboard opening reflows the layout: tap coordinates captured
  pre-keyboard hit the wrong field. Screenshot between focus and typing.
- A back keyevent right after dismissing a bottom sheet gets consumed by
  the sheet/keyboard — use the app-bar back arrow to pop reliably.

## Follow-ups

- Physical-device pass still pending (background-audio lock-screen; also
  worth re-running this smoke on hardware once available).
- FCM push (chat is foreground-only while the app is open).
- Reconnect/backoff path not fault-injected this session (would need a
  server bounce mid-stream) — unit-level watchdog logic remains the
  coverage there.
- Live Stripe smoke + Apple Pay entitlements unchanged from prior sessions.
