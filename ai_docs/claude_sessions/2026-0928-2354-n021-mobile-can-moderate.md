# N-021: permission-only moderators in the grmob app

Session ID: `94221085-fab0-4a83-8d9b-13d285c765ba`
Date: 2026-09-28

(Earlier in this session: `2026-0928-2339-n004-unique-index-recommendation`,
`2026-0928-2347-n005-theme-vars-material-form`,
`2026-0928-2351-n009-postgres-import-webhook-tests`.)

## Goal

N-021: check that the mobile app shows moderation controls to a
permission-only moderator — a RegisteredUser (role 9) whose site role grants
`chat.moderate`, for whom the server sends `can_moderate: true`. The app is
now the grmob (Go) port in `../church_mobile`.

## Finding: the check failed

- Server side is correct: `authz.CanModerate` counts the legacy role rule or
  a role granting `chat.moderate`, and `resource/apitoken/api.go` sends it as
  the additive `can_moderate` on login and `/auth/me`
  (`resource/chat/web.go` sends it in the web chat's `me` too).
- church_mobile `8a71947` (2026-09-13) ported `can_moderate` into the
  **Flutter** model only (`lib/src/models/user.dart` + its test). The grmob
  app's `internal/api.User` never decoded the field, and `CanModerate()` was
  role-only (`Role == 99 || 1..7`). So a permission-only moderator got no
  pin/delete or mark-answered controls in the app that ships, even though the
  server would have allowed the actions.
- The chat and prayer-wall screens (`app/chat.go`, `app/prayer_wall.go`) gate
  everything on `services.Session.CanModerate()` → `User.CanModerate()`, so
  the model is the one place to fix.

This is church_mobile N-002's "every server-contract change is made twice"
cost, realized; N-002's text now records it.

## Fix (church_mobile, `internal/api/models_account.go`)

- `User.ServerCanModerate *bool` with tag `json:"can_moderate,omitempty"`:
  - pointer, so "not sent" (older server) stays distinct from `false`;
  - `omitempty`, so the session store's cached user JSON keeps that
    distinction across a restore (the store persists `api.User` as JSON).
  Same shape as the Flutter model's `serverCanModerate`.
- `CanModerate()`: the server's verdict when present, else the legacy role
  rule (mirrors `authz.LegacyModerator`). Comments updated, with a small
  decision diagram.

## Tests (church_mobile)

- `internal/api`: `TestCanModeratePrefersTheServerFlag` (permission-only
  member → yes; editor the server refuses → no; older server editor → yes;
  older server member → no) and `TestCanModerateSurvivesTheCacheRoundTrip`
  (absent stays absent through marshal/unmarshal).
- `internal/session`: `TestPermissionOnlyModeratorGetsTheControls` — a fake
  server returns a role-9 user with `can_moderate: true`; the controller
  grants moderation after login, on a cold restore from the cached user, and
  after `/auth/me` lands.
- Mutation check: with the server-flag branch disabled, the first and third
  tests fail; restored, church_mobile's full `go test ./...` passes and
  `go vet` is clean.
- Not walked on a device: church_mobile N-001 (real-data walk) is still
  blocked on `/api/v1` not being live.

## Commits

- church_mobile: model fix + tests, and the N-002 text update in its
  `ai_docs/todo/next-list.md`.
- church: this doc + `next-list.md` (N-021 closed).

## Next

Closed: N-021. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
(church_mobile's list: N-002 updated.)
