# Church sites on Linode LKE — bytdb edition

Runs each church site (ccswm.org, calvaryeastmetro.org, …) as a single-pod
Deployment on a shared LKE cluster. bytdb is embedded in the site process;
the database file lives on Linode Block Storage; its WAL ships continuously
to Linode Object Storage (S3-compatible), backed by hourly full snapshots to
the same bucket; one shared ingress-nginx NodeBalancer fans out by Host
header to all sites, with Let's Encrypt TLS via cert-manager.

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
                                │  ~5s  in-app WAL shipping   → <site>/wal/gen/…
                                ▼  1h   Engine.BackupTo       → <site>/<ts>/church.db
                        [Object Storage]         replicas only — never live WAL
                                │
                                └──▶ boot on an empty PVC restores from it
```

## Why this shape

- **One pod per site, `Recreate`, RWO block storage** — bytdb is an embedded
  engine; the process is the sole writer of its file. Two replicas would
  corrupt it, and RollingUpdate would briefly run two.
- **Live DB on block storage, never object storage** — the WAL requires
  honest fsync semantics; S3-compatible stores can't provide that. Object
  storage is strictly the backup/restore target.
- **Two replication tiers, one bucket** — WAL shipping (`bytdb/replicate`,
  driven from `db/replicate.go`) gives RPO ~5s; the hourly full snapshot
  stays as an independent second tier. Keeping both is deliberate: they share
  no code path, so a bug in the WAL chain cannot take out both copies, and
  `latest/` remains the migration-delivery vehicle. Their key namespaces are
  disjoint by shape, so neither tier's pruning can reach the other's objects.
- **Restore is in-app, not an initContainer** — `startBytDB()` self-heals an
  empty volume before opening the engine: newest complete WAL generation
  first, `latest/` snapshot on `ErrNoReplica`, fresh schema bootstrap if
  neither exists. A store that is reachable but errors aborts the boot on
  purpose — an empty site serving 200s is worse than a crash-looping pod, and
  `Recreate` means nothing else is serving anyway.
- **Shared namespace `churches`** — same operator for all sites; naming
  (`ccswm-*`, `cema-*`) is isolation enough.
- **Probes hit `/healthz`, and a `startupProbe` guards the restore** —
  `/healthz` (`router_rweb.go`) takes no session, no DB round trip and no page
  render, so a slow database can't be mistaken for a wedged process and
  restarted. The `startupProbe` matters more than it looks: cold-start restore
  from object storage runs *before* the app binds a port and may take up to two
  minutes, and a liveness probe with a short initial delay would kill the pod
  mid-recovery, repeatedly, precisely when recovery is the point. Readiness is
  not wired to DB or replication health either — at one replica there is
  nowhere for traffic to go, so failing readiness converts a degraded site into
  a hard 503.

### Uploaded images

Article images pasted into the editor are written by `resource/chimage` to the
CWD-relative `dist/img/` and referenced as `/assets/img/…`. The Deployment
mounts that path from a subdirectory (`uploads`) of the same block volume as
the database, so a redeploy no longer silently breaks the images in existing
articles — which is what happened while they lived in the container's writable
layer.

Note the asymmetry, because it is a real limitation and not an oversight:
**uploaded images are not shipped to object storage.** The WAL replication and
snapshot tiers cover the database only. Images are protected by the
`-retain` storage class and whatever Linode volume snapshots you schedule. The
durable fix is to put them on IDrive e2 next to the sermon media, which is a
code change in `resource/chimage`, not a manifest one.

## Bucket layout

One prefix per site, three key shapes that never overlap:

| Key | Writer | Read by |
|---|---|---|
| `<prefix>/wal/gen/<gen>/<start>-<end>.wlog` | replicator (`db/replicate.go`) | boot restore, replicator prune |
| `<prefix>/wal/gen/<gen>/manifest.json` | replicator | boot restore (completeness check) |
| `<prefix>/<UTC timestamp>/church.db` | `resource/dbbackup` | its own prune |
| `<prefix>/latest/church.db` | `resource/dbbackup` (CopyObject) | boot restore fallback, migration delivery |

The snapshot prune only deletes keys shaped exactly
`<prefix>/<timestamp>/church.db`; the replicator only lists and prunes under
`<prefix>/wal/gen/`. Sharing one prefix (and one credential set) is safe by
construction and keeps a site to a single `<site>-backup` secret.

## WAL shipping

Enabled per site with `BACKUP_REPLICATE=true` in the Deployment (or
`backup.replicate: true` in options.yml); cadence via
`BACKUP_REPLICATE_INTERVAL` / `backup.replicate_interval`, default 5s. Started
by `db.StartBytDBReplication()` from each site's `main.go`, which is a no-op
on the Postgres backend, without object-store credentials, or without the
flag. A start failure is logged and the site keeps serving — shipping is
insurance, and refusing to boot on stale credentials would turn a degraded
backup into an outage.

Idle ticks cost one local `LogState()` call and zero requests, so a quiet
site generates no traffic. A pod restart with the PVC intact rolls a new
generation and re-ships the whole (MB-scale) file from offset zero, which is
why the missing SIGTERM handler costs nothing: `church.ServeRWeb()` blocks
until process death, so `defer db.CloseDB()` — and with it the replicator's
final flush — usually does not run on a pod kill. That only matters if the
pod *and* the volume are lost inside one interval, which is exactly the
≤1-interval loss the design already accepts.

Health: `GET /api/admin/db/replication`, same bearer token as the backup
endpoint.

```bash
curl -sS -H "Authorization: Bearer $BACKUP_TOKEN" \
  http://ccswm.churches.svc.cluster.local:4000/api/admin/db/replication
# {"generation":"20260801t…","epoch":3,"watermark":52480,
#  "last_ship":"2026-08-01T21:04:05Z","lag_seconds":2,
#  "interval_seconds":5,"last_error":null}
```

`last_error` non-null, or `lag_seconds` ≫ `interval_seconds`, means shipping
is stalled (bucket outage, rotated credentials) while the site itself is
fine. 503 means replication is off or the backend is not bytdb. Deliberately
**not** wired into the readiness probe: killing traffic because object
storage hiccuped inverts the priority — serving outranks shipping.

## The backup endpoint

`POST /api/admin/db/backup` (implemented in `resource/dbbackup`; route in
`router_rweb.go`). The CronJob calls it because only the app's engine handle
can produce a consistent snapshot (`Engine.BackupTo`) — an external job must
not copy the live file. On each trigger it: authenticates the
`Authorization: Bearer` token against `BACKUP_TOKEN` (constant-time),
snapshots the engine, uploads to `<bucket>/<prefix>/<UTC timestamp>/church.db`,
server-side copies over `<bucket>/<prefix>/latest/church.db` (the boot
restore's fallback source), and prunes timestamped snapshots beyond
`BACKUP_RETAIN` (default 72 ≈ 3 days hourly). Responses: 503 unconfigured or
non-bytdb backend, 401 bad token, 200 with
`{key, latest_key, bytes, pruned, dur_millis}`.

Configuration arrives as env from the `<site>-backup` secret (`OBJ_ENDPOINT`,
`OBJ_REGION`, `OBJ_BUCKET`, `OBJ_ACCESS_KEY`, `OBJ_SECRET_KEY`, `BACKUP_TOKEN`)
plus `BACKUP_PREFIX` and `BACKUP_REPLICATE` set in the Deployment — or
equivalently a `backup:` block in options.yml (see `config/config.go`). Gate
ordering and JSON shapes for both endpoints are frozen by
`resource/dbbackup/api_contract_test.go`.

`OBJ_REGION` is required by `deploy.sh` rather than left to its default. Both
S3 clients fall back to `us-east-1` when it is blank, and against a bucket in
any other Linode region that fails SigV4 signing in the quietest way available:
the site keeps serving, the error only reaches the log, and both the snapshot
and WAL tiers simply stop producing copies.

On-demand run: `kubectl -n churches create job --from=cronjob/ccswm-backup
ccswm-backup-manual`, or curl the endpoint directly with the token.

## Install

`deploy/deploy.sh` runs the whole thing. Every phase is idempotent — re-running
is the normal way to apply a change, not a recovery action.

```bash
cp deploy/backup.env.sample deploy/backup.env   # fill in Linode Object Storage creds
./deploy/deploy.sh all
```

Phases, in the order `all` runs them (each also runnable on its own):

| Phase | Does |
|---|---|
| `preflight` | Tooling, cluster context confirmation, manifests, build context |
| `infra` | helm-installs ingress-nginx + cert-manager; prints the NodeBalancer IP |
| `base` | Namespace + Let's Encrypt ClusterIssuer (needs cert-manager's CRDs) |
| `seeds` | Generates `cfg/random_seeds.txt` where missing; never overwrites |
| `secrets` | `<site>-config` and `<site>-backup`, validated before they are applied |
| `images` | `docker build` + push, one image per site, tagged with the site's git SHA |
| `sites` | DNS precheck → apply manifests with the pinned image → wait for rollout |
| `verify` | Read-only: pod readiness, cert readiness, `/healthz`, replication lag |

**The one manual step in the middle is DNS.** The NodeBalancer only exists after
`infra`, and an Ingress applied before its domains resolve produces an HTTP-01
challenge that cannot succeed — Let's Encrypt rate-limits failed authorizations
(5/hour/hostname), so impatience here costs an hour. So either run the phases in
two passes:

```bash
./deploy/deploy.sh preflight infra          # prints the NodeBalancer IP
#   → point A records for every domain at that IP, let them propagate
./deploy/deploy.sh base seeds secrets images sites verify
```

…or run `all` and answer `n` when the DNS precheck warns, then re-run `sites`
once the records resolve. The precheck compares each `- host:` in the site
manifest against `dig +short A`, so the manifests remain the single source of
truth for which domains a site owns.

Common follow-ups:

```bash
./deploy/deploy.sh images sites             # redeploy after a code change
SITES=ccswm ./deploy/deploy.sh images sites # one site only
./deploy/deploy.sh verify                   # health check, changes nothing
./deploy/deploy.sh --yes all                # non-interactive (CI)
```

Overridable via environment: `SITES`, `NAMESPACE`, `REGISTRY`,
`BACKUP_ENV_FILE`, `INGRESS_NGINX_VERSION`, `CERT_MANAGER_VERSION`,
`ASSUME_YES`. The chart versions default to "whatever the repo serves today" —
pin them once you know what you want to live with.

### What each site actually needs to boot

Three things beyond the image, and the first two are unforgiving:

1. **`cfg/random_seeds.txt`** — `resource/auth`'s `init()` opens it and
   `log.Fatal()`s when it can't. That runs before `main()`, so a site missing
   this file is an unconditional crash loop, not a degraded mode. It ships in
   the `<site>-config` Secret alongside `options.yml`, which is why that Secret
   is mounted as a whole directory over `/app/cfg` rather than as a single-file
   `subPath`. `deploy.sh seeds` generates one where absent; doing so is safe on
   a live site, because the pool is entropy for *new* salts and tokens and each
   user's salt is stored next to their hash.
2. **`APP_ENV=production`** — `config.InitConfig` defaults to `development`
   when it is unset, and would then read the wrong section of `options.yml`
   entirely. Set in the Deployment.
3. **`options.yml` with a `production:` section** — `deploy.sh secrets` refuses
   to build the Secret without one, since `getOptionsForEnvironment` `log.Fatal`s
   on a missing section.

`server.port` and `use_tls` in that file are deliberately *not* checked: the
Deployment overrides both with `SERVER_PORT=4000` and `USE_TLS=false`
(`config/env_overrides.go`), so each site's own production section can keep its
bare-metal values — ccswm's sample says port 80 with certbot paths, cema's says
8088 — while the pod, its Service, and its probes all agree on 4000. TLS
terminates at the ingress, which also owns the ACME challenge.

### Image build

CGO is required and the image is **not** static: `resource/chimage` imports
`h2non/bimg`, a cgo binding to libvips, so the builder installs `vips-dev` and
the runtime stage installs `vips`. With `CGO_ENABLED=0` the package does not
compile at all.

BuildKit is required too. The context is the workspace parent directory
(~2.9 GB unfiltered, including `church_mobile/` and each site's live
`options.yml`), and the exclusions live in `deploy/docker/Dockerfile.dockerignore`
— a per-Dockerfile ignore file that only BuildKit honors. `deploy.sh` exports
`DOCKER_BUILDKIT=1`; a hand-run `docker build` must do the same.

## Migration runbook: Postgres → bytdb per site

The boot restore path doubles as the migration delivery mechanism: migrate
locally, upload the file as the "latest backup", and the first deploy restores
it. No surgery inside the cluster. A brand-new site has no WAL generations, so
the snapshot fallback is what fires — which is why the two-tier precedence
leaves this runbook unchanged.

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
5. **Deploy** the site: `SITES=<site> ./deploy/deploy.sh secrets images sites`.
   The app finds an empty volume, restores the migrated file from `latest/`
   (no WAL generation exists yet), and boots on it. Answer `n` to the DNS
   precheck if the domain still points at the old stack, then verify via
   `kubectl port-forward` before touching DNS. Within a few seconds of
   serving, generations should appear: `mc ls obj/<bucket>/<site>/wal/gen/` —
   and `./deploy/deploy.sh verify` should report a small `lag_seconds`.
6. **Cut DNS** to the NodeBalancer IP, then re-run
   `SITES=<site> ./deploy/deploy.sh sites verify` so the Ingress applies with
   DNS resolving and cert-manager issues on the first attempt. Old PG stack
   stays warm as rollback (`db.type: postgres` in options.yml is the escape
   hatch) until confident; then decommission.

## Operations quick reference

```bash
./deploy/deploy.sh verify        # pods, certs, /healthz, replication lag, all sites
kubectl -n churches get pods,pvc,ingress,cronjobs
kubectl -n churches logs deploy/ccswm -f
kubectl -n churches create job --from=cronjob/ccswm-backup ccswm-backup-manual  # on-demand backup

# The backup/replication token lives only in the cluster (deploy.sh mints it
# once and reuses it thereafter):
BACKUP_TOKEN=$(kubectl -n churches get secret ccswm-backup \
  -o jsonpath='{.data.BACKUP_TOKEN}' | base64 -d)
kubectl -n churches exec deploy/ccswm -- \
  wget -qO- --header="Authorization: Bearer $BACKUP_TOKEN" \
  http://localhost:4000/api/admin/db/replication

# First-run superadmin: with no BOOTSTRAP_ADMIN_USER/PASS set, the app writes a
# one-time bootstrap token to /app/token.txt in the pod. It lives on the
# container's writable layer, so read it before the pod restarts:
kubectl -n churches exec deploy/ccswm -- cat /app/token.txt

# psql into a live site's embedded DB: pin db.listen (e.g. 127.0.0.1:5433)
# in its options.yml, then:
kubectl -n churches port-forward deploy/ccswm 5433:5433
```

### Recovering a lost volume

Nothing to do: delete the PVC's pod, let a fresh volume attach, and the app
restores the newest complete WAL generation on boot (seconds of loss). The
log line names the source, e.g.
`bytdb: restored from WAL generation 20260801t… (5242880 bytes, 1 chunks)`.

The one case that needs hands is a store that *lost* objects: if every
manifested generation is missing chunks, the boot aborts with
`ErrIncompleteReplica` rather than silently rolling back to the hour-old
snapshot — that trade is an operator's call, not the app's. Make it by
putting a file on the volume, after which the in-app restore sees it and
stands down:

```bash
kubectl -n churches debug -it deploy/ccswm --image=minio/mc:latest \
  --target=ccswm -- sh -c '
    mc alias set obj "https://$OBJ_ENDPOINT" "$OBJ_ACCESS_KEY" "$OBJ_SECRET_KEY" &&
    mc cp "obj/$OBJ_BUCKET/ccswm/latest/church.db" /data/church.db'
```

(The commented-out `restore-if-empty` initContainer in the site manifest is
the same command, kept for one release as documentation of this path.)

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

1. Copy `sites/ccswm.yaml` → `sites/<site>.yaml`; replace `ccswm`→`<site>` and
   the two domains, and offset the CronJob schedule minute so backups don't
   stampede (ccswm is `:07`, cema `:37`).
2. Add the module to `go.work` and to the Dockerfile's `COPY` list.
3. Point DNS at the same NodeBalancer IP.
4. `SITES=<site> ./deploy/deploy.sh seeds secrets images sites verify`.

Everything else is derived: `deploy.sh` reads the domains straight out of the
new manifest, generates the seed file if the site doesn't have one, and mints
the site's backup token on first deploy.
