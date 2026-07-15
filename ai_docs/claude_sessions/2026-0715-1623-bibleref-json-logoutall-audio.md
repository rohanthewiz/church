# Session: bibleref in Mobile JSON + logout-all/RevokeAllForUser + Background Audio

**Session ID**: `350a732f-9fc9-4182-907c-81b8622de5c1`
**Date**: 2026-07-15
**Branch**: master (both repos)

## Goal

Execute the next steps from the previous session (BLB ScriptTagger + bibleref
sketch): wire `bibleref.FindAll` into the mobile JSON handlers, wire
`RevokeAllForUser`, device testing, and background audio (audio_service).

## Work Done — server (`church`)

### 1. bibleref refs in sermon/article JSON

- `resource/bibleref`: added `APIRef` (embedded `Ref` + `url`), `FindAllAPI`
  (always non-nil → `[]` never `null`), and `FirstURL` (first ref of a string
  or ""). Empty translation defaults to NKJV, matching web ScriptTagger.
- `SermonAPI` gains:
  - `scripture_ref_urls []string` — **index-aligned** with `scripture_refs`;
    "" when an entry doesn't parse (topical notes). Alignment is contract.
  - `summary_refs []bibleref.APIRef` — refs found in `summary` with **UTF-8
    byte offsets** (Go string indexing). Dart clients must slice utf8 bytes,
    not UTF-16 code units.
- `ArticleAPI` gains `summary_refs`. Summary only — body is HTML where byte
  offsets would index markup; the website's ScriptTagger covers rendered HTML.
- Feed aggregator reuses the same DTO builders, so it got both for free.
- Contract tests updated: sermon fixture summary now contains a real ref
  ("Grace in Eph 2:8-9 explained") and asserts raw/start/end/url values;
  article test freezes summary_refs=[] for ref-free summaries.

### 2. RevokeAllForUser wiring

- New guarded `POST /api/v1/auth/logout-all` (`APILogoutAllRWeb`) — revokes
  every token for the bearer's account. Contract test asserts the DELETE is
  by **user_id** (all devices), not token_hash (one device), + 401 unauthed.
- `user_controller.UpsertUserRWeb`: on update with password change or
  enabled=false → `apitoken.RevokeAllForUser`. Lives in the controller, NOT
  `resource/user`, because `apitoken` imports `resource/user` (import cycle
  the other way). Revoke failure logs loudly but doesn't fail the admin save.
- User delete needs nothing: `api_tokens.user_id` FK is `ON DELETE CASCADE`.

## Work Done — Flutter (`church_mobile`)

- `lib/src/models/bible_ref.dart` — `BibleRef` DTO mirroring APIRef;
  `listFromJson(null)` → empty (older servers).
- `lib/src/widgets/ref_text.dart` — `RefText` renders text with tappable BLB
  spans. Key detail: slices `utf8.encode(text)` bytes by the server's
  offsets and decodes each segment — indexing the Dart String with byte
  offsets would corrupt anything containing curly quotes/em-dashes.
  Out-of-range/overlapping refs degrade to plain text (never RangeError).
  Stateful only to dispose TapGestureRecognizers.
- `Sermon` model: `scriptureRefUrls`, `summaryRefs`, and `refUrl(i)` — server
  URL preferred, legacy client-side guesser as fallback for older servers
  (guesser produces dead links for abbreviated books; server is
  authoritative). `Article` model: `summaryRefs`.
- Sermon detail: chips use `refUrl(i)`; summary rendered via RefText.
  Article detail: now shows summary as an italic lede with RefText.
- Logout-all: `ApiClient.logoutAll()`, `SessionController.logoutAll()` —
  deliberately opposite error posture to `logout()`: it THROWS on failure and
  keeps the local session (a false "signed out everywhere" would be a broken
  security promise). More tab gains "Sign out everywhere" with confirm
  dialog + failure snackbar.
- **Background audio** via `just_audio_background` (audio_service under the
  hood; fits the single-player shape):
  - `lib/src/audio/sermon_audio_controller.dart` — app-lifetime AudioPlayer
    in `AppServices.audio`; `load(sermon, uri)` no-ops when the same sermon
    is already current, so re-entering the screen rebinds at position.
    Sets `MediaItem` tag (required — plain `setUrl` now throws app-wide).
  - `SermonPlayer` widget is now a view over the shared player (no
    create/dispose; load kicked off in didChangeDependencies — AppScope is
    unavailable in initState). Speed display syncs from the live player.
  - `main.dart`: `JustAudioBackground.init` before the first player exists;
    channel id `org.church.mobile.channel.audio`.
  - Android: MainActivity now extends `AudioServiceFragmentActivity` —
    satisfies BOTH audio_service and flutter_stripe (PaymentSheet requires a
    FragmentActivity). Manifest: WAKE_LOCK + FOREGROUND_SERVICE(+
    _MEDIA_PLAYBACK) permissions, AudioService service
    (foregroundServiceType=mediaPlayback) + MediaButtonReceiver.
  - iOS: `UIBackgroundModes` = audio in Info.plist.

## Verification

- Go: full `go test ./...` green. Flutter: `flutter analyze` clean; 16 tests
  pass (`test/bible_ref_test.dart` covers model parse, ASCII + non-ASCII
  byte-offset splicing, bad-offset degradation, recognizer wiring).
- `flutter build apk --debug` succeeds — proves the Kotlin/manifest changes.
- Live end-to-end: built **cema** against the local church module using a
  scratch `go.work` (`GOWORK=... go build`, go 1.25.0) — cema/ccswm pin a
  published church version, so don't edit their go.mod for local testing.
  cema serves :8088 against DB `church_development`. Inserted sermon id=1
  ("Living by Faith") whose summary mixes refs with em-dash/curly quotes:
  API returned aligned scripture_ref_urls (["…jhn/3/16/", "…2ti/1/7/", ""])
  and summary_refs with correct byte offsets (Hab 2:4 @ 11-18, Romans
  1:16-18 @ 32-46, Eph 2:8-9 @ 66-75). logout-all 401s unauthed. Test
  sermon left in the dev DB for app testing.

## Blocked / Next Steps

- **On-device run** (hardware-blocked this session): no phone/emulator; no
  Xcode; Android cmdline-tools/licenses unaccepted (gradle debug builds do
  work). When a device is attached:
  `flutter run --dart-define=API_BASE=http://<mac-ip>:8088`. Lock-screen
  controls need a physical device to judge properly.
- `has_more` pagination on the older list endpoints; live Stripe smoke test;
  browser check of the responsive pass on a running site binary.
