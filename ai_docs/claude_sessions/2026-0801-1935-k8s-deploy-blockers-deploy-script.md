# Session: LKE Deploy Audit — Blocker Fixes + deploy.sh

- **Date:** 2026-08-01 19:35
- **Session ID:** 88439206-0e4c-408e-bb2b-a79181e4dda2
- **Companion docs:** `deploy/k8s/README.md` (rewritten install section),
  `ai_docs/fable_bytdb_k8s_readiness.md`, `ai_docs/wal_shipping_integration_design.md`
- **Prior deploy sessions:** `2026-0719-1955-k8s-deploy-backup-endpoint-pg-importer.md`,
  `2026-0801-0852-wal-shipping-implementation.md`

## What happened

Audited the existing `deploy/` tree (manifests written 2026-07-19, updated
2026-08-01 for WAL shipping) against the actual runtime behavior of the site
binaries. The architecture held up; the gap was between the manifests and what
the process needs to boot. Twelve findings, then fixed the blockers and wrote
the missing deploy script.

**Nothing was deployed** — no cluster was contacted. Docker daemon was down, so
the image build is the one fix that remains unproven.

## 1. Audit findings (evidence-backed)

Blockers — nothing would have come up:

1. **Dockerfile could not build.** `resource/chimage/image.go` imports
   `h2non/bimg`, a cgo binding to libvips, while the Dockerfile set
   `CGO_ENABLED=0` with the comment "CGO not needed". Reproduced:
   `CGO_ENABLED=0 go build -C ccswm .` → `undefined: Gravity`, `undefined:
   Options`, … `CGO_ENABLED=1` builds fine (37 MB, dynamically linked). The
   "static binary" claim in the header comment and the prior session doc was
   wrong.
2. **`cfg/random_seeds.txt` never delivered.** `resource/auth/random.go:18`
   opens it in `init()` and `log.Fatal`s — before `main()`. The Dockerfile made
   an empty `/app/cfg`; the Deployment `subPath`-mounted only `options.yml`.
   Unconditional crash loop.
3. **`APP_ENV` never set.** `config.InitConfig` defaults `AppEnv` to
   `development`, so pods would read the `development:` section of options.yml;
   `buildTLSCfg` also short-circuits on that value.
4. **Port mismatch, no env override.** Manifests hardcode 4000; cema's
   `production.server.port` is 8088 and ccswm's sample is 80 + `use_tls: true`
   with certbot paths. `config/env_overrides.go` had no `SERVER_PORT`/`USE_TLS`.
5. **`ccswm/cfg/options.yml` does not exist** (gitignored; only the sample), so
   the README's `--from-file=...` secret command failed outright.

Data loss / correctness:

6. **Uploaded images ephemeral.** `resource/chimage/image.go:19` writes to
   `dist/img/` → container writable layer. Every redeploy broke `<img
   src="/assets/img/…">` in existing articles.
7. **ingress-nginx defaults break SSE.** `/chat/stream` (`router_rweb.go:210`)
   vs. default `proxy_buffering on` + 60s read timeout.
8. **Liveness probe could kill a cold-start restore.** Restore runs in
   `startBytDB()` before the port binds, up to a 2-minute timeout;
   `initialDelaySeconds: 15` would restart the pod mid-recovery.
9. **`OBJ_REGION` absent from the README secret.** Both S3 clients
   (`db/replicate.go:115`, `resource/dbbackup/dbbackup.go:89`) default to
   `us-east-1`; a wrong region fails SigV4 and both tiers stop *silently*.

Hygiene:

10. **No `.dockerignore`; context 2.9 GB** (church_mobile 2.6 G, cema/sermons
    92 M, 53 M of zips) — and `COPY cema/ cema/` pulled live `cfg/options.yml`
    credentials into the build stage.
11. **Nested `cema/go.work`** (`use (. ../church)`) — `go build -C cema`
    resolves it first, so the cema image built against a different workspace
    than ccswm, dragging in a gitignored `go.work.sum`.
12. Minor: no `imagePullSecrets`; probes hit `/` (full render + DB query every
    10s); no www→apex redirect (split cookie jars).

## 2. Fixes

### Go

- `config/env_overrides.go` — `SERVER_PORT` and `USE_TLS` overrides.
  Affirmative-only bool parsing, matching the `BACKUP_REPLICATE` precedent.
  This is what lets the manifest own the listener shape while each site's
  production section keeps its bare-metal values.
- `router_rweb.go` — `GET /healthz`, `ctx.WriteText("ok")`. No session
  middleware, no DB, no render. Comment records why DB health is deliberately
  *not* reported: at one replica, failing readiness has nowhere to shift
  traffic and only converts degraded into a hard 503.

### Manifests (`deploy/k8s/sites/{ccswm,cema}.yaml`)

- `APP_ENV=production`, `SERVER_PORT=4000`, `USE_TLS=false`.
- Config Secret mounts as a **whole directory** over `/app/cfg` (two files
  needed, so `subPath` was structurally wrong), `defaultMode: 0440` + existing
  `fsGroup: 1000`.
- `/app/dist/img` ← `subPath: uploads` of the same block PVC. One device, no
  second claim. Documented asymmetry: uploads are **not** shipped to object
  storage; only the `-retain` class protects them.
- `startupProbe` 30 × 5s = 150s of headroom over the 2-minute restore;
  readiness/liveness moved to `/healthz` and parked behind it.
- Ingress SSE annotations: `proxy-buffering: "off"`,
  `proxy-read-timeout`/`proxy-send-timeout` 3600.
- CronJob container gained resource requests/limits.
- cema generated from ccswm by sed (domains first, then name) so the two stay
  structurally identical; only the schedule offset and the port comment differ.

### Dockerfile

- `CGO_ENABLED=1`; builder adds `build-base vips-dev`, runtime adds `vips`.
  Header comment now explains the constraint instead of asserting the opposite.
- `rm -f cema/go.work cema/go.work.sum` before build.
- `mkdir -p /app/dist/img` as a mount point; comment marks `cfg/` as
  Secret-supplied and names `random_seeds.txt` as non-optional.
- New `deploy/docker/Dockerfile.dockerignore` — BuildKit's per-Dockerfile
  ignore convention, chosen because the build context is the workspace parent,
  which is **not a git repo**, so a `.dockerignore` there could not be
  version-controlled. Excludes credentials first, then bulk.

### `deploy/deploy.sh` (new, executable)

Eight idempotent phases: `preflight → infra → base → seeds → secrets → images
→ sites → verify`, plus `all`. Written for bash 3.2 (macOS): no associative
arrays, no `mapfile`.

Design decisions worth keeping:

- **Site domains are parsed out of the Ingress manifest**, never duplicated in
  the script — the YAML stays single source of truth.
- **DNS precheck before applying an Ingress.** Let's Encrypt rate-limits failed
  authorizations 5/hour/hostname, so a premature apply costs an hour. Warns and
  prompts rather than proceeding.
- **`seeds` generates `random_seeds.txt` only when absent.** Safe on a live
  site: the pool is entropy for *new* salts/tokens and each user's salt is
  stored beside their hash — verified by reading `resource/auth`
  (`randStrings` feeds only `RandomString()` → `RandomKey()`/salt inputs).
- **`secrets` validates `production:` exists** but deliberately does *not*
  check port/TLS, because the env overrides now own those.
- **`BACKUP_TOKEN` is reused from the cluster**, minted only on first deploy —
  rotating it silently would break the CronJob's next call.
- **`OBJ_REGION` required**, not defaulted, precisely because its failure mode
  is silent.
- **Image tag = site's git SHA** (`-dirty` suffix when the tree is dirty),
  substituted over the manifest's `:latest` placeholder with sed at apply time
  — pinned running spec, readable committed YAML, no templating dependency.
- `DOCKER_BUILDKIT=1` exported in `images` (mandatory, per the ignore-file
  choice above).

- `deploy/backup.env.sample` added; `deploy/backup.env` + `backup.*.env`
  gitignored.

## 3. Verification

Passing:

- `go build` both sites (CGO_ENABLED=1); `go test ./config/...
  ./resource/dbbackup/... ./db/...`
- All four manifests YAML-parse; image substitution round-trips and leaves the
  commented-out `minio/mc` initContainer and the curl image untouched
- `bash -n` under macOS bash 3.2; `--help`; domain parsing; `preflight` fails
  cleanly with no cluster
- **Config overrides proven end-to-end** with a throwaway probe (created, run,
  deleted): no env → `env=development port=3000`; `APP_ENV=production` alone →
  `port=8088 useTLS=true` (the cema mismatch, reproduced); with both overrides
  → `port=4000 useTLS=false`

Not verified:

- **The container image build** — Docker daemon was not running. The CGO
  failure is confirmed and the vips packaging is standard, but whether bimg
  v1.1.9 compiles against Alpine 3.20's libvips 8.15 is unproven until
  `./deploy/deploy.sh images` runs.

## Environment/API notes

- Debugging note worth remembering: `bash -n` failed with a misleading
  "syntax error near unexpected token `('" pointing ~85 lines past the real
  cause. The culprit was an **apostrophe inside `${var:?word}`** — bash parses
  that word with quote awareness even within double quotes, so a lone `'`
  opens a quoted section and swallows the rest of the file. Bisecting prefixes
  at function boundaries located it.
- `rweb.Context` has `WriteString` (no Content-Type) and `WriteText`
  (text/plain); used the latter.
- Pre-existing and untouched: `router_rweb.go` already failed gofmt at HEAD
  (import ordering of `resource/payment`, comment alignment in the sermons
  block) — confirmed via `git show HEAD:router_rweb.go | gofmt -l`, so not
  introduced here. Also `db/connect2.go` fails gofmt; gopls complains that
  go.work wants 1.26.1 vs local go 1.25.4 (builds still resolve a newer
  toolchain).
- Found but deliberately out of scope: `resource/auth`'s `init()` prints the
  first 50 crypto seeds to stdout on every boot (they land in pod logs), and
  `AuthBootstrap` writes `token.txt` with `os.ModePerm` (0777).

## Next steps

1. Start Docker and run `./deploy/deploy.sh images` — the one unproven fix.
   If bimg/libvips 8.15 fights, options are pinning an older Alpine or
   replacing bimg with a pure-Go resizer (which would also restore
   `CGO_ENABLED=0` and a static image).
2. Provision LKE + Object Storage, fill `deploy/backup.env`, then
   `./deploy/deploy.sh preflight infra` → point DNS at the NodeBalancer IP →
   `./deploy/deploy.sh base seeds secrets images sites verify`.
3. Create `ccswm/cfg/options.yml` from the sample (cema already has one).
4. Durable fix for uploaded images: move `resource/chimage` onto IDrive e2
   beside the sermon media, retiring the `dist/img` volume mount.
5. Still open from the readiness doc: SIGTERM → `CloseDB()` in `ServeRWeb`;
   migrate `resource/dbbackup` off aws-sdk-go-v2 onto `replicate/s3`.
6. Optional hardening surfaced by the audit: `imagePullSecrets` if the ghcr
   packages go private, www→apex redirect, and the two `resource/auth` issues
   above.
