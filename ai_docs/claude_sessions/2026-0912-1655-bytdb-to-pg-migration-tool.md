# Session: bytdb → Postgres migration tool

**Session:** https://claude.ai/code/session_01XmyuJDjLKSFLe5DSTmjVH6
**Date:** 2026-09-12

## What happened

1. **Question:** does church still fully support Postgres alongside bytdb, so a
   site can pick either backend and migrate data from the other?
2. **Answer (audit):** yes at runtime, but migration only went Postgres → bytdb,
   and the Postgres path had not been exercised since bytdb became the default
   (2026-07-19).
3. **Built** `test_scripts/bytdb_to_pg` — the reverse of `pg_to_bytdb`.
4. **Found and fixed** a latent `pg_to_bytdb` bug: any `date` column value
   (`event_recurrences.until`) aborted the cutover.
5. **Verified** with a full Postgres → bytdb → Postgres round trip plus
   refusal/rollback/immutability tests on throwaway databases (since dropped).

## 1. Postgres support audit (evidence)

- **Backend switch:** `db.type` in options.yml or `DB_TYPE` env
  (`config/env_overrides.go`). Empty ⇒ bytdb (`db/connect.go:112`). `cema/main.go`
  and `ccswm/main.go` both still pass the `PG.*` settings through.
- **One data-access layer:** bytdb is embedded and served over loopback pgwire, so
  SQLBoiler models and raw `$n` SQL use `lib/pq` against either backend.
- **Schema parity:** scripted comparison of column sets, goose migrations
  (`db/migrate/*.sql`) vs bytdb bootstrap (`db/bytdb_schema.go`): all 14 tables
  identical. Migrations are still maintained (`event_locations`, commit 021b6ee,
  landed in both).
- **bytdb-only features:** `POST /api/admin/db/backup` (503 on Postgres — use
  `pg_dump`), WAL replication (`db/replicate.go`), self-heal restore on an empty
  volume. k8s manifests hardcode `DB_TYPE=bytdb`; no Postgres in the cluster.
- **Caveat:** no commit, test, or smoke run has exercised Postgres since
  2026-07-19. chat/prayer wall, recurrence, event locations were only proven on
  bytdb. Queries stick to the shared dialect subset (`RETURNING`, `ON CONFLICT`,
  `ILIKE`, `array_to_string`, `::date`), so breakage is unlikely but unverified.
- **Postgres schema** is still goose-managed; bytdb bootstraps in code.

## 2. `test_scripts/bytdb_to_pg/main.go` (new)

```bash
cd db/migrate && goose postgres "<dsn>" up      # destination schema first
go run ./test_scripts/bytdb_to_pg -src data/church.db -pg "<dsn>"   # or PG_DSN
```

```
-src church.db ──copy──▶ temp snapshot ──db.InitDB (bytdb, loopback pgwire)──┐
                                                                             │ SELECT *
Postgres ◀── one transaction: INSERT per table (FK order) ◀──────────────────┘
         ◀── setval() every serial/identity sequence past MAX(col)
         ◀── per-table count check ── COMMIT (or ROLLBACK on any failure)
```

Design choices:
- **Source is a temp copy, never opened in place.** Opening a bytdb file runs the
  schema bootstrap (may add tables) and may compact. bytdb v0.8.0 (btypedb
  v0.7.0) is a single file with no lock and only a transient compaction sidecar,
  so a file copy is a valid snapshot. Don't point `-src` at a running site's live
  file. `latest/church.db` from object storage or a stopped site's file is fine.
- **Missing `-src` rejected up front**: `bytdb.Open` would otherwise create an
  empty file and "successfully" copy zero rows.
- **Replication/restore inert:** no app config is loaded, so
  `replicationConfigured()` is false and `restoreBytDBIfMissing` is a no-op.
- **Destination preflight across all tables before any write:** every table must
  exist (else: run goose) and be empty (refuses to merge; no truncate flag, on
  purpose).
- **Single Postgres transaction** covers copy + sequence reset + count
  verification ⇒ all-or-nothing, retry needs no cleanup. (`pg_to_bytdb` is only
  per-table atomic.)
- **Sequences reset** (unlike `pg_to_bytdb`, where bytdb identity counters
  self-heal): columns found via `information_schema.columns` where
  `column_default LIKE 'nextval(%' OR is_identity = 'YES'`, so `event_id`-keyed
  tables are skipped naturally. `setval(seq, max, true)`, or `setval(seq, 1, false)`
  when empty.
- **Table order** from `db.BytDBTableNames()` (FK order), same as the importer.
- **Value handling:** `[]byte` → `string` (text, `text[]` literals, jsonb coerce
  by column type). `date` columns → `YYYY-MM-DD` (see §3).
- **Exit codes:** 2 bad args / missing source, 1 migration failure.

## 3. Bug fixed in `test_scripts/pg_to_bytdb/main.go`

`pq: invalid input syntax for type date (XX000)` on `event_recurrences`. `lib/pq`
scans a Postgres `date` into `time.Time`, and re-binding that sends RFC3339, which
bytdb's date parser rejects. Any site with a recurrence that has an `until` date
would have failed cutover. Fix: `rows.ColumnTypes()` flags
`DatabaseTypeName() == "DATE"`, and those `time.Time` values are sent as
`t.Format("2006-01-02")`. The same guard was added to `bytdb_to_pg`: Postgres would
accept RFC3339, but the plain form can't shift a day under a server TimeZone.

## 4. Verification (local Postgres 16 via Homebrew, `/tmp:5432`)

Throwaway DBs `church_b2p_{src,dst,nomig,rollback,immut}`, all dropped afterwards.
`church_development` was only read (`pg_dump --data-only`).

| Test | Result |
|---|---|
| Round trip: dev data + seeded event (quotes, `{braces}`, unicode, `text[]` with comma/quote elements, timestamptz), event_location (double precision), event_recurrence (smallint, date) → `pg_to_bytdb` → `bytdb_to_pg` | Sorted `pg_dump --data-only` of src vs dst **identical** (42 lines). 14 rows across 14 tables. |
| Sequences | every `nextval` = `MAX(id)+1` (users 5/4, events 6/5, api_tokens 7/6, …); empty tables → 1 |
| Non-empty destination | refused, dst users still 2 |
| Destination without migrations | refused with goose hint |
| Mid-copy failure (dropped `event_locations.updated_at`; failed after 10 tables) | all tables back to 0, `users_id_seq.is_called = f` |
| Source immutability | SHA-256 of `-src` identical before/after; temp snapshot dir removed |
| Arg validation | no args → exit 2; missing `-src` → exit 2, no file created; failure → exit 1 |

`go vet` clean on both tools.

## Environment/API notes

- Homebrew `postgresql@16` binaries are not on PATH: `/opt/homebrew/opt/postgresql@16/bin`.
  Server runs as a brew service on socket `/tmp`, roles `ro` and `devuser`.
- `church_development` goose status: `20260911160000_CreateEventLocationsTable.sql`
  is **Pending**. Left alone this session.
- Docker daemon was not running (same as the 2026-08-01 session).
- gopls warns `go.work requires go >= 1.26.1 (running go 1.25.4)`. It's editor-only;
  `go` resolves toolchain go1.26.5 and builds fine.
- Shell is zsh: `${PIPESTATUS[0]}` is empty there (`$pipestatus`). Don't read a
  blank as success.
- `.cats-todo/todos.json` (a personal todo list, untracked before this session)
  was deliberately not committed.

## Next

1. Start Docker and run `./deploy/deploy.sh images`, the one unproven fix from
   2026-08-01. If bimg/libvips 8.15 fights: pin older Alpine or replace bimg with
   a pure-Go resizer (restores `CGO_ENABLED=0` / static image).
2. Provision LKE + Object Storage, fill `deploy/backup.env`, then
   `./deploy/deploy.sh preflight infra` → DNS to NodeBalancer IP →
   `./deploy/deploy.sh base seeds secrets images sites verify`.
3. Create `ccswm/cfg/options.yml` from the sample (cema already has one).
4. Move `resource/chimage` uploads onto IDrive e2 beside sermon media, retiring
   the `dist/img` volume mount.
5. From the readiness doc: SIGTERM → `CloseDB()` in `ServeRWeb`; migrate
   `resource/dbbackup` off aws-sdk-go-v2 onto `replicate/s3`.
6. Optional hardening: `imagePullSecrets` if ghcr packages go private; www→apex
   redirect.
7. `resource/auth` issues (previously out of scope): `init()` prints the first 50
   crypto seeds to stdout on every boot (they land in pod logs), and
   `AuthBootstrap` writes `token.txt` with `os.ModePerm` (0777).
8. **New:** smoke-test a site booted with `DB_TYPE=postgres`, covering chat, prayer
   wall, recurrence, event locations, sermon search, admin flows. Nothing has run
   on Postgres since 2026-07-19. Ideally make `test_scripts/bytdb_wire_check`
   take a Postgres DSN so the same 35 checks prove both backends.
9. **New:** apply the pending `event_locations` goose migration to
   `church_development`.
10. **New:** add `bytdb_to_pg` (and the `pg_to_bytdb` date fix) to
    `ai_docs/fable_bytdb_k8s_readiness.md` §7.
11. **New, optional:** `bytdb_to_pg` checks counts only. A content checksum per
    table would catch silent value coercion. Deliberately skipped for now: the
    round-trip dump diff covered it for the current schema.
12. **Non-goal (declined):** a `-truncate` flag on `bytdb_to_pg`. Refusing
    non-empty destinations is intentional protection for live Postgres sites.
