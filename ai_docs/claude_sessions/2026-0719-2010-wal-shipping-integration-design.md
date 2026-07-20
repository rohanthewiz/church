# Session: WAL-Shipping Replication Integration Design

- **Date:** 2026-07-19 20:10
- **Session ID:** `0dfd7dfd-8e08-4d6d-8afa-38e7afa30d1d`
- **Previous session:** `2026-0719-1955-k8s-deploy-backup-endpoint-pg-importer.md`
- **Deliverable:** `ai_docs/wal_shipping_integration_design.md` (new);
  `ai_docs/fable_bytdb_k8s_readiness.md` next-step 4 updated.

## The task, and the discovery that reframed it

The ask was readiness-doc next-step 4: "Design `bytdb/replicate` (WAL shipping)". First
finding: **that item was stale** — the replicate package was designed AND built earlier
the same day in the bytdb repo (its session `2026-0719-1729-s3-replication.md`), shipped
in bytdb v0.6.0, and is already present in the v0.6.2 this repo pins. The church session
doc written at 19:55 listed the design as pending without knowing the upstream work
existed (bytdb commit `a815462`, 17:20).

So the design work that genuinely remained was the **church-side integration**, and that
is what got designed (no implementation this session — design doc only).

## What exists upstream (for orientation)

- `replicate` package: `Source` interface (`LogState`/`ReadLogRange` — satisfied by
  `*bytdb.Engine`), `Replicator` (`New/Run/Start/Close/ShipNow/Status`), generations
  (fresh per process start / compaction epoch bump), chunk keys
  `<prefix>gen/<id>/<start>-<end>.wlog` (hex offsets; contiguity verifiable from
  listing), watermark advances only after successful PUT, interval loop = retry policy.
- `restore.go`: `Restore(ctx, store, prefix, destPath)` — newest complete generation,
  chain validated from listing, temp file + fsync + rename, `ErrNoReplica` sentinel.
- `replicate/s3`: stdlib-only SigV4 client (Put/Get/List/Delete), path-style default.
- Doctrine: **replica = recovery, not HA**; RPO ≈ ship interval (5s default).

## Design decisions (see the design doc for full reasoning)

1. **Config rides the existing `backup:` block** — `replicate: true` +
   `replicate_interval`, env `BACKUP_REPLICATE`/`BACKUP_REPLICATE_INTERVAL`. Same bucket,
   same creds, same k8s secret: WAL chunks and snapshots have identical blast radius.
   Explicit opt-in so a version bump never silently starts writing to the bucket.
2. **Key layout — shared prefix, disjoint namespaces:** WAL under `<prefix>/wal/gen/...`.
   Verified `dbbackup.prune` cannot touch them (it only deletes keys shaped exactly
   `<prefix>/<timestamp>/church.db`), and the replicator lists/prunes only under
   `<prefix>/wal/gen/`.
3. **Replicator lives in `db` package** (`db/replicate.go` — `StartBytDBReplication()`,
   `BytDBReplicationStatus()`), following the `db/backup.go` unexported-engine
   precedent. `CloseDB()` closes replicator FIRST (final flush needs a live source),
   then dbHandle → pgwire → engine. Start-replication config errors log loudly but don't
   abort startup: serving > shipping (snapshot tier still protects).
4. **Cold-start restore moves in-app**, into `startBytDB()` before `bytdb.Open`, only
   when the data file is absent: WAL generation first (seconds stale) →
   `latest/church.db` snapshot on `ErrNoReplica` (preserves the PG-migration runbook) →
   fresh schema bootstrap. Restore *errors* abort startup loudly — an empty site serving
   200s is worse than a crash-looping pod. Retires the `minio/mc` initContainer.
5. **Observability:** `GET /api/admin/db/replication` in `resource/dbbackup` (reuses its
   gate order: 503 unconfigured → 401 bearer → 503 non-bytdb), returns
   `replicate.Status` + derived `lag_seconds`. Deliberately NOT wired into readiness
   probes.
6. **Manifests:** add `BACKUP_REPLICATE=true` env; delete initContainer; hourly snapshot
   CronJob stays unchanged as the independent second tier. Per-site, reversible rollout.

## Load-bearing findings from code reading

- **SIGTERM gap:** `church.ServeRWeb()` blocks until process death, so the sites'
  `defer db.CloseDB()` never runs on pod kill → the final flush usually won't happen.
  This does NOT widen RPO: a restart with the PVC intact starts a new generation and
  re-ships the whole MB-scale file from offset 0. Final flush only matters in the
  double-failure (pod + volume lost within one interval) the doctrine already accepts.
  SIGTERM → CloseDB handler in ServeRWeb listed as non-blocking hardening.
- `dbbackup` carries its own aws-sdk-go-v2 S3 client; consolidating onto `replicate/s3`
  (dropping the AWS SDK dep) noted as a follow-up, not part of this change.
- Clock-skew generation mis-ordering on restore: accepted upstream; LKE nodes run NTP.

## Failure-mode summary

| Failure | Data at risk |
|---|---|
| Bucket outage / bad creds | none locally; replica staleness grows, site unaffected |
| Pod restart, PVC intact | none (new generation re-ships file) |
| PVC lost | ≤ interval + upload (~5–10 s) |
| PVC lost, pre-rollout site (no WAL gens) | ≤ 1 h via `latest/` snapshot (status quo) |
| Chunk corrupted in store | restore fails loudly (size-vs-name check); fall back to snapshot |

## Next steps

1. Implement per the design doc's checklist: config fields + env overrides;
   `db/replicate.go`; restore-if-empty in `startBytDB()`; status endpoint + contract
   tests; one-liner in `ccswm/main.go` + `cema/main.go`; manifest updates.
2. Test plan: restore-precedence matrix against the upstream in-memory Storage fake;
   MinIO boot→write→delete-file→reboot integration (doubles as readiness item 1
   exercise); cluster dry-run additions to the cutover runbook.
3. Still open from before: boot a site end-to-end on bytdb + SQLBoiler admin flows;
   provision LKE; real cutover dry-run.
