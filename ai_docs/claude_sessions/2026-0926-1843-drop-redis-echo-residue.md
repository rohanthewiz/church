# Drop the remaining Redis and Echo residue

Session ID: `31482bfa-94bf-480e-b758-b25e316eadc3`
Date: 2026-09-26

## Goal

Remove every remaining trace of Redis (code and config) and the Echo web
server from church, then from the two sites that pin it (cema, ccswm), and
delete the stray `cema/dump.rdb`.

## Findings

- The functional removal had already been done: sessions/tokens moved to
  `core/kvstore` (`2026-0418-replace-redis-with-kvstore`) and Echo was replaced
  by rweb (`2026-0707-0650-echo-removal-ccswm-rweb-migration`). Neither Redis
  nor `labstack/echo` appears in any `go.mod` / `go.sum`.
- What was left was residue: a tracked `router_echo.go.bak` (204 lines, still
  importing `labstack/echo`), a commented-out `Redis` config struct, commented
  `roredis.InitRedis` blocks in both sites' `main.go`, `redis:` blocks in the
  sites' options files, READMEs that still listed Echo and Redis as the stack
  and as install requirements, and comments describing kvstore in terms of
  roredis.
- `config.InitConfig` reads options with non-strict `yaml.Unmarshal`, so a
  `redis:` block in any site's `options.yml` is ignored rather than rejected.
  Removing the struct can't break a site that still has the block.

## Changes

### church

- `git rm router_echo.go.bak`.
- `config/config.go`: removed the commented-out `Redis` struct and the comment
  that kept it "for reference".
- Comments now describe the current behaviour instead of roredis/Echo:
  `core/kvstore/kvstore.go` (package doc, `Set` zero-TTL rule, `Del`
  no-op-on-absent), `core/kvstore/kvstore_test.go`,
  `resource/auth/random.go` and `random_test.go` ("kvstore keys"),
  `arch_test_scripts/grid_preview/main.go`,
  `payment_controller/payment_recorder.go` ("free of HTTP-framework imports"),
  `payment_controller/payment_controller_rweb.go` (dropped the "Echo twin"
  note).
- `README.md`: dropped the "originally ran on Echo" and "(Redis)" asides.
- `deploy/docker/Dockerfile.dockerignore`: removed the `**/dump.rdb` guard,
  since the only such file is now deleted.
- `CEMA_LOCAL_SERVER.md` (local-only, excluded from git): the diagram no
  longer mentions a `redis:` block.
- Left on purpose: the "moving this to Redis" comments in
  `db/migrate/20170419004813_CreateUsersTable.sql`, because applied migrations
  shouldn't be edited, and the history in `ai_docs/`.

### cema and ccswm

- `main.go`: removed the commented-out `roredis.InitRedis` block, plus cema's
  commented roredis import.
- `cfg/options-sample.yml`: removed the `redis:` block from both.
- `cema/cfg/options.yml` (git-ignored, live dev config): removed the `redis:`
  block.
- `README.md`: the intro and Architecture section now say RWeb instead of
  Echo, the sessions line points to the in-process kvstore, and Redis is gone
  from the requirements and install steps. cema also dropped its "we are now
  on rweb" TODO line.
- `cema/.gitignore`: removed the `dump.rdb` entry and its comment.
- Deleted `cema/dump.rdb` (an 88-byte stray Redis dump).
- ccswm's pre-existing uncommitted `.gitignore` change (`.cats-todo`) was not
  made in this session and was left out of the commit.

## Verification

- church: `go build ./...`, `go vet` on the touched packages, and the full
  `go test ./...` all pass.
- cema and ccswm: `go build` and `go vet` pass.
- A case-insensitive search for `redis|labstack|dump.rdb` across all three
  repos (excluding `ai_docs/`, `node_modules/`, `dist/`) finds only the
  migration comments that were left on purpose.

## Next

Closed: None. Declined: None. Raised: N-056.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
