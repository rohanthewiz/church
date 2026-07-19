# Session: bytdb Phase 1 — Default Backend + Wire Proof

- **Date:** 2026-07-19 18:41
- **Session ID:** dc128015-5db6-4314-b6bf-6e490c639d73
- **Companion doc:** `ai_docs/fable_bytdb_k8s_readiness.md` (full assessment; updated this session)

## What happened

Assessed replacing Postgres with `github.com/rohanthewiz/bytdb`, concluded it is nearly a
drop-in via the pgwire loopback, then implemented Phase 1: bytdb is now the **default
backend** for cema and ccswm, with Postgres as an explicit fallback. The phase-1 wire
proof (`test_scripts/bytdb_wire_check`) passes all 35 checks; full `go test ./...` green.

## Key decisions

1. **Wire-over-loopback architecture**: the app keeps `lib/pq` and dials an in-process
   pgwire listener. SQLBoiler models and all raw `$n` queries run unchanged; only the
   endpoint differs. Hot paths can later move to the embedded `bytdb/sql` API piecemeal.
2. **No migration tool on the bytdb path** (per Ro): the goose chain was flattened into
   an in-code idempotent bootstrap (`db/bytdb_schema.go`), created per-table when missing
   via `information_schema.tables`. Goose remains authoritative for the PG fallback only.
3. **Postgres fallback stays**: `db.type: postgres` in options.yml (or `DB_TYPE=postgres`)
   restores the old path untouched.
4. **Deployment target** (from the assessment): one pod per site on shared LKE; bytdb
   data file on a Linode Block Storage PVC (~$1/mo). Live WAL must never sit on object
   storage (no honest fsync); object storage is for backups / future WAL shipping.
   `Engine.Backup/BackupTo/ReadLogRange/LogState` already exist as primitives.

## Changes (church repo)

- `db/connect.go` — bytdb branch: open engine → bootstrap schema → listen-then-serve
  pgwire on loopback (no readiness race) → dial with lib/pq → fail-fast ping.
  `DBTypes.BytDB`, `DBOpts.File/Listen`, `BytDBWireAddr()`. CloseDB drains server before
  engine.
- `db/bytdb_schema.go` — consolidated 12-table schema + indexes. Deviations from goose:
  goose_db_version omitted; user_id FK indexes added on chat/prayer (bytdb cascade probes
  need them); CHECKs use `>=`/`<=` (bytdb v0.6.1 won't parse BETWEEN in CHECK).
- `db/bytdb_schema_probe_test.go` — every DDL statement regression-tested on a scratch engine.
- `config/config.go`, `config/env_overrides.go` — `db: {type, file, listen}` block;
  `DB_TYPE`/`DB_FILE`/`DB_LISTEN` env overrides. Defaults: bytdb, `data/church.db`,
  `127.0.0.1:0`.
- `resource/chat/queries.go`, `resource/prayerwall/prayerwall.go` — LIMIT/OFFSET now
  interpolated typed ints (bytdb rejects placeholders there; injection-safe; PG-compatible).
- `test_scripts/bytdb_wire_check/` — drives the REAL query functions (chat, prayerwall,
  apitoken, event recurrence) plus wire assumptions: StringArray `{a,b}` binding/scan on
  `text[]`, lossless timestamptz round-trip, jsonb `->>`, RETURNING, ON CONFLICT upsert,
  UNIQUE, `array_to_string` + ILIKE sermon search, FK ON DELETE CASCADE fan-out.
- `go.mod` — bytdb v0.6.1 + pgwire v0.6.1; module now `go 1.26.1`.
- `ai_docs/fable_bytdb_k8s_readiness.md` — Phase 1 recorded; next steps updated.

## Changes (sibling repos)

- `cema/main.go`, `ccswm/main.go` — DBOpts built from the new `db:` config block with PG
  fallback branch; cema's dead `roredis.InitRedis` commented out (kvstore replaced Redis).
- `cema/go.work` bumped to go 1.26.1; a root `~/projs/go/church/go.work` was added for
  workspace builds across the three modules (parent dir is not a git repo).

## bytdb findings (upstream candidates)

1. `BETWEEN` doesn't parse inside CHECK constraints (worked around).
2. Placeholders rejected in LIMIT/OFFSET — `pq: XX000` (worked around; SQLBoiler
   unaffected since it emits literals).
3. Every table requires a PRIMARY KEY (schema already complies).
4. Earlier this session: `OWNER TO` no-op shipped upstream as bytdb v0.6.1 / pgwire v0.6.1.

## Environment notes

- bytdb needs Go ≥ 1.26.1; local `go` is 1.25.4 — builds auto-download the toolchain but
  gopls complains until the local Go is upgraded.
- `data/` (default bytdb file location) should be gitignored in the site repos before
  first run.
- cema repo has untracked `token.txt` and `dump.rdb` — left uncommitted deliberately.

## Next steps

1. Boot a site end-to-end on bytdb; exercise SQLBoiler admin flows (article first).
2. `pg_dump --data-only` → bytdb import script for cutover of existing site data.
3. Per-site k8s manifest: Deployment (Recreate) + PVC (linode-block-storage) + backup CronJob.
4. Design `bytdb/replicate` (WAL shipping to S3-compatible storage).
5. Upstream the BETWEEN-in-CHECK and LIMIT/OFFSET placeholder fixes in bytdb.
