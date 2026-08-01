# Session: WAL-Shipping Replication — Implementation

- **Date:** 2026-08-01 08:52
- **Session ID:** `15f95b8a-9021-4c57-8cb3-2c9a39a707ae`
- **Previous session:** `2026-0719-2010-wal-shipping-integration-design.md`
- **Implements:** `ai_docs/wal_shipping_integration_design.md` (now marked IMPLEMENTED)
- **Also updated:** `ai_docs/fable_bytdb_k8s_readiness.md` next-step 4 closed, new item 6

## What happened

The design from 2026-07-19 got built, on a newly bumped **bytdb v0.8.0** (the user's
first instruction was to upgrade before implementing — the repo was on v0.6.2, which is
what the design doc was written against). RPO on volume loss drops 1 h → ~5 s.

## The v0.8.0 upgrade

`go get github.com/rohanthewiz/bytdb@v0.8.0` + `.../bytdb/pgwire@v0.8.0`; pulled
`btypedb` v0.6.0 → v0.7.0 transitively. Everything built clean with no source changes,
and `go run ./test_scripts/bytdb_wire_check` re-passed all 35 checks — the phase-1 query
surface (text[] binding, timestamptz, jsonb, RETURNING, ON CONFLICT, cascade fan-out) is
unaffected by the two-minor-version jump. ccswm/cema needed no bump: they resolve bytdb
transitively through the church module via go.work.

## Files

**New**

- `db/replicate.go` — `StartBytDBReplication()`, `BytDBReplicationStatus()`,
  `closeBytDBReplication()`, `restoreIfMissing()` (pure, over `replicate.Storage`),
  `restoreBytDBIfMissing()` (config-reading wrapper), `writeFileAtomic()`,
  `backupStore()`, `replicateInterval()`, `replicationConfigured()`, and the
  `ReplicationStatus` DTO.
- `db/replicate_test.go` — restore-precedence matrix + config gating.
- `db/replicate_integration_test.go` — end-to-end against a fake S3 over real HTTP.

**Modified**

- `config/config.go` — `Replicate bool`, `ReplicateInterval string` on the `Backup` struct.
- `config/env_overrides.go` — `BACKUP_REPLICATE` (affirmative-only parse),
  `BACKUP_REPLICATE_INTERVAL` (left unvalidated; db parses it).
- `db/connect.go` — `restoreBytDBIfMissing(file)` before `bytdb.Open` in `startBytDB()`;
  `closeBytDBReplication()` as the FIRST step of `CloseDB()`.
- `resource/dbbackup/api_rweb.go` — gate chain extracted to `gateOps()`; new
  `APIReplicationStatusRWeb`.
- `resource/dbbackup/dbbackup.go` — package doc recast from "interim replication story"
  to "the SNAPSHOT tier", with the WAL keys listed as the other tier.
- `resource/dbbackup/api_contract_test.go` — `TestReplicationStatusGates`.
- `router_rweb.go` — `s.Get("/api/admin/db/replication", ...)`.
- `ccswm/main.go`, `cema/main.go` — `db.StartBytDBReplication()` after `InitDB`,
  logged-not-fatal.
- `deploy/k8s/sites/{ccswm,cema}.yaml` — `BACKUP_REPLICATE=true` env in;
  `restore-if-empty` initContainer commented out (not deleted); CronJob comment recast.
- `deploy/k8s/README.md` — bucket-layout table, WAL-shipping section, lost-volume
  recovery section; migration runbook step 5 rewritten.

## Three deltas from the design, all caused by v0.8.0

1. **`ErrIncompleteReplica` is new.** v0.8.0 added per-generation `manifest.json`
   certifying a generation was once shipped complete, and `Restore` now returns
   `(*RestoreInfo, error)`. That creates a third outcome the design didn't have:
   manifested generations exist but have lost chunks. Treated as a **hard abort**, not a
   fallback to the hour-old snapshot — a silent rollback is precisely what the manifest
   exists to prevent, and that trade is an operator's call. Recovery = put a file on the
   volume by hand, after which the in-app restore sees it and stands down.
2. **Cold-start restore is gated on the destination, not on `replicate: true`.** Pulling
   `latest/` onto an empty volume was the initContainer's job; it must keep working on
   sites that never opt into WAL shipping, otherwise removing the initContainer would
   regress them.
3. **Status endpoint returns a DTO** (`db.ReplicationStatus`), not `replicate.Status` —
   the latter carries a bare `error` field, which marshals to `{}`. Also answers **503**
   when replication is off rather than 200-with-a-flag: a monitor pointed at a site that
   silently stopped replicating should get a hard failure.

## Other decisions worth remembering

- **`db` now imports `config`.** Verified no cycle — `config` is a leaf (stdlib +
  yaml.v2 only, no church imports). This is what lets `StartBytDBReplication()` take no
  arguments (design's shape) and lets the restore run inside `InitDB`, before any site
  code gets a chance to pass options in. Testability preserved by keeping
  `restoreIfMissing` pure over `replicate.Storage`.
- **Snapshot-absent vs store-unreachable is distinguished by `List`, not by error text.**
  `replicate/s3` surfaces a 404 as a generic `serr` with a `status` attribute — no typed
  sentinel — and matching on message text would be a trap. `List(ctx, exactKey)` then an
  exact-match scan is unambiguous and costs one request.
- **Two S3 clients now exist against one bucket** (aws-sdk-v2 in dbbackup, stdlib SigV4
  `replicate/s3` in db). Deliberately not consolidated in this change so a replication
  bug and an SDK swap can't be confused. Tracked as a follow-up.
- **`fmt`, not `logger`, inside `db`** — kept the existing package convention; the
  replicator's `Logf` is an adapter around `fmt.Printf`.
- Chunk size (8 MB) and `RetainGenerations` (3) keep upstream defaults, unexposed in
  config: MB-scale church DBs fit one chunk per generation.

## Tests

- `db/replicate_test.go` (9 tests): existing file untouched **and the store never
  contacted** (fake fails every op); WAL beats snapshot (different `note` values make the
  chosen source observable); snapshot fallback on `ErrNoReplica`; fresh site is a silent
  no-op with no file created; unreachable store aborts leaving no file;
  `ErrIncompleteReplica` aborts (built by deleting a `.wlog` while leaving its manifest);
  `replicationConfigured` gating; `replicateInterval` parsing table; the three no-op
  paths of `StartBytDBReplication`.
- `db/replicate_integration_test.go`: `fakeS3` (path-style PUT/GET/DELETE +
  ListObjectsV2 XML, rejecting unsigned requests) behind `httptest`, driven by the real
  `replicate/s3` client. Scenario: boot on empty volume → write rows → real Run loop
  ships at 100ms → assert keys land only under `<prefix>/wal/` and a manifest exists →
  `CloseDB` → delete the data file → `InitDB` again → the row is back. Log line proves
  it: `restored from WAL generation 20260801t… (37526 bytes, 1 chunks)`.
- `resource/dbbackup/api_contract_test.go`: gate matrix re-run against the new endpoint
  (the guarantee is per-endpoint, so trusting the shared helper would defeat the point).
- Verification: `go test ./...` green; `go test -race ./db/` green; `go vet ./...` clean
  in all three modules; `gofmt` clean on every touched file.

## Environment notes

- `gofmt -l` flags ~40 files repo-wide (pre-existing, incl. `router_rweb.go` and
  `db/connect2.go`) — verified via `git stash` that `router_rweb.go` was already
  unformatted before this change. Left alone.
- gopls still emits `packages.Load` errors: go.work wants ≥1.26.1, local go is 1.25.4
  (builds auto-download the toolchain). Pre-existing.
- `strings.Builder` has no `ReadFrom` — use `io.ReadAll` in HTTP fakes.
- Go type parameters cannot be constrained on struct fields
  (`[T interface{ Key string }]` does not compile); use `sort.Slice` on the concrete type.

## Next steps

1. **Readiness item 1, still open:** boot a site end-to-end on bytdb and exercise the
   SQLBoiler admin flows (article first). Last unchecked item before a real cutover.
2. Provision LKE + Object Storage; install ingress-nginx + cert-manager; create per-site
   secrets; then the cutover dry-run per `deploy/k8s/README.md` — which now also verifies
   generations appear under `<prefix>/wal/gen/` and `GET /api/admin/db/replication`
   reports a small `lag_seconds`.
3. Non-blocking follow-ups (readiness doc item 6): SIGTERM → `CloseDB()` in
   `church.ServeRWeb()`; migrate `resource/dbbackup` onto `replicate/s3` and drop the
   aws-sdk-go-v2 dependency.
