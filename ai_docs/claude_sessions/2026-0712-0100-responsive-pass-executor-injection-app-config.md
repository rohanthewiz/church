# Session: Responsive Pass + Executor Injection + /api/v1/app-config

Session ID: `52a7aa5d-f51c-466c-b1d7-fe796aad9a01`
Date: 2026-07-12

## What was asked

Continue from the previous session with sequencing items 5 and 6 of
`ai_docs/fable_platform_analysis.md`, plus the mobile `GET /api/v1/app-config`
milestone:

5. Responsive pass — media queries for the 3-column layout and grid
6. Executor-injection refactor for queries, then auth/payment tests

## Item 5 — Responsive pass (implemented)

- **`template/responsive_css.go`** (new): `template.ResponsiveCSS`, inlined by
  `template.Page` immediately after the site `app.css` link (same slot as
  grid.CSS) so it wins the cascade on every site without a stylus rebuild.
  Under 768px it stacks `#left-side/#main/#right-side` as a flex column
  **inside** the `#mid` scroll container, `#main` first via `order:-1`, and
  neutralizes the site CSS's `min-width:800px` (the single rule forcing
  horizontal panning on phones). Deliberately does NOT touch the fixed
  banner/menu/footer frame (per-site metrics: cema banner 116px, menu 28px,
  footer 110px) and never sets `display` on the children, so layout-specific
  `display:none` sidebar rules keep working. `!important` throughout — the
  site rules are id+class selectors compiled from stylus.
- **`grid/assets.go`**: new `@media (max-width:640px)` block — 16px fonts on
  search/filter/pager controls (prevents iOS Safari zoom-on-focus), ~44px
  touch targets (Apple HIG) on pager buttons/group button/select, wrapping
  toolbar with the row count on its own line, taller year-group rows.
- Not browser-verified yet — needs a running site binary (cema/ccswm) and a
  phone-width viewport check next session.

## /api/v1/app-config (implemented)

- **`resource/apiv1/appconfig.go`**: bare-object JSON (single resources are
  unenveloped): `{church_name, theme, stripe_publishable_key, giving_contacts,
  features: {giving, sermon_audio}, server_version}`.
  - `church_name` = CopyrightOwner (banner_inner_html is HTML, unusable).
  - `features.giving` true only when BOTH Stripe keys configured (client SDK
    needs pub key, server create-intent needs priv key).
  - `features.sermon_audio` = IDrive.Enabled.
  - `giving_contacts` serializes `[]` never null (Dart contract discipline).
  - Reads only `config.Options` — no DB, works during a DB outage.
- Route in `router_rweb.go` on the public `/api/v1` group (fetched pre-login).
- Contract tests `resource/apiv1/appconfig_test.go`: full shape, both-keys
  rule, never-null contacts, private key must not appear anywhere in the body.

## Item 6 — Executor-injection refactor (implemented)

- **`db/executor.go`** (new): `db.Executor` interface (Exec/Query/QueryRow) —
  method-set-identical to vattle SQLBoiler v2's `boil.Executor`, so it passes
  implicitly into generated model calls; hand-written-SQL packages (apitoken,
  idrive cache, event recurrences) use it without importing sqlboiler.
- **Convention** (documented on the interface): executor is always the FIRST
  param of query/presenter functions; `db.Db()` is fetched only at boundaries —
  HTTP handlers, module `getData`/`GetData`, background services, bootstrap,
  menu render — and threaded down. Query functions never reach the global.
- Converted (~60 call sites, ~45 files): `resource/{sermon,article,event,user,
  payment,apitoken,menu}`, `page`, `core/idrive` sermon cache, plus all
  callers (controllers, modules, standalone_modules, admin bootstrap,
  payment_recorder, import2). Notes:
  - `resource/menu` was missing from the original analysis inventory —
    included. `menuDefFromSlug(nil, slug)` (nil executor) falls back to the
    hardwired menus so a fresh-install/DB-down site stays navigable;
    `RenderNav` is the boundary (called from the template with no DB context).
  - `import2.go` reads from db2 (legacy) but upserts to the main DB — the
    destination handle is fetched separately (`mainDbH`).
  - idrive: `TrackSermonAccess` and `runCacheCleanupPass` remain boundaries
    (goroutine/background loop, log-only errors); `selectIdleCachedSermons`
    lost its `QueryContext(context.TODO())` for plain `Query` to fit the
    interface.
  - `event.LoadRecurrence`, `page.PageFromSlug/PageFromId`, `user.SaveUser/
    SuperAdminsExist/AuthUserByUsername/UserCreds`, apitoken
    `Issue/LookupUser/RevokeByHash/RevokeAllForUser` all take exec now.
  - `arch_test_scripts/recurrence_live_check` updated (part of `go build ./...`).

## Auth + payment tests (implemented)

- **`auth_controller/auth_flow_test.go`** (+ `auth_controller/cfg/random_seeds.txt`
  fixture — the resource/auth init() landmine): real rweb router
  (`Server.Request`), real scrypt, real kvstore sessions; sqlmock via
  `apitest.MockDB`. Covers: login success 303→`/` issuing a session cookie
  that then passes `AdminGuardRWeb` on `/admin/home`; wrong password and
  unknown user answer identically (303→`/login`, no session cookie — no
  username oracle); missing fields rejected before any query; anonymous and
  stale-cookie admin requests redirect to `/login`. First Set-Cookie asserted
  to be the session cookie (order: session before flash).
- **`resource/payment/payment_model_test.go`** (+ cfg fixture — payment pulls
  resource/auth transitively): `ChargePresenter.Upsert` insert path (no Id →
  INSERT..RETURNING), update path (Id → SELECT then UPDATE, `updateOp=true`),
  customer-name validation fails before any SQL; `FindChargeIdByPaymentToken`
  hit/miss(-is-not-error)/blank-token-short-circuit. These hand sqlmock
  directly to the functions — the executor seam's payoff, no global swap.
- **`payment_controller/webhook_test.go`** (+ cfg fixture): 503 when
  `stripe.webhook_secret` unconfigured (Stripe keeps retrying), 400 on forged
  signature, 200 ack for unhandled event types using
  `webhook.GenerateTestSignedPayload` (real HMAC path; payload must carry
  `stripe.APIVersion` or ConstructEvent rejects it), and
  `recordPaymentIntent` idempotency: same intent twice = one INSERT then
  UPDATE (second INSERT would fail ExpectationsWereMet). The
  payment_intent.succeeded HTTP path is not driven end-to-end by design — the
  handler re-retrieves the intent from Stripe's API and can't run offline.

## Verification

- `go build ./...`, `go vet ./...` clean; `go test ./...` green — 14 test
  packages (was 11), new: auth_controller, payment_controller,
  resource/payment, plus apiv1 grew app-config tests.
- Live: `go run ./test_scripts/auth_live_check` against local postgres@16 —
  bad/good login, me, logout, hash-not-plaintext storage, 0 tokens after
  logout; validates the refactored auth/token SQL end-to-end.

## Docs / memory

- `ai_docs/fable_platform_analysis.md`: items 5 & 6 marked DONE with detail;
  app-config noted (mobile milestone (b) part 1).
- `ai_docs/plans/2026-0707-mobile-app-flutter-api-plan.md`: app-config marked
  IMPLEMENTED with the response shape.
- Memory `mobile-interop-priority` refreshed (all six sequencing items done).

## Not done / next steps

- JSON create-intent for giving (milestone (b) part 2) +
  `GET /api/v1/payments/history`.
- `RevokeAllForUser` still not wired to password change / user disable.
- `has_more` pagination; Flutter side (token store, login screen, screens,
  just_audio).
- Browser check of the responsive layout on a running site binary; admin
  click-through of POST deletes still pending from last session.
- Conventions to remember: new query code takes `db.Executor` first; test
  packages transitively importing resource/auth need the
  `cfg/random_seeds.txt` fixture.
