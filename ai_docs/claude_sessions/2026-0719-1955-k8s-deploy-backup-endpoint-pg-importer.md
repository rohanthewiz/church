# Session: LKE Deploy Manifests, Backup Endpoint, PG→bytdb Importer

- **Date:** 2026-07-19 19:55
- **Session ID:** e241e4a9-b64f-4b4e-a681-913cbc3d354c
- **Previous session:** `2026-0719-1920-bytdb-v062-workaround-reverts.md` (same session id — continuation after wrap)
- **Companion docs:** `deploy/k8s/README.md` (new), `ai_docs/fable_bytdb_k8s_readiness.md` (updated)

## What happened

Three deliverables landed, closing readiness-doc next-steps 2 and 3:

1. **k8s deployment story** for multi-domain church sites on Linode LKE.
2. **In-app backup endpoint** `POST /api/admin/db/backup` (`resource/dbbackup`).
3. **PG→bytdb importer** `test_scripts/pg_to_bytdb`, validated against the live
   local church_development database.

## 1. Deploy manifests (`deploy/`)

- Topology: shared LKE cluster, namespace `churches`, one single-replica
  Deployment per site (`strategy: Recreate` — bytdb is embedded, single writer),
  DB file on a `linode-block-storage-retain` RWO PVC. Live WAL never on object
  storage (no honest fsync); object storage is backups only.
- Multi-domain: one ingress-nginx NodeBalancer shared by all sites, Host-header
  routing (ccswm.org → svc ccswm, calvaryeastmetro.org → svc cema), cert-manager
  HTTP-01 ClusterIssuer for Let's Encrypt. TLS terminates at ingress; sites run
  plain HTTP :4000 (`use_tls: false`).
- Files: `deploy/k8s/README.md` (architecture, install order, migration runbook,
  ops reference, costs ~$65/mo for two sites), `00-namespace.yaml`,
  `01-clusterissuer.yaml`, `sites/ccswm.yaml`, `sites/cema.yaml` (each: PVC,
  Deployment with restore-if-empty initContainer via minio/mc, Service, Ingress
  apex+www, hourly backup CronJob staggered :07/:37),
  `deploy/docker/Dockerfile` (multi-stage, `--build-arg SITE=`, workspace
  parent-dir context, static binary, non-root).
- Key pattern: the initContainer pulls `<bucket>/<prefix>/latest/church.db`
  when the volume is empty — one mechanism for migration delivery, disaster
  recovery, and fresh-site boot.

## 2. Backup endpoint

- `resource/dbbackup/dbbackup.go` — Run(): engine snapshot (in-memory buffer;
  DBs are MBs) → PUT `<prefix>/<UTC ts>/church.db` → server-side CopyObject over
  `<prefix>/latest/church.db` (atomic) → prune past retention (default 72).
  Own S3 client, separate creds from IDrive media deliberately. Prune failures
  log but don't fail the run.
- `resource/dbbackup/api_rweb.go` — gate order: 503 unconfigured (loud
  misconfig), 401 bad/missing bearer (constant-time compare), 503 non-bytdb
  backend (PG installs use pg_dump). 200 → `{key, latest_key, bytes, pruned,
  dur_millis}`.
- `db/backup.go` — `BytDBBackupTo(w)`: the only sanctioned live-copy path;
  engine handle stays unexported.
- `config/config.go` + `env_overrides.go` — `backup:` block; env overrides
  match the k8s secret keys: `OBJ_ENDPOINT/REGION/BUCKET/ACCESS_KEY/SECRET_KEY`,
  `BACKUP_PREFIX/TOKEN/RETAIN`. One `<site>-backup` secret feeds app pod,
  CronJob, and initContainer.
- Route in `router_rweb.go`, outside `/api/v1` (ops endpoint, not mobile
  contract).
- Tests: `resource/dbbackup/api_contract_test.go` (7 cases: gate order + uniform
  `{"error":...}` shape) and `db/backup_snapshot_test.go` (round-trip: write →
  BytDBBackupTo → open snapshot as fresh engine → read rows back — pins
  restorability in CI).

## 3. PG→bytdb importer

- bytdb's upstream answer on the setval concern: **identity counters self-heal**
  — explicit-id inserts bump the durable counter past that id in the same
  transaction, so no sequence fix-up is needed. Note: `setval` on identity
  readback names (`users_id_seq`) ERRORS over the wire (XX000, not a no-op);
  standalone-sequence setval works. Importer therefore emits none.
- `test_scripts/pg_to_bytdb/main.go` — destination via production path
  (db.InitDB → bootstrap → loopback wire) so the file is exactly what a site
  self-creates. Copies in FK order from new `db.BytDBTableNames()` (importer
  can't drift from bootstrap schema). Ids preserved; `[]byte` scans passed
  through as strings (arrays `{a,b}`, jsonb ride in text form). Strict: drift /
  wire rejection / count mismatch aborts and deletes the half-written file;
  only source-absent tables (older installs) are skipped. Per-table count
  verification both sides.
- Validated against live local church_development (13 tables incl. sermons
  text[] and pages jsonb; all counts matched).
- `test_scripts/selfheal_probe/main.go` — on-file proof on the migrated DB:
  max(id)=4, DEFAULT insert returned 5. Probe row self-deletes. Found:
  `bsql.DB.Exec` rejects BEGIN — transaction control needs a Session.

## Environment/API notes

- `bsql.DB.Exec` returns `*Result{Cols, Rows [][]any}` — usable for SELECTs.
- `Engine.BackupTo(w io.Writer) (int64, error)` — streams consistent snapshot.
- kubectl on this machine has no reachable cluster; manifests validated by YAML
  parse (ruby) only.
- Local dev PG reachable on :5432 (no psql/pg_isready CLI installed).
- Pre-existing, untouched: `db/connect2.go` fails gofmt; gopls go.work 1.26.1
  vs local go 1.25.4 complaints.

## Next steps

1. Boot a site end-to-end on bytdb; exercise SQLBoiler admin flows (article
   first) — last unchecked readiness item before a real cutover dry-run.
2. Real cutover dry-run per `deploy/k8s/README.md` runbook (importer → upload
   to `latest/` → deploy → port-forward verify → DNS).
3. Provision LKE + Object Storage; install ingress-nginx + cert-manager; create
   per-site secrets.
4. Design `bytdb/replicate` (WAL shipping; `ReadLogRange`/`LogState` exist).
5. Optional upstream (non-blocking): bridge setval/nextval on `table_col_seq`
   names to identity counters; map missing-relation error to 42P01.
