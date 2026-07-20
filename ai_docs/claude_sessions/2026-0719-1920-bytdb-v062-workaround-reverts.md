# Session: bytdb v0.6.2 — Workaround Reverts

- **Date:** 2026-07-19 19:20
- **Session ID:** e241e4a9-b64f-4b4e-a681-913cbc3d354c
- **Previous session:** `2026-0719-1841-bytdb-phase1-wire-proof.md`
- **Companion doc:** `ai_docs/fable_bytdb_k8s_readiness.md` (updated this session)

## What happened

bytdb v0.6.2 shipped upstream fixes for the two parser gaps found during Phase 1, so
this session upgraded the church repo to v0.6.2 and reverted both workarounds. All
verification is green: `TestBytdbSchemaStatements`, full `go test ./...`, and the
35-check wire proof (`test_scripts/bytdb_wire_check`) all pass on v0.6.2.

## Changes (church repo)

- `go.mod` / `go.sum` — `bytdb` and `bytdb/pgwire` upgraded v0.6.1 → v0.6.2
  (`go get github.com/rohanthewiz/bytdb/pgwire@v0.6.2`).
- `db/bytdb_schema.go` — event_recurrences CHECKs restored to `BETWEEN`
  (`weekday BETWEEN 0 AND 6`, `week BETWEEN 1 AND 4 OR week = -1`), matching the goose
  originals; the >=/<= deviation note removed from the header comment.
- `resource/chat/queries.go` — both `RecentMessages` branches bind `LIMIT` with `$n`
  placeholders again (no more `fmt.Sprintf` interpolation); unused `fmt` import dropped.
- `resource/prayerwall/prayerwall.go` — `ListRequests` binds `LIMIT $1 OFFSET $2`
  again; unused `fmt` import dropped.
- `ai_docs/fable_bytdb_k8s_readiness.md` — both findings marked RESOLVED (v0.6.2,
  2026-07-19); dependency line bumped; the "upstream these fixes" next-step checked off.

## Verification

- `go test ./db/ -run TestBytdbSchemaStatements` — every DDL statement, including the
  restored BETWEEN CHECKs, runs clean on a scratch v0.6.2 engine.
- `go test ./...` — green across the module.
- `go run ./test_scripts/bytdb_wire_check` — all 35 checks pass; this drives the real
  chat/prayerwall query functions, so the placeholder-bound LIMIT/OFFSET paths are
  exercised over the actual lib/pq ↔ pgwire loopback.
- `go vet` clean on the edited packages.

## Notes

- cema/ccswm do not pin bytdb directly — they resolve it transitively through the
  church module via go.work, so no sibling-repo bumps were needed.
- `gofmt -l` flags `db/connect2.go`; pre-existing, untouched this session.
- Local `go` is still 1.25.4 (go.work wants ≥1.26.1) — builds auto-download the
  toolchain, but gopls emits packages.Load errors until the local Go is upgraded.

## Next steps (unchanged from Phase 1, minus the upstreaming item)

1. Boot a site end-to-end on bytdb; exercise SQLBoiler admin flows (article first).
2. `pg_dump --data-only` → bytdb import script for cutover of existing site data.
3. Per-site k8s manifest: Deployment (Recreate) + PVC (linode-block-storage) + backup CronJob.
4. Design `bytdb/replicate` (WAL shipping to S3-compatible storage).
