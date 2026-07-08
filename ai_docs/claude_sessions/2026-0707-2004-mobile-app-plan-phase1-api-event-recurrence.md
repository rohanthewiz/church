# Session: Mobile App Plan + Phase 1 JSON API + Event Recurrence

Session ID: `f9474e3e-d3d4-4a82-9068-600652487a00`
Date: 2026-07-07

## What happened

Three workstreams; church changes are ALL UNCOMMITTED in the working tree (only the
prior sessions' commits `cb80039`..`2a8f896` were pushed, by the user, mid-session):

1. Mobile app planning: Flutter chosen; full API plan saved to
   `ai_docs/plans/2026-0707-mobile-app-flutter-api-plan.md` (the living doc for this work —
   includes an as-implemented section kept current).
2. New repo **`church_mobile`** (sibling dir `~/projs/go/church/church_mobile`), pushed to
   github.com/rohanthewiz/church_mobile (private), 2 commits.
3. church server: Phase 1 read-only `/api/v1` endpoints + full event recurrence support
   ("every Sunday", "every second Saturday", "last Sunday of the month").

## 1. Framework + API plan (see the plan doc for full detail)

- **Flutter** over RN/KMP: `just_audio`+`audio_service` fit the Range-streaming sermon
  endpoint (already mobile-ready, verified 206/Accept-Ranges); `flutter_stripe` PaymentSheet
  consumes the PaymentIntent client secret flow the server already has; no JS investment
  server-side.
- Phases: 1 read-only (done, this session) / 2 auth+payments (tokens must be DB-backed, NOT
  in-process kvstore; DTO layer first — user.Presenter leaks credentials) / 3 chat (SSE) +
  push (FCM).
- BlueLetterBible: client-side deep links parsed from scripture_refs (now a JSON array).

## 2. church_mobile repo

- `flutter create --empty` (iOS/Android, org com.rohanthewiz) + `http` dep.
- `lib/src/api/api_client.dart`: one method per Phase 1 endpoint; `API_BASE` via
  --dart-define (default localhost:8000); uniform ApiException from `{"error": msg}`;
  `resolveMediaUrl` resolves relative audio URLs against the API base.
- `lib/src/models/`: Sermon (with `blbUrlFor` BLB deep-link parser), Article,
  ChurchEvent + EventRecurrence (named to avoid dart `Event` collision), Feed.
- `dart analyze` clean. No UI yet — next: home feed screen, sermon list/player.

## 3. church Phase 1 API (uncommitted)

- `resource/apiv1/apiv1.go` — shared ParseLimitOffset (hard caps) + JSON error shape.
- `resource/sermon/api_rweb.go` rewritten: SermonAPI DTO (id, refs array, audio_url,
  summary/teacher/place/categories; body detail-only), filters year/teacher/ref (bound
  params), detail endpoint, RecentSermonsAPI. Old thin SermonsResp commented for reference
  (no known consumers).
- `resource/article/api_rweb.go`, `resource/event/api_rweb.go` new: list/detail + feed
  helpers. `resource/feed/feed_rweb.go`: /api/v1/feed aggregator, sections degrade
  independently; own package to keep feed -> {sermon,article,event} -> apiv1 acyclic.
- `router_rweb.go`: `/api/v1` group (public reads, outside session middleware).
- Contract: published-only (drafts 404 same as missing); arrays serialize `[]` never null;
  enveloped lists `{"sermons": [...], "limit": n, "offset": n}`; detail returns bare object.
- Tests: `resource/sermon/api_rweb_test.go` (DTO mapping). The `resource/auth` init()
  landmine (needs `cfg/random_seeds.txt` per test package cwd) required fixture copies to
  `resource/sermon/cfg/` and `resource/event/cfg/`.

## 4. Event recurrence (uncommitted)

- **Migration `20260707130000_CreateEventRecurrencesTable.sql`**: 1:1 `event_recurrences`
  (event_id PK/FK CASCADE, freq 'weekly'|'monthly', weekday 0-6 = Go time.Weekday,
  week 1..4|-1=last, until date NULL-able) + CHECK constraints.
- **Hand-written SQL** (`recurrence_queries.go`) — deliberately NO SQLBoiler model
  (avoids legacy vattle v2 regen; same precedent as sermon_cache_access).
- **Engine** (`recurrence.go`): pure `Occurrences(anchor, from, to)`; anchor = event_date,
  occurrences strictly after it (base row represents its own date); date math on yyyymmdd
  ints to dodge tz drift; `Describe()` -> "Last Sunday of each month". Tests use
  hand-verified 2026 dates incl. year-wrap.
- **`event.WindowedEvents(from, to)`** = single expansion point for /api/v1/events, feed,
  AND the website FullCalendar endpoint (rewritten: now published-only — it previously
  leaked unpublished events — and parses start/end instead of SQL-binding raw strings).
  Occurrences share base id; `recurring` + `recurrence_desc` in lists; structured
  `recurrence` object on detail. Default 92-day window (`DefaultWindowDays`); paging
  post-expansion in memory; base query capped 500 with logged warning.
- **Admin form** (`module_event_form.go`): Repeats/On-weekday/Week(monthly)/Repeat-until
  selects; controller passes recur_* form fields; rule syncs in `UpsertEvent`
  (delete on "None"; save failure surfaced not swallowed). `Presenter.LoadRecurrence`
  only on single-event edit (avoids N+1 in lists).

## Verification

- church: `go build ./...`, `go vet ./...`, `go test ./...` green throughout.
- **Live DB validation**: local brew `postgresql@16` was fresh/empty (real dev data lives
  elsewhere, wasn't running) — created dev user + `church_development`, applied ALL goose
  migrations cleanly (pressly goose installed to `~/go/bin/goose`), SQL-validated CHECK
  constraints + ON CONFLICT upsert + FK cascade.
- **`test_scripts/recurrence_live_check/main.go`** seeded weekly (until Sep 30),
  monthly-last-Sunday, and one-time events; Jul–Oct 2026 expansion printed exactly right
  (17 entries; weekly stops at until; last Sundays Jul 26/Aug 30/Sep 27/Oct 25).
- **Local Postgres note**: `postgresql@16` brew service was started this session and left
  RUNNING; DB has schema only, no content. `brew services stop postgresql@16` to stop.

## Outstanding / follow-ups

- **Commit the church working tree** (Phase 1 API + recurrence) — user reviews/commits.
- Dependabot banner on push was stale; `57a35f3` bumps should clear alerts after rescan.
- Phase 1 endpoints not yet exercised with real content data (fresh DB had none beyond the
  smoke seeds) — sanity-check against real dev data when available.
- Mobile next: home feed screen, sermon list + player (just_audio/audio_service),
  `table_calendar` month view + `add_2_calendar` (event_time is free-form text — parse or
  fall back to all-day).
- Recurrence maybe-laters: exceptions/skips (e.g. "no service Dec 27"), per-occurrence
  overrides, admin list badge for recurring events.
- Phase 2 prerequisites unchanged: DB-backed API tokens, DTO layer (credential leak),
  proper registration w/ email verification, rate-limited login, app-config endpoint.
- Prior sessions' follow-ups still open: IDrive S3 SDK jump sanity check; TLS deploy;
  cema/ccswm church-version bumps; ccswm stripe.webhook_secret.
