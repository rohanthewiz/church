# Drop the `_rweb` / `RWeb` suffixes (N-056) and release v0.12.0

Session ID: `a582efc1-b481-4003-996f-33648f44b3ef`
Date: 2026-09-26

## Goal

Work next-list item N-056: with Echo gone there is one web stack, so the
`_rweb` file suffixes and `RWeb` identifier suffixes no longer tell anything
apart. Rename in church, release, and re-pin cema and ccswm.

## Findings

- 36 `*_rweb*.go` files (the list said 37), none of which collided with an
  existing file once the suffix was dropped.
- The only exported name the sites call is `church.ServeRWeb()` (one call in
  each site's `main.go`); nothing in church_mobile uses the names.
- `rweb` (lowercase, ~355 hits) is the package name
  `github.com/rohanthewiz/rweb` and must stay; standalone "RWeb" in comments is
  the framework name and stays too. `context/rweb_helpers.go` is prefixed, not
  suffixed, and was left as is.
- Three names would have collided with existing functions in the same package
  once `RWeb` was stripped:
  - `app.VerifyFormTokenRWeb` vs `app.VerifyFormToken(token)`
  - `chat.DeleteMessageRWeb` vs the query `chat.DeleteMessage(exec, id)`
  - `prayerwall.DeleteRequestRWeb` vs the query `prayerwall.DeleteRequest`
- Past church tags were cut on whatever commit was head (v0.11.x); this rename
  is a breaking API change, so it took a minor bump.

## Changes

### church (`a16be77`, `1537a21`, tag `v0.12.0`)

- `git mv` of all 36 files, dropping `_rweb` (e.g. `router_rweb.go` →
  `router.go`, `resource/chat/web_rweb.go` → `web.go`).
- Identifier renames, applied with one perl pass over tracked `.go` files:
  - default: strip `RWeb` wherever it appears inside an identifier
    (`ServeRWeb` → `Serve`, `RedirectRWebError` → `RedirectError`,
    `flash.GetRWeb` → `flash.Get`, `(Flash).SetRWeb` → `Set`)
  - `context.*FromRWeb` / `SetSessionInRWeb` → `GetSession`, `IsAdmin`,
    `GetUsername`, `ClearSession`, `SetSession`
  - collisions: `app.VerifyRequestFormToken`; chat web handlers
    `WebListMessages`, `WebPostMessage`, `WebKeepMessage`, `WebDeleteMessage`;
    prayer wall web handlers `WebPostRequest`, `WebMarkAnswered`,
    `WebDeleteRequest` (mirrors the `API…` prefix the API handlers use)
  - `*_rweb.go` / `*_rweb_test.go` file references inside comments
- Rewrote three comments in `auth_controller/auth_middleware.go` that only said
  "RWeb version …"; README roadmap names updated (`AdminGuard`, `Serve`,
  `basectlr.SendAudioFile`). Historical `ai_docs` analysis/session docs left
  unchanged.
- N-056 moved to Closed in `ai_docs/todo/next-list.md`.

### cema (`a337548`), ccswm (`a53674f`)

- `church.ServeRWeb()` → `church.Serve()` in `main.go` (ccswm's stale comment
  about the deprecated Echo `church.Serve` replaced).
- `go get github.com/rohanthewiz/church@v0.12.0` + `go mod tidy`; only the
  church line changed in each `go.mod`.
- ccswm's pre-existing uncommitted `.gitignore` edit (`.cats-todo`) was left
  out of the commit and is still uncommitted.

## Verification

- church: `gofmt`, `go build ./...`, `go vet ./...`, `go test ./...` clean.
- Sites: built against the workspace, then again with `GOWORK=off` against the
  published v0.12.0.
- CI green: church master and tag runs, cema, ccswm.

## Gotchas

- In zsh an unquoted `$files` is not word-split, so `perl -pi … $files` got
  one giant filename and edited nothing. Use `git ls-files -z | xargs -0`.
- A stray `git stash` got appended to a command mid-rename; `git stash pop`
  restored everything (renames came back as A/D pairs until `git add -A`).
  Nothing was lost, but keep verification-only commands free of state changes.

## Next

Closed: N-056. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
