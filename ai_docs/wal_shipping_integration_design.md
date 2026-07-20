# Design: WAL-Shipping Replication Integration (bytdb/replicate)

- **Date:** 2026-07-19
- **Status:** DESIGN — ready to implement
- **Upstream:** `github.com/rohanthewiz/bytdb/replicate` + `replicate/s3` shipped in
  bytdb v0.6.0 and are present in the v0.6.2 this repo already pins. The package-level
  design (generations, epochs, chunk keys, restore semantics) is settled upstream — see
  bytdb's `ai_docs/claude_sessions/2026-0719-1729-s3-replication.md`. **This document
  designs the church-platform side only**: config, wiring, cold-start restore,
  observability, and the k8s manifest changes.

## 1. Goals and non-goals

**Goals**

- Cut the volume-loss data window (RPO) from 1 hour (hourly snapshot CronJob) to
  ~5 seconds (ship interval + upload time), per site, with zero new dependencies —
  `replicate` and `replicate/s3` live in the already-pinned bytdb module.
- Make a fresh PVC self-heal on boot from the newest replica, in-app, without the
  `minio/mc` initContainer.
- Keep the existing snapshot tier untouched — full snapshots guard against a WAL-chain
  bug and remain the migration-delivery mechanism.

**Non-goals** (doctrine settled upstream: *replica = recovery, not HA*)

- No live failover / read replicas. A restored node comes up from object-store state.
- No point-in-time restore UI (the chunk chain supports it in principle; not needed).
- No change to the Postgres fallback path — replication is bytdb-backend-only, same as
  the backup endpoint.

## 2. Architecture

```
                 pod (per site)                                Linode Object Storage
  ┌───────────────────────────────────────────┐               ┌─────────────────────────┐
  │ site binary                               │               │ bucket: church-backups  │
  │                                           │               │                         │
  │  bytdb engine ──▶ /data/church.db (PVC)   │               │ ccswm/                  │
  │        ▲  │                               │   every ~5s   │  ├ 20260719.../church.db│  ← hourly snapshot
  │        │  └ LogState / ReadLogRange       │   changed     │  ├ latest/church.db     │  ← snapshot CronJob (kept)
  │        │           │                      │   bytes only  │  └ wal/gen/<gen>/       │  ← NEW: replicator
  │  restore-if-empty  ▼                      │               │      0000...-0000...wlog│
  │  (boot)      replicate.Replicator ────────┼──────────────▶│      ...                │
  │        ▲            (replicate/s3 client) │               └─────────────────────────┘
  │        └────────── replicate.Restore ◀────┼── boot, only when /data/church.db absent
  └───────────────────────────────────────────┘
```

The replicator polls the engine's append-only log and PUTs new byte ranges; idle ticks
cost one `LogState()` call and zero requests. Compaction and process restarts roll a new
generation automatically (upstream behavior); `RetainGenerations: 3` bounds bucket growth.

## 3. Configuration

Extend the existing `backup:` block rather than adding a new credential set. Rationale:
WAL chunks and snapshots have the identical blast radius (full DB contents), target the
same bucket, and the k8s `<site>-backup` secret already feeds every consumer. A separate
`replication:` block would duplicate five fields and a secret for no isolation gain.

```yaml
backup:
  endpoint:  us-east-1.linodeobjects.com
  region:    us-east-1
  bucket:    church-backups
  access_key: ...
  secret_key: ...
  prefix:    ccswm
  token:     ...
  retain:    72
  # NEW — WAL shipping (bytdb backend only). Explicit opt-in so adopting a new
  # bytdb version never silently starts writing to the bucket.
  replicate: true
  replicate_interval: 5s   # optional; upstream default 5s. RPO knob.
```

- `Replicate bool` + `ReplicateInterval string` (parsed with `time.ParseDuration`) on the
  `Backup` struct in `config/config.go`.
- Env overrides in `env_overrides.go`, matching the existing naming scheme:
  `BACKUP_REPLICATE` (`true`/`1`) and `BACKUP_REPLICATE_INTERVAL`.
- Effective only when the backend is bytdb AND endpoint/bucket/creds are set — same
  gating shape as the backup endpoint, but `replicate: true` is additionally required.

### Bucket key layout (shared prefix, disjoint namespaces)

| Keys | Writer | Reader |
|---|---|---|
| `<prefix>/<UTC ts>/church.db` | dbbackup (snapshot) | dbbackup prune |
| `<prefix>/latest/church.db` | dbbackup (CopyObject) | boot restore fallback |
| `<prefix>/wal/gen/<gen>/<start>-<end>.wlog` | replicator | `replicate.Restore`, replicator prune |

Verified safe to share the prefix: `dbbackup.prune` only deletes keys shaped exactly
`<prefix>/<timestamp>/church.db` (`resource/dbbackup/dbbackup.go:184-193` skips anything
else), and the replicator's own listing/pruning is scoped to `<prefix>/wal/gen/`
(`Options.Prefix = cfg.Backup.Prefix + "/wal"`). Neither tier can touch the other's keys.

## 4. Wiring in the `db` package

The engine handle is deliberately unexported (`db/backup.go` set this precedent with
`BytDBBackupTo`); the replicator therefore lives inside `db` too. New file
`db/replicate.go`:

```go
// Sketch — signatures, not final code.
var bytdbReplicator *replicate.Replicator

// StartBytDBReplication builds the s3 client from cfg and starts the
// background ship loop. No-op (nil error) when the backend is not bytdb
// or replication is not configured — callers don't need to gate.
func StartBytDBReplication() error

// BytDBReplicationStatus returns (status, true) when a replicator is
// running — feeds the ops endpoint in §6.
func BytDBReplicationStatus() (replicate.Status, bool)
```

- Storage client: `replicate/s3.New(s3.Config{Endpoint, Region, Bucket, AccessKey,
  SecretKey})` — the stdlib SigV4 client that ships with the feature. dbbackup keeps its
  aws-sdk-v2 client for now; consolidating it onto `replicate/s3` (dropping the AWS SDK
  dependency) is a worthwhile follow-up, not part of this change.
- Options: `Prefix: cfg.Backup.Prefix + "/wal"`, `Interval` from config,
  `Logf: logger`-backed adapter (the db package convention of using `fmt` is for
  import-cycle reasons; an injected func sidesteps that — pass the adapter in from the
  caller or use `fmt` like the rest of `db`, implementer's choice).
- Defaults for `MaxChunkBytes` (8 MB) and `RetainGenerations` (3) are right for MB-scale
  church DBs; not exposed in config until a need appears.

### Lifecycle ordering

Upstream contract: **close the replicator before the engine** so the final flush has a
live source. `CloseDB()` (`db/connect.go:72`) gains the replicator as its *first* step:

```
CloseDB(): replicator.Close() → dbHandle.Close() → pgwire.Close() → engine.Close()
```

Startup in each site `main.go` (ccswm, cema — one line after the existing InitDB call):

```go
err := db.InitDB(dbOpts)
...
if err := db.StartBytDBReplication(); err != nil {
    logger.LogErr(err, "Could not start DB replication")  // log, don't abort:
}                                                          // serving > shipping
```

A replication *config* error (bad endpoint URL, missing creds with `replicate: true`)
should log loudly and leave the site running — the hourly snapshot tier still protects
it. Ship *runtime* errors are already handled upstream (logged, retried next tick).

### Shutdown reality check (important, and fine)

`church.ServeRWeb()` blocks until process death, so the sites' `defer db.CloseDB()`
never runs on SIGTERM — the final flush will usually not happen on a pod kill. **This
does not widen the RPO**: on any restart where the PVC survives, the new process starts
a fresh generation and re-ships the whole (small) file from offset 0 — nothing is lost.
The final flush only matters in the double-failure case (pod killed AND volume lost in
the same ~5s window), which is exactly the ≤1-interval loss the doctrine accepts.
Adding a SIGTERM → `CloseDB()` handler in `ServeRWeb` is a nice-to-have hardening item,
not a prerequisite.

## 5. Cold-start restore (in-app; retires the initContainer)

Today an initContainer (`minio/mc`) copies `<prefix>/latest/church.db` onto an empty
volume. Move restore into `startBytDB()` (`db/connect.go:158`), *before* `bytdb.Open`:

```
if /data/church.db does not exist AND replication/backup is configured:
    1. replicate.Restore(ctx, store, prefix+"/wal", file)   // newest WAL generation
    2. on ErrNoReplica → GET <prefix>/latest/church.db      // snapshot fallback
    3. on neither present → fall through                    // fresh site: schema bootstrap
```

- **Precedence is WAL-first** because the WAL replica is at most seconds stale while
  `latest/` is up to an hour stale. The snapshot fallback preserves the migration
  runbook unchanged: the PG cutover uploads the imported file to `latest/`, and a fresh
  volume picks it up (a brand-new site has no `wal/` generations yet).
- The existence check is the only guard needed: restore never runs over an existing
  file, and `replicate.Restore` is internally atomic (temp file + fsync + rename), so a
  crash mid-restore leaves no plausible-looking partial DB.
- The `restore-if-empty` initContainer is then deleted from both site manifests — one
  less third-party image in the boot path, and the restore logic lives where it can be
  unit-tested. (Keep the block commented in the manifest for one release as a documented
  manual-recovery escape hatch.)
- Timeout: wrap the restore in a generous context (e.g. 2 min). On failure, **abort
  startup loudly** rather than bootstrapping an empty schema over a site that *should*
  have data — an empty site serving 200s is worse than a crash-looping pod, and
  `strategy: Recreate` means nothing else is serving anyway. ("Failure" = store reachable
  but restore errored; `ErrNoReplica` + no snapshot = legitimately fresh site.)

## 6. Observability

- New ops endpoint `GET /api/admin/db/replication` in `resource/dbbackup` (same package —
  it already owns the token gate and the uniform `{"error":...}` shape; reuse the gate
  order: 503 unconfigured → 401 bad bearer → 503 non-bytdb). Response is
  `replicate.Status` plus a derived `lag_seconds` (now − LastShipTime):

  ```json
  {"generation":"20260719t...","epoch":3,"watermark":52480,
   "last_ship":"2026-07-19T21:04:05Z","lag_seconds":2,"last_error":null}
  ```

- `LastError` non-nil or `lag_seconds` ≫ interval = shipping is stalled (bucket outage,
  bad creds). The site keeps serving; the snapshot CronJob keeps running. Alerting can
  curl this endpoint later — out of scope here.
- Do **not** wire replication health into the readiness probe: killing traffic because
  object storage hiccuped inverts the priority (serving > shipping).

## 7. k8s manifest changes (`deploy/k8s/sites/*.yaml`)

1. **Deployment env:** add `- {name: BACKUP_REPLICATE, value: "true"}` next to
   `BACKUP_PREFIX`. Creds already arrive via the `<site>-backup` secret. No new secret.
2. **initContainer:** remove `restore-if-empty` (§5). The `minio/mc` image drops out of
   the boot path.
3. **Backup CronJob:** keep, unchanged. Hourly full snapshots stay as the independent
   second tier (protects against a replicate-chain bug; `latest/` remains the migration
   vehicle). The CronJob comment about "WAL shipping will tighten RPO later" gets
   updated to "WAL shipping (in-app) is the primary tier; this is the snapshot tier."
4. **PVC / probes / resources:** unchanged. Replicator memory is one ≤8 MB staging
   buffer; PUT traffic is trivial for MB-scale DBs.

Rollout is per-site and reversible: setting `BACKUP_REPLICATE=false` (or removing the
env) stops shipping; already-uploaded generations remain restorable.

## 8. Failure modes

| Failure | Behavior | Data at risk |
|---|---|---|
| Object storage down / bad creds | Ship fails, logged, retried every tick; site unaffected; status endpoint shows `last_error` | none locally; replica staleness grows |
| Pod restart, PVC intact | New generation, whole file re-ships within seconds | none |
| PVC lost / volume destroyed | Next boot restores newest complete WAL generation | ≤ interval + upload time (~5–10 s) |
| PVC lost AND no WAL generation (pre-rollout site) | Falls back to `latest/` snapshot | ≤ 1 h (status quo today) |
| Compaction during ship | `ErrEpochChanged` → generation rolls next tick (upstream-handled) | none |
| Torn tail in shipped chunks | Restore truncates at last valid record — replay semantics identical to local crash recovery | seconds |
| Chunk corrupted in store | Restore fails loudly on size mismatch (never splices); operator falls back to snapshot | recovery choice |
| Node clock far in the past on restart | New generation ID may sort older than a prior one → restore could prefer stale data. Accepted upstream; LKE nodes run NTP | theoretical |

## 9. Implementation checklist

1. `config/config.go` + `config/env_overrides.go` — `replicate`, `replicate_interval`
   fields and `BACKUP_REPLICATE*` overrides.
2. `db/replicate.go` — start/status/stop; hook into `CloseDB()` (replicator first).
3. `db/connect.go` — restore-if-empty inside `startBytDB()` before `bytdb.Open`.
4. `resource/dbbackup` — `GET /api/admin/db/replication` status endpoint + contract tests
   (mirror `api_contract_test.go` gate-order cases).
5. `ccswm/main.go`, `cema/main.go` — `db.StartBytDBReplication()` after `InitDB`.
6. Manifests: env var in, initContainer out, CronJob comment updated; README runbook
   note (restore is now in-app; manual recovery = `mc cp latest/` as before).
7. Follow-ups (non-blocking): SIGTERM → CloseDB in `ServeRWeb`; migrate dbbackup off
   aws-sdk-v2 onto `replicate/s3`.

### Test plan

- **Unit (church repo):** restore-precedence matrix against the in-memory `Storage`
  fake from `bytdb/replicate` (WAL gen wins; snapshot fallback on `ErrNoReplica`; fresh
  site bootstraps; restore error aborts). Config gating (off unless bytdb + creds +
  flag). Status endpoint contract cases.
- **Integration:** boot a site with replication against MinIO (or the signature-checking
  fake), write via admin flow, kill the pod-equivalent, delete the data file, boot again
  → rows present. This doubles as readiness-item-1 exercise.
- **Cluster dry-run:** part of the existing cutover runbook — after the importer upload,
  first boot restores from `latest/`, then generations appear under `<prefix>/wal/gen/`
  within seconds (verifiable with `mc ls`).
