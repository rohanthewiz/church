# Session: Phase 2 Mobile Auth + Security Quartet

Session ID: `e4474079-0a63-4db5-a40c-de04f52e575d`
Date: 2026-07-11 → 2026-07-12

## What was asked

Continue with items 3 and 4 of `ai_docs/fable_platform_analysis.md`:

3. Phase 2 mobile auth — token table, Bearer guard, `/api/v1/auth/*`
4. Security quartet — plaintext-password log, `/debug/*` guard, POST-based
   deletes, `connect2.go` swallowed error

## Item 3 — Phase 2 mobile auth (implemented)

- **Migration `20260711150000_CreateApiTokensTable.sql`** (applied to local PG):
  `api_tokens` holds SHA-256 hex of the token (plaintext exists only in the
  login response), `user_id` FK CASCADE, `device` label, `created_at`,
  `last_used_at`, fixed 30-day `expires_at`, index on user_id.
- **New `resource/apitoken` package**, hand-written SQL (no SQLBoiler regen —
  same precedent as event_recurrences/sermon_cache_access):
  - `apitoken.go`: `Issue` (32 crypto/rand bytes, hex), `HashToken` (SHA-256 —
    fast is fine, input is 256 random bits), `LookupUser` (single JOIN to users
    checking expiry + `enabled` in the query, best-effort `last_used_at` touch),
    `RevokeByHash`, `RevokeAllForUser` (for future password-change/disable wiring).
  - `api_rweb.go`: `APILoginRWeb` (JSON `{username,password,device?}` with
    urlencoded fallback → `{token, expires_at RFC3339, user}`), `APIMeRWeb`
    (`{user}` envelope), `APILogoutRWeb` (`{"ok":true}`, revokes only the
    presented token), `APIGuard`, credential-free `APIUser` DTO (id, username,
    first_name, last_name, email, role, role_name), in-process login throttle
    (sliding window, 10 failures/15 min per client-IP+username, cleared on
    success). Unknown user and wrong password answer identically (no username
    oracle); passwords never logged.
- **`user.AuthUserByUsername`** added to `resource/user` — enabled-only lookup
  returning identity + scrypt hash/salt; SQLBoiler no-rows detected by message
  (v2 wraps the sentinel). Credentials stay in the resource layer.
- **Router**: `api.Post("/auth/login", ...)`;
  `api.Get("/auth/me", apitoken.APIGuard(apitoken.APIMeRWeb))`;
  `api.Post("/auth/logout", apitoken.APIGuard(apitoken.APILogoutRWeb))`.

### Key gotcha discovered (contract test caught it)

**rweb group middleware auto-continues into the route handler when middleware
returns nil without calling `Next()`** (`Group.addRoute` in rweb). A guard
written as group middleware that writes a 401 and returns nil still runs the
handler → response body written twice. Hence `APIGuard` is a **per-handler
decorator** (`func(next rweb.Handler) rweb.Handler`) — denial means the
handler structurally never runs. Note: the web `AdminGuardRWeb` + redirect
"works" only because browsers follow the 303 and ignore the appended body.

## Item 4 — Security quartet (implemented)

1. **Password log**: failed-login warn in `auth_controller_rweb.go` no longer
   logs the submitted password (was logging it verbatim — often a typo of the
   real one).
2. **`/debug/*`**: the four element-debug routes moved into a
   `s.Group("/debug", UseCustomContextRWeb, AdminGuardRWeb)` — they toggle
   process-wide debug state and were fully public.
3. **POST deletes** (all six: users/articles/sermons/events/pages/menus):
   - Routes `ad.Get(".../delete/:id")` → `ad.Post(...)`.
   - `grid.Grid` gained `CSRFToken` (rendered as `data-csrf` on the wrapper);
     the grid JS delete handler now builds and submits a POST form with a
     hidden `csrf` input instead of `window.location = url`.
   - Each admin list module (incl. `page/module_pages_list.go`) generates a
     token in its constructor via `app.GenerateFormToken()` — only when
     `Opts.IsAdmin`, so public renders skip the kvstore write.
   - New helper `app.VerifyFormTokenRWeb(ctx, redirectTo)` — one-liner check in
     each delete handler; failure = warn flash + redirect (common cause is the
     1h token TTL aging out).
4. **`db/connect2.go`**: `InitDB2` now returns the wrapped error (was
   discarding the `serr.Wrap` result → silent init failure).

## Testing / verification

- `resource/apitoken/api_contract_test.go` (sqlmock via apitest): login 200
  shape (64-hex token, RFC3339 expires_at ≈ 30d, user keys, no credential
  fields), form fallback, 400/401 (identical messages), 429 after 10 failures
  with no DB touch, me contract, guard rejections (no header / wrong scheme /
  unknown token), logout deleting the exact presented hash.
- `apitest.RequestJSON` added (method/headers/body generalization of GetJSON).
- `resource/apitoken/cfg/random_seeds.txt` fixture copied (resource/auth
  init() landmine, as with the other test packages).
- **Live end-to-end**: `go run ./test_scripts/auth_live_check` against local
  postgres@16 (brew, still running) — seeds a user, drives login (bad+good) /
  me / logout / me in-process via `Server.Request`; confirmed hash-not-plaintext
  storage and zero rows after logout. This validates the hand-written SQL that
  sqlmock cannot.
- `go build ./...`, `go vet ./...` clean; `go test ./...` green (11 pkgs).
- Old test scripts moved to `arch_test_scripts/` per convention;
  `auth_live_check` kept current in `test_scripts/`.

## Docs updated

- `ai_docs/fable_platform_analysis.md` — items 3 & 4 marked DONE with detail.
- `ai_docs/plans/2026-0707-mobile-app-flutter-api-plan.md` — new "Phase 2 auth
  implementation notes" section (includes the middleware gotcha).
- Project memory (`mobile-interop-priority`) refreshed.

## Not done / next steps

- Sequencing item 5: responsive pass (3-column layout + grid media queries,
  event form as pattern); item 6: executor-injection refactor + auth/payment
  tests.
- Mobile milestones: `GET /api/v1/app-config`, JSON create-intent for giving;
  Flutter side: token store (flutter_secure_storage), login screen, Bearer
  header in api_client.dart, re-login on 401.
- `RevokeAllForUser` not yet wired to password change / user disable.
- Web deletes not live-smoke-tested in a browser (unit-level coverage only);
  worth a click-through on the next admin session.
