# Session: ccswm RWeb Migration + Echo Removal from Church Framework

Session ID: `a20f4367-4edf-48f0-a09d-e78544e190bd`
Date: 2026-07-06 → 2026-07-07

## What happened

1. **Migrated ccswm off the legacy Echo stack** to `church.ServeRWeb()` and bumped its church dependency from v0.1.15 (June 2025) to master HEAD — closing a year of drift.
2. **Moved ccswm hosting from Bitbucket to GitHub** (Bitbucket rejected pushes: app passwords deprecated, HTTP 410).
3. **Removed the entire Echo layer from the church framework** — 20 files deleted, Echo and friends dropped from go.mod.

## ccswm migration (committed `9280f9f`, pushed)

- `main.go`: `church.Serve()` → `church.ServeRWeb()`; commented out `roredis.InitRedis` (framework sessions now use in-process `core/kvstore`, Redis unused) and the rubberneck config dump (logs credentials).
- `go.mod`: church pinned to `v0.10.1-0.20260707023449-0dac7cbdec84` (master HEAD incl. PaymentIntents migration). `stripe-go` v55 → `stripe-go/v86` (indirect), `logger`/`serr`/`element` upgraded, `roredis`/`rubberneck` dropped, `rweb` added.
- `cfg/options-sample.yml`: documented new `stripe.tx_description` and `stripe.webhook_secret` keys (parity with cema's sample).
- Church master (PaymentIntents commits `c82322c`, `0dac7cb`) had to be pushed first — the auto-mode classifier blocked my push; user pushed via `! git push origin master`.

### ccswm repo move
- Old remote was `https://<user>@bitbucket.org/<user>/ccswm.git` — pushes fail with 410 (Atlassian CHANGE-3222: app passwords deprecated in favor of API tokens).
- Created **private** GitHub repo `rohanthewiz/ccswm` via `gh repo create`, repointed origin, pushed master with tracking. Bitbucket copy still exists but is frozen pre-migration and is no longer a remote.

## Echo removal from church framework (this repo)

### Deleted (20 files)
- `router.go` — legacy `church.Serve()` entrypoint + Echo middleware/route wiring.
- All 13 Echo controller files: `admin_controller/admin_controller.go`, `app/application_controller.go`, `article_controller/article_controller.go`, `auth_controller/{auth_controller,auth_helpers,auth_middleware}.go`, `basectlr/{base_controller,send_file}.go`, `event_controller/event_controller.go`, `menu_controller/menu_controller.go`, `page_controller/page_controller.go`, `payment_controller/payment_controller.go`, `sermon_controller/sermon_controller.go`, `user_controller/user_controller.go`.
- `context/context_crud.go`, `context/custom_context.go` (Echo `CustomContext`; RWeb side lives in `context/rweb_helpers.go`).
- `resource/calendar/fullcalendar_events.go`, `resource/sermon/api.go`.
- `resource/cookie/` — entire package was Echo-only (RWeb has native cookie support; flash uses `ctx.SetCookie`/`GetCookieAndClear`).

### Shared code rescued out of deleted Echo files
- `app/form_token.go` (new) — `GenerateFormToken`/`VerifyFormToken` CSRF helpers; kvstore-based and transport-agnostic, used by all `resource/*/module_*_form.go`.
- `basectlr/recover.go` (new) — `logPanic` + `recoverMsg` used by the RWeb page renderers' panic recovery.
- `FullcalendarEvent` type → `resource/calendar/fullcalendar_events_rweb.go`; fixed its malformed `json:"end, omitempty"` tag (was a vet warning).
- `SermonsResp` DTO → `resource/sermon/api_rweb.go`.
- `flash/flash.go` trimmed to RWeb + `Render()` halves (Echo `Set/Get/GetOrNew` removed).

### Docs/comments updated
- README: RWeb replaces Echo in blurb + architecture; sessions documented as in-process kvstore (Redis requirement and install section removed).
- `payment_controller/payment_controller_rweb.go`: legacy-handler comment no longer references the deleted Echo twin.

### Dependency payoff
`go mod tidy` dropped `labstack/echo`, `labstack/gommon`, `valyala/fasttemplate`, `valyala/bytebufferpool`. Likely retires some of the 9 Dependabot findings GitHub reported on push (1 critical, 1 high, 7 moderate) — **follow-up: check https://github.com/rohanthewiz/church/security/dependabot**.

## Verified
- church: `go build ./...`, `go vet ./...` (now **zero** vet warnings — both pre-existing ones were in deleted/fixed code), `go test ./...` pass.
- **Both cema and ccswm build** against the Echo-free framework via temporary `go mod edit -replace` (reverted afterward).
- Net church diff: 22 files changed, +81/−1573.

## Outstanding / follow-ups
- Deploy note for ccswm: server's real `options.yml` needs `stripe.webhook_secret` (+ Stripe dashboard webhook for `payment_intent.succeeded`), optional `stripe.tx_description`; sessions are in-process now, so restarts log users out.
- cema still calls `roredis.InitRedis` in main.go — harmless but removable; cema is also still pinned to a pre-PaymentIntents church (`96257e0`) and should be bumped to pick up the payments flow and Echo removal.
- Review remaining Dependabot findings after pushing the Echo removal.
- Architecture-review backlog (from previous session): role-level enforcement in `AdminGuardRWeb`, parameterize raw SQL conditions (e.g. fullcalendar events date range), extract lifecycle from `ServeRWeb`, durable kvstore backend for mobile tokens.
