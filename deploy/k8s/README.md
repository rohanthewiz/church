# Church sites on Linode LKE — bytdb edition

Runs each church site (ccswm.org, calvaryeastmetro.org, …) as a single-pod
Deployment on a shared LKE cluster. bytdb is embedded in the site process;
the database file lives on Linode Block Storage; hourly consistent backups
go to Linode Object Storage (S3-compatible); one shared ingress-nginx
NodeBalancer fans out by Host header to all sites, with Let's Encrypt TLS
via cert-manager.

```
                 DNS A records ─┐ (all domains → one IP)
                                ▼
                    ┌──────────────────────┐
                    │ NodeBalancer ($10/mo)│
                    └──────────┬───────────┘
                               ▼
                    ┌──────────────────────┐
                    │ ingress-nginx        │  Host: ccswm.org → svc ccswm
                    │ (TLS: cert-manager)  │  Host: calvaryeastmetro.org → svc cema
                    └─────┬──────────┬─────┘
                          ▼          ▼
                   ┌───────────┐ ┌───────────┐
                   │ ccswm pod │ │ cema pod  │   1 replica each, Recreate
                   │  bytdb ←──┼─┼─→ bytdb   │   (embedded, single writer)
                   └─────┬─────┘ └─────┬─────┘
                         ▼             ▼
                   [Block PVC]    [Block PVC]    live DB file (honest fsync)
                         └──────┬──────┘
                                ▼  hourly Engine.BackupTo (CronJob-triggered)
                        [Object Storage]         backups only — never live WAL
```

## Why this shape

- **One pod per site, `Recreate`, RWO block storage** — bytdb is an embedded
  engine; the process is the sole writer of its file. Two replicas would
  corrupt it, and RollingUpdate would briefly run two.
- **Live DB on block storage, never object storage** — the WAL requires
  honest fsync semantics; S3-compatible stores can't provide that. Object
  storage is strictly the backup/restore target.
- **Hourly full backup = interim replication** — the databases are MBs, so
  full snapshots are cheap; RPO is the backup interval (1h). The future
  `bytdb/replicate` (WAL shipping via `Engine.ReadLogRange`/`LogState`)
  tightens RPO to seconds without changing the manifests' shape.
- **Shared namespace `churches`** — same operator for all sites; naming
  (`ccswm-*`, `cema-*`) is isolation enough.

## The backup endpoint

`POST /api/admin/db/backup` (implemented in `resource/dbbackup`; route in
`router_rweb.go`). The CronJob calls it because only the app's engine handle
can produce a consistent snapshot (`Engine.BackupTo`) — an external job must
not copy the live file. On each trigger it: authenticates the
`Authorization: Bearer` token against `BACKUP_TOKEN` (constant-time),
snapshots the engine, uploads to `<bucket>/<prefix>/<UTC timestamp>/church.db`,
server-side copies over `<bucket>/<prefix>/latest/church.db` (the
initContainer's restore source), and prunes timestamped snapshots beyond
`BACKUP_RETAIN` (default 72 ≈ 3 days hourly). Responses: 503 unconfigured or
non-bytdb backend, 401 bad token, 200 with
`{key, latest_key, bytes, pruned, dur_millis}`.

Configuration arrives as env from the `<site>-backup` secret (`OBJ_ENDPOINT`,
`OBJ_BUCKET`, `OBJ_ACCESS_KEY`, `OBJ_SECRET_KEY`, `BACKUP_TOKEN`) plus
`BACKUP_PREFIX` set in the Deployment — or equivalently a `backup:` block in
options.yml (see `config/config.go`). Gate ordering and JSON shapes are
frozen by `resource/dbbackup/api_contract_test.go`.

On-demand run: `kubectl -n churches create job --from=cronjob/ccswm-backup
ccswm-backup-manual`, or curl the endpoint directly with the token.

## Install order

```bash
# 1. Cluster + kubectl context (Linode Cloud Manager or terraform). Two
#    shared-CPU 4GB nodes is plenty for several sites.

# 2. Ingress controller — creates the NodeBalancer; note its EXTERNAL-IP
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx --create-namespace
kubectl get svc -n ingress-nginx ingress-nginx-controller  # EXTERNAL-IP

# 3. DNS: A records for ccswm.org, www.ccswm.org, calvaryeastmetro.org,
#    www.calvaryeastmetro.org → that EXTERNAL-IP. Do this BEFORE applying
#    site manifests so HTTP-01 challenges succeed on first try.

# 4. cert-manager
helm repo add jetstack https://charts.jetstack.io
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager --create-namespace --set crds.enabled=true

# 5. Base objects
kubectl apply -f deploy/k8s/00-namespace.yaml
kubectl apply -f deploy/k8s/01-clusterissuer.yaml

# 6. Per-site secrets (never committed). Repeat per site:
kubectl -n churches create secret generic ccswm-config \
  --from-file=options.yml=../ccswm/cfg/options.yml
kubectl -n churches create secret generic ccswm-backup \
  --from-literal=OBJ_ENDPOINT=us-east-1.linodeobjects.com \
  --from-literal=OBJ_BUCKET=church-backups \
  --from-literal=OBJ_ACCESS_KEY=... \
  --from-literal=OBJ_SECRET_KEY=... \
  --from-literal=BACKUP_TOKEN="$(openssl rand -hex 32)"
#    In options.yml for k8s: server.port 4000, use_tls false (TLS terminates
#    at ingress), db.type bytdb (default). Stripe keys can ride in
#    options.yml or as STRIPE_* env overrides.

# 7. Images (context = parent dir; see deploy/docker/Dockerfile header)
cd ~/projs/go/church
docker build -f church/deploy/docker/Dockerfile --build-arg SITE=ccswm \
  -t ghcr.io/rohanthewiz/ccswm:<tag> . && docker push ghcr.io/rohanthewiz/ccswm:<tag>

# 8. Sites
kubectl apply -f deploy/k8s/sites/ccswm.yaml
kubectl apply -f deploy/k8s/sites/cema.yaml
```

## Migration runbook: Postgres → bytdb per site

The initContainer restore path doubles as the migration delivery mechanism:
migrate locally, upload the file as the "latest backup", and first deploy
restores it. No surgery inside the cluster.

1. **Run the importer** (`test_scripts/pg_to_bytdb`): brings the destination
   up through the production path (schema bootstrap + wire loopback), then
   copies rows table-by-table in FK dependency order (`db.BytDBTableNames`),
   preserving ids, and verifies per-table counts. Strict by design — any
   schema drift or wire rejection aborts; only a table absent in an older
   source install is tolerated (skipped, left empty).
   No sequence fix-up is needed: bytdb identity counters self-heal — an
   explicit-id insert bumps the counter past that id (verified upstream on
   v0.6.2 and on-file by `test_scripts/selfheal_probe`), so Postgres-style
   `setval` is unnecessary.
2. **Dry run locally**: import from a restored production dump, then
   `go run ./test_scripts/bytdb_wire_check` against the produced file and
   boot the site on it (`DB_FILE=…`). Click through admin: article CRUD,
   page builder, menus, sermons, events — this doubles as the SQLBoiler
   smoke test from the readiness doc's next steps.
3. **Content freeze** on the live PG site (church sites are low-write;
   a short freeze beats building delta sync).
4. **Final import** against live PG; upload the result to
   `s3://<bucket>/<site>/latest/church.db`.
5. **Deploy** the site manifest. The initContainer finds an empty volume,
   pulls the migrated file, and the app boots on it. Verify via
   `kubectl port-forward` before touching DNS.
6. **Cut DNS** to the NodeBalancer IP. Old PG stack stays warm as rollback
   (`db.type: postgres` in options.yml is the escape hatch) until confident;
   then decommission.

## Operations quick reference

```bash
kubectl -n churches get pods,pvc,ingress,cronjobs
kubectl -n churches logs deploy/ccswm -f
kubectl -n churches create job --from=cronjob/ccswm-backup ccswm-backup-manual  # on-demand backup
# psql into a live site's embedded DB: pin db.listen (e.g. 127.0.0.1:5433)
# in its options.yml, then:
kubectl -n churches port-forward deploy/ccswm 5433:5433
```

## Cost (monthly, approx)

| Item | Cost |
|---|---|
| 2× LKE shared 4GB nodes | $48 |
| NodeBalancer (shared by all sites) | $10 |
| Block storage 10GiB × 2 sites | $2 |
| Object Storage (250GB flat) | $5 |
| **Total for two sites** | **~$65** |

Each additional site adds ~$1/mo (its PVC) until node capacity forces a
third node — the marginal-cost story that motivated bytdb in the first
place.

## Adding a site

Copy `sites/ccswm.yaml`, replace `ccswm`→`<site>` and the domains, offset
the backup schedule minute, create the two secrets, build/push the image
(`--build-arg SITE=<site>` — add its module to the Dockerfile COPY list and
go.work if new), point DNS at the same NodeBalancer IP, `kubectl apply`.
