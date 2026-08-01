package dbbackup

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/apiv1"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

// gateOps runs the shared admission checks for this package's ops endpoints.
// When ok is false the response has already been written and the caller must
// return the accompanying error value verbatim.
//
// Auth is a static bearer token (config Backup.Token), not the session/role
// system: the caller is a k8s CronJob or a monitor, not a person — it has no
// session, and wiring machine credentials into the user table would give these
// endpoints a login they don't need. The token grants exactly two
// capabilities: "cause a backup now" and "read replication progress". Neither
// can read data back, so even a leaked token only costs some object-storage
// churn and a peek at a byte counter.
//
// Ordering of the gates matters for what each response reveals:
//  1. 503 when unconfigured — deploy-time state, safe to expose, and it makes
//     a misconfigured CronJob fail loudly rather than 401-ing forever.
//  2. 401 on bad/missing token — constant-time compare; the response does not
//     distinguish missing vs wrong.
//  3. 503 when the backend isn't bytdb — Postgres installs back up via
//     pg_dump and have no WAL shipping; these endpoints answering 500 would
//     page someone for a non-error.
func gateOps(ctx rweb.Context) (written error, ok bool) {
	if !Configured() { // also covers config.Options == nil
		return apiv1.Error(ctx, http.StatusServiceUnavailable, "Backup is not configured"), false
	}
	cfgToken := strings.TrimSpace(config.Options.Backup.Token)
	if cfgToken == "" {
		return apiv1.Error(ctx, http.StatusServiceUnavailable, "Backup is not configured"), false
	}

	const scheme = "Bearer "
	authz := ctx.Request().Header("Authorization")
	if len(authz) <= len(scheme) || !strings.EqualFold(authz[:len(scheme)], scheme) {
		return apiv1.Error(ctx, http.StatusUnauthorized, "Authentication required"), false
	}
	presented := strings.TrimSpace(authz[len(scheme):])
	if subtle.ConstantTimeCompare([]byte(presented), []byte(cfgToken)) != 1 {
		return apiv1.Error(ctx, http.StatusUnauthorized, "Invalid token"), false
	}

	if db.BytDBWireAddr() == "" {
		return apiv1.Error(ctx, http.StatusServiceUnavailable,
			"This endpoint requires the bytdb backend (Postgres installs use pg_dump)"), false
	}
	return nil, true
}

// APIBackupRWeb triggers one backup run. POST /api/admin/db/backup.
// Gates per gateOps; normally invoked by the per-site backup CronJob.
func APIBackupRWeb(ctx rweb.Context) error {
	if written, ok := gateOps(ctx); !ok {
		return written
	}

	res, err := Run()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Backup failed")
	}
	logger.Info("Database backup completed",
		"key", res.Key, "bytes", res.Bytes, "pruned", res.Pruned)
	return ctx.WriteJSON(res)
}

// APIReplicationStatusRWeb reports WAL-shipping progress.
// GET /api/admin/db/replication.
//
// Read-only, so it shares the backup token rather than minting a second one.
// A 200 means shipping is running; the payload's lag_seconds and last_error
// say whether it is healthy. Shipping being switched off is a 503 for the
// same reason an unconfigured destination is: it is deploy-time state, and a
// monitor pointed at a site that silently stopped replicating should see a
// hard failure, not a cheerful 200 with a flag buried in the body.
//
// Deliberately NOT wired into the readiness probe. Killing traffic to a
// healthy site because object storage hiccuped inverts the priority —
// serving outranks shipping, and the snapshot tier still covers the site.
func APIReplicationStatusRWeb(ctx rweb.Context) error {
	if written, ok := gateOps(ctx); !ok {
		return written
	}

	status, running := db.BytDBReplicationStatus()
	if !running {
		return apiv1.Error(ctx, http.StatusServiceUnavailable,
			"Replication is not enabled (set backup.replicate / BACKUP_REPLICATE)")
	}
	return ctx.WriteJSON(status)
}
