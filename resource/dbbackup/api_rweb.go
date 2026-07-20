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

// APIBackupRWeb triggers one backup run. POST /api/admin/db/backup.
//
// Auth is a static bearer token (config Backup.Token), not the session/role
// system: the caller is a k8s CronJob, not a person — it has no session, and
// wiring machine credentials into the user table would give the backup
// trigger a login it doesn't need. The token grants exactly one capability:
// "cause a backup now". It cannot read data back, so even a leaked token
// only costs some object-storage churn.
//
// Ordering of the gates matters for what each response reveals:
//  1. 503 when unconfigured — deploy-time state, safe to expose, and it makes
//     a misconfigured CronJob fail loudly rather than 401-ing forever.
//  2. 401 on bad/missing token — constant-time compare; the response does not
//     distinguish missing vs wrong.
//  3. 503 when the backend isn't bytdb — Postgres installs back up via
//     pg_dump; this endpoint answering 500 would page someone for a
//     non-error.
func APIBackupRWeb(ctx rweb.Context) error {
	if !Configured() { // also covers config.Options == nil
		return apiv1.Error(ctx, http.StatusServiceUnavailable, "Backup is not configured")
	}
	cfgToken := strings.TrimSpace(config.Options.Backup.Token)
	if cfgToken == "" {
		return apiv1.Error(ctx, http.StatusServiceUnavailable, "Backup is not configured")
	}

	const scheme = "Bearer "
	authz := ctx.Request().Header("Authorization")
	if len(authz) <= len(scheme) || !strings.EqualFold(authz[:len(scheme)], scheme) {
		return apiv1.Error(ctx, http.StatusUnauthorized, "Authentication required")
	}
	presented := strings.TrimSpace(authz[len(scheme):])
	if subtle.ConstantTimeCompare([]byte(presented), []byte(cfgToken)) != 1 {
		return apiv1.Error(ctx, http.StatusUnauthorized, "Invalid token")
	}

	if db.BytDBWireAddr() == "" {
		return apiv1.Error(ctx, http.StatusServiceUnavailable,
			"Backup requires the bytdb backend (Postgres installs use pg_dump)")
	}

	res, err := Run()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Backup failed")
	}
	logger.Info("Database backup completed",
		"key", res.Key, "bytes", res.Bytes, "pruned", res.Pruned)
	return ctx.WriteJSON(res)
}
