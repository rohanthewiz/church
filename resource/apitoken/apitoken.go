// Package apitoken implements DB-backed bearer tokens for the mobile JSON API
// (Phase 2 of ai_docs/plans/2026-0707-mobile-app-flutter-api-plan.md).
//
// Why not the web session store: web sessions live in the in-process kvstore
// with a 30-minute TTL — fine for a browser, useless for a phone that should
// stay signed in for weeks and across server deploys. Tokens therefore live
// in Postgres (api_tokens table), hashed so the DB never holds replayable
// credentials.
//
// Token lifecycle:
//
//	login ──► Issue() ── 32 random bytes, hex ──► client stores plaintext
//	                     SHA-256 hex ──────────► api_tokens.token_hash
//	request ─► APIGuard ─► LookupUser(hash) ──► identity into ctx, touch last_used_at
//	logout ──► RevokeByHash ─► row deleted
//	expiry ──► expires_at passes; lookups stop matching (rows are inert, and
//	           any later issue/revoke sweep can clear them)
//
// Data access is hand-written SQL (no SQLBoiler model) — api_tokens postdates
// the generated models; same precedent as event_recurrences and
// sermon_cache_access.
package apitoken

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

// TokenTTL is how long a mobile login lasts. Fixed (not sliding) so a stolen
// token has a hard upper bound on usefulness; the app simply re-logins when
// it gets a 401, so a 30-day ceiling is invisible to an active user.
const TokenTTL = 30 * 24 * time.Hour

// tokenBytes = 256 bits of entropy — unguessable, no need for structure
// (this is an opaque reference token, not a JWT: revocation and expiry are
// DB lookups, which we need anyway for "log out everywhere").
const tokenBytes = 32

// TokenUser is the identity a valid token resolves to — everything the
// authenticated API paths need, loaded in the guard's single JOIN so handlers
// never re-query.
type TokenUser struct {
	UserID    int64
	Username  string
	FirstName string
	LastName  string
	Email     string
	Role      int
}

// HashToken maps a plaintext bearer token to its storage form. SHA-256 (not
// scrypt) is deliberate: the input is 256 random bits, so brute force is
// hopeless and a fast hash keeps the per-request guard cheap.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// Issue creates a token for the user and returns the plaintext exactly once.
// device is an optional client label ("Pixel 9") to help users recognize
// sessions in any future "manage devices" UI.
func Issue(userID int64, device string) (plain string, expiresAt time.Time, err error) {
	buf := make([]byte, tokenBytes)
	if _, err = rand.Read(buf); err != nil {
		return "", expiresAt, serr.Wrap(err, "error generating api token")
	}
	plain = hex.EncodeToString(buf)

	dbH, err := db.Db()
	if err != nil {
		return "", expiresAt, serr.Wrap(err)
	}
	now := time.Now()
	expiresAt = now.Add(TokenTTL)
	_, err = dbH.Exec(`
		INSERT INTO api_tokens (token_hash, user_id, device, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		HashToken(plain), userID, device, now, expiresAt)
	if err != nil {
		return "", expiresAt, serr.Wrap(err, "error inserting api token")
	}
	return plain, expiresAt, nil
}

// LookupUser resolves a plaintext bearer token to its user. found=false (no
// error) covers every auth-failure case identically — unknown token, expired
// token, disabled user — so the guard's 401 leaks nothing about which it was.
func LookupUser(plain string) (tu TokenUser, found bool, err error) {
	dbH, err := db.Db()
	if err != nil {
		return tu, false, serr.Wrap(err)
	}

	hash := HashToken(plain)
	// Expiry and enabled checks live in the query so the answer is atomic
	// with the read — no window where a just-disabled user still passes.
	var lastName sql.NullString
	row := dbH.QueryRow(`
		SELECT u.id, u.username, u.first_name, u.last_name, u.email_address, u.role
		FROM api_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1 AND t.expires_at > $2 AND u.enabled = true`,
		hash, time.Now())
	err = row.Scan(&tu.UserID, &tu.Username, &tu.FirstName, &lastName, &tu.Email, &tu.Role)
	if err == sql.ErrNoRows {
		return tu, false, nil
	}
	if err != nil {
		return tu, false, serr.Wrap(err, "error looking up api token")
	}
	tu.LastName = lastName.String

	// Touch last_used_at best-effort: it powers "which devices are active"
	// diagnostics only, so a failed touch must not fail the request.
	_, _ = dbH.Exec(`UPDATE api_tokens SET last_used_at = $1 WHERE token_hash = $2`,
		time.Now(), hash)
	return tu, true, nil
}

// RevokeByHash deletes one token (logout of this device). Idempotent: revoking
// an already-gone token is success, not an error.
func RevokeByHash(hash string) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	if _, err = dbH.Exec(`DELETE FROM api_tokens WHERE token_hash = $1`, hash); err != nil {
		return serr.Wrap(err, "error revoking api token")
	}
	return nil
}

// RevokeAllForUser deletes every token a user holds — "log out everywhere",
// and the right call after a password change or account disable.
func RevokeAllForUser(userID int64) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	if _, err = dbH.Exec(`DELETE FROM api_tokens WHERE user_id = $1`, userID); err != nil {
		return serr.Wrap(err, "error revoking user api tokens")
	}
	return nil
}
