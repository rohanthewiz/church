package apitoken

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/apiv1"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

// APIUser is the JSON DTO for the authenticated user — a deliberate,
// credential-free subset (user.Presenter and models.User both carry password
// hash fields and must never be serialized; see the mobile plan doc).
// Key names are contract with church_mobile — snake_case like the rest of
// /api/v1.
type APIUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Role      int    `json:"role"`
	RoleName  string `json:"role_name"`
}

func (tu TokenUser) apiUser() APIUser {
	return APIUser{
		ID:        tu.UserID,
		Username:  tu.Username,
		FirstName: tu.FirstName,
		LastName:  tu.LastName,
		Email:     tu.Email,
		Role:      tu.Role,
		RoleName:  user.RoleToString[tu.Role],
	}
}

// Context keys the guard publishes for downstream handlers. Distinct from the
// web middleware's "session"/"isAdmin" keys — the two auth systems must not
// be confused for one another (a Bearer token grants API access, never the
// admin HTML UI).
const (
	ctxKeyAPIUser      = "apiUser"
	ctxKeyAPITokenHash = "apiTokenHash"
)

// ---------------------------------------------------------------------------
// Login rate limiting
//
// Mobile exposes /auth/login to scripted guessing far more than the web form,
// so failures are throttled per (client IP, username): a sliding window that
// only counts *failed* attempts and resets on success. In-process on purpose —
// at church scale a distributed limiter is overkill, and the worst case of a
// multi-instance deploy is a proportionally higher (still tiny) budget.
// ---------------------------------------------------------------------------

const (
	maxLoginFails   = 10
	loginFailWindow = 15 * time.Minute
)

// failedLoginLimiter tracks recent failure timestamps per key.
type failedLoginLimiter struct {
	mu    sync.Mutex
	fails map[string][]time.Time
}

var loginLimiter = &failedLoginLimiter{fails: map[string][]time.Time{}}

// prune drops entries older than the window; called under the lock.
func (l *failedLoginLimiter) prune(key string, now time.Time) {
	kept := l.fails[key][:0]
	for _, t := range l.fails[key] {
		if now.Sub(t) < loginFailWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, key) // don't let dead keys accumulate forever
	} else {
		l.fails[key] = kept
	}
}

// allowed reports whether another attempt may proceed right now.
func (l *failedLoginLimiter) allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(key, time.Now())
	return len(l.fails[key]) < maxLoginFails
}

// fail records a failed attempt.
func (l *failedLoginLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[key] = append(l.fails[key], time.Now())
}

// clear wipes the key after a successful login.
func (l *failedLoginLimiter) clear(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// loginRequest is the POST /api/v1/auth/login body. JSON is the primary form
// (what the Flutter client sends); urlencoded form values are accepted as a
// fallback so curl-style testing stays easy.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Device   string `json:"device"` // optional client label, e.g. "Pixel 9"
}

// APILoginRWeb handles POST /api/v1/auth/login.
// 200 → {"token": ..., "expires_at": RFC3339, "user": {APIUser}}
// 400 missing fields / 401 bad credentials / 429 throttled — all {"error": msg}.
func APILoginRWeb(ctx rweb.Context) error {
	var req loginRequest
	if body := ctx.Request().Body(); len(body) > 0 {
		// A malformed body isn't fatal by itself — the form fallback below
		// may still supply credentials (e.g. urlencoded POST).
		_ = json.Unmarshal(body, &req)
	}
	if req.Username == "" && req.Password == "" {
		req.Username = ctx.Request().FormValue("username")
		req.Password = ctx.Request().FormValue("password")
	}
	if req.Username == "" || req.Password == "" {
		return apiv1.Error(ctx, http.StatusBadRequest, "username and password are required")
	}

	// Keyed by IP+username: one attacker can't lock out a user from
	// everywhere, and a distributed attack on one account still trips the
	// per-username component from each source.
	limiterKey := ctx.ClientIP() + "|" + req.Username
	if !loginLimiter.allowed(limiterKey) {
		return apiv1.Error(ctx, http.StatusTooManyRequests,
			"Too many login attempts. Please try again later.")
	}

	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not process login")
	}
	au, found, err := user.AuthUserByUsername(dbH, req.Username)
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not process login")
	}
	// Unknown user and wrong password answer identically — no username oracle.
	// (Same generic message the web login uses; never log the password.)
	if !found || auth.PasswordHash(req.Password, au.Salt) != au.PassHash {
		loginLimiter.fail(limiterKey)
		logger.Log("warn", "API login attempt failed", "username", req.Username, "ip", ctx.ClientIP())
		return apiv1.Error(ctx, http.StatusUnauthorized, "Invalid username or password")
	}
	loginLimiter.clear(limiterKey)

	token, expiresAt, err := Issue(dbH, au.ID, strings.TrimSpace(req.Device))
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not process login")
	}

	logger.Info("API login", "username", au.Username)
	return ctx.WriteJSON(map[string]any{
		"token":      token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
		"user": APIUser{
			ID:        au.ID,
			Username:  au.Username,
			FirstName: au.FirstName,
			LastName:  au.LastName,
			Email:     au.EmailAddress,
			Role:      au.Role,
			RoleName:  user.RoleToString[au.Role],
		},
	})
}

// APIGuard wraps a handler with Bearer authentication — the token-world
// parallel of the web's AdminGuardRWeb (which redirects to a login page; an
// API must answer 401 JSON instead). On success the resolved TokenUser is in
// the context for the wrapped handler.
//
// A decorator, deliberately not group middleware: rweb's group chain
// auto-continues into the route handler whenever middleware returns nil
// without calling Next(), so a middleware auth denial (which returns nil
// after writing its 401) would run the handler anyway and double-write the
// body. Wrapping makes "denied means the handler never runs" structural.
func APIGuard(next rweb.Handler) rweb.Handler {
	return func(ctx rweb.Context) error {
		const scheme = "Bearer "
		authz := ctx.Request().Header("Authorization")
		if len(authz) <= len(scheme) || !strings.EqualFold(authz[:len(scheme)], scheme) {
			return apiv1.Error(ctx, http.StatusUnauthorized, "Authentication required")
		}
		plain := strings.TrimSpace(authz[len(scheme):])

		dbH, err := db.Db()
		if err != nil {
			return apiv1.ServerError(ctx, err, "Could not verify credentials")
		}
		tu, found, err := LookupUser(dbH, plain)
		if err != nil {
			return apiv1.ServerError(ctx, err, "Could not verify credentials")
		}
		if !found {
			return apiv1.Error(ctx, http.StatusUnauthorized, "Invalid or expired token")
		}

		ctx.Set(ctxKeyAPIUser, tu)
		// The hash (never the plaintext) rides along so logout can revoke the
		// exact token that authenticated this request.
		ctx.Set(ctxKeyAPITokenHash, HashToken(plain))
		return next(ctx)
	}
}

// APIMeRWeb handles GET /api/v1/auth/me — the signed-in identity, enveloped
// as {"user": {...}} to match the login response.
func APIMeRWeb(ctx rweb.Context) error {
	tu, ok := ctx.Get(ctxKeyAPIUser).(TokenUser)
	if !ok { // only reachable if routed without APIGuard — a wiring bug
		return apiv1.Error(ctx, http.StatusUnauthorized, "Authentication required")
	}
	return ctx.WriteJSON(map[string]any{"user": tu.apiUser()})
}

// APILogoutRWeb handles POST /api/v1/auth/logout — revokes the presented
// token only (other devices stay signed in).
func APILogoutRWeb(ctx rweb.Context) error {
	hash, ok := ctx.Get(ctxKeyAPITokenHash).(string)
	if !ok {
		return apiv1.Error(ctx, http.StatusUnauthorized, "Authentication required")
	}
	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not log out")
	}
	if err := RevokeByHash(dbH, hash); err != nil {
		return apiv1.ServerError(ctx, err, "Could not log out")
	}
	return ctx.WriteJSON(map[string]bool{"ok": true})
}
