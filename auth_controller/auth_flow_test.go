package auth_controller

// Web auth-flow tests: login success/failure, session-cookie issuance, and the
// AdminGuard redirect — the highest-risk untested path called out in
// ai_docs/fable_platform_analysis.md. Handlers run through a real rweb router
// (Server.Request, in-process); only the DB is stubbed (go-sqlmock via
// db.SetHandleForTesting through apitest.MockDB). Sessions use the real
// in-process kvstore, and password verification runs the real scrypt path.

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/resource/apiv1/apitest"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/rweb"
)

// Test credentials: a real scrypt hash so AuthHandlerRWeb's PasswordHash
// comparison runs for real — the mock only fakes the DB, not the crypto.
var (
	webTestSalt = auth.GenSalt("web-auth-flow-test")
	webTestHash = auth.PasswordHash("secret", webTestSalt)
)

// newWebAuthServer wires the routes exactly as router_rweb.go does, so the
// paths and the middleware order are part of what's under test.
func newWebAuthServer() *rweb.Server {
	s := apitest.NewServer()
	s.Post("/auth", AuthHandlerRWeb)
	ad := s.Group("/admin", UseCustomContextRWeb, AdminGuardRWeb)
	// Every admin route is wrapped in a permission decorator, as in the router:
	// the group guard alone cannot stop a handler from running.
	ad.Get("/home", RequireAdmin(func(ctx rweb.Context) error {
		return ctx.WriteHTML("admin home")
	}))
	ad.Get("/articles", Require(authz.ArticlesRead, func(ctx rweb.Context) error {
		return ctx.WriteHTML("articles list")
	}))
	ad.Post("/articles/delete/:id", Require(authz.ArticlesDelete, func(ctx rweb.Context) error {
		return ctx.WriteHTML("deleted")
	}))
	return s
}

var formHdr = []rweb.Header{{Key: "Content-Type", Value: "application/x-www-form-urlencoded"}}

// withCSRF appends a freshly minted form token, mirroring what the login form
// module embeds. Minted per call — tokens are one-shot.
func withCSRF(t *testing.T, form string) string {
	t.Helper()
	tok, err := app.GenerateFormToken()
	if err != nil {
		t.Fatalf("could not generate form token: %v", err)
	}
	return form + "&csrf=" + tok
}

// expectCredsQuery arms the mock for user.UserCreds' SELECT (enabled-only,
// credential columns) with one matching row.
func expectCredsQuery(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "encrypted_password", "encrypted_salt" FROM "users"`)).
		WillReturnRows(sqlmock.NewRows([]string{"encrypted_password", "encrypted_salt"}).
			AddRow(webTestHash, webTestSalt))
}

// sessionCookie extracts the session cookie pair ("church_session=<key>") from
// the login response. StartSessionRWeb sets the session cookie before the
// redirect adds its flash cookie, so the first Set-Cookie is the one we want —
// asserted here so a reordering shows up as a test failure, not silence.
func sessionCookie(t *testing.T, resp rweb.Response) string {
	t.Helper()
	setCookie := resp.Header("Set-Cookie")
	if !strings.HasPrefix(setCookie, session.CookieName+"=") {
		t.Fatalf("first Set-Cookie should be the session cookie %q, got %q", session.CookieName, setCookie)
	}
	return strings.SplitN(setCookie, ";", 2)[0]
}

func TestWebLoginSuccessGrantsAdminSession(t *testing.T) {
	mock := apitest.MockDB(t)
	expectCredsQuery(mock)

	s := newWebAuthServer()
	resp := s.Request("POST", "/auth", formHdr, strings.NewReader(withCSRF(t, "username=kim&password=secret")))

	if resp.Status() != http.StatusSeeOther {
		t.Fatalf("login status = %d, want 303 (body: %s)", resp.Status(), resp.Body())
	}
	if loc := resp.Header("Location"); loc != "/" {
		t.Errorf("successful login should redirect to /, got %q", loc)
	}
	cookie := sessionCookie(t, resp)

	// The cookie must actually work: the same session passes the AdminGuard.
	// kim is a SuperAdmin, so permission resolution stops after the users
	// read (LoadActor never consults a SuperAdmin's roles).
	expectActor(mock, 42, authz.SuperAdminRole)
	resp2 := s.Request("GET", "/admin/home", []rweb.Header{{Key: "Cookie", Value: cookie}}, nil)
	if resp2.Status() != http.StatusOK || !strings.Contains(string(resp2.Body()), "admin home") {
		t.Errorf("session cookie should pass AdminGuard: status=%d body=%s", resp2.Status(), resp2.Body())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestWebLoginWrongPasswordRedirectsToLogin(t *testing.T) {
	mock := apitest.MockDB(t)
	expectCredsQuery(mock) // creds load fine; the scrypt comparison is what fails

	resp := newWebAuthServer().Request("POST", "/auth", formHdr,
		strings.NewReader(withCSRF(t, "username=kim&password=wrong")))

	if resp.Status() != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", resp.Status())
	}
	if loc := resp.Header("Location"); loc != "/login" {
		t.Errorf("failed login should redirect to /login, got %q", loc)
	}
	// No session cookie may be issued on failure — only the flash cookie rides
	// along with the redirect.
	if sc := resp.Header("Set-Cookie"); strings.HasPrefix(sc, session.CookieName+"=") {
		t.Errorf("failed login must not set a session cookie, got %q", sc)
	}
}

// Unknown user answers exactly like a wrong password (no username oracle).
func TestWebLoginUnknownUserRedirectsToLogin(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "encrypted_password", "encrypted_salt" FROM "users"`)).
		WillReturnError(errNoRows{})

	resp := newWebAuthServer().Request("POST", "/auth", formHdr,
		strings.NewReader(withCSRF(t, "username=nobody&password=whatever")))

	if resp.Status() != http.StatusSeeOther || resp.Header("Location") != "/login" {
		t.Errorf("unknown user should 303 to /login, got %d %q",
			resp.Status(), resp.Header("Location"))
	}
}

func TestWebLoginMissingFieldsRedirectsToLogin(t *testing.T) {
	// No DB expectation on purpose: blank credentials must be rejected before
	// any query runs.
	apitest.MockDB(t)

	resp := newWebAuthServer().Request("POST", "/auth", formHdr, strings.NewReader(withCSRF(t, "username=kim")))
	if resp.Status() != http.StatusSeeOther || resp.Header("Location") != "/login" {
		t.Errorf("missing password should 303 to /login, got %d %q",
			resp.Status(), resp.Header("Location"))
	}
}

func TestAdminGuardRedirectsAnonymous(t *testing.T) {
	apitest.MockDB(t) // sessions are kvstore-only; no DB should be touched

	resp := newWebAuthServer().Request("GET", "/admin/home", nil, nil)
	if resp.Status() != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", resp.Status())
	}
	if loc := resp.Header("Location"); loc != "/login" {
		t.Errorf("anonymous admin access should redirect to /login, got %q", loc)
	}
	// rweb runs the route handler when group middleware returns nil without
	// calling Next(), and a redirect returns nil. Before the Require
	// decorators, the admin handler's body rode along behind the 303.
	if strings.Contains(string(resp.Body()), "admin home") {
		t.Errorf("anonymous request must not run the admin handler, body: %s", resp.Body())
	}
}

// A cookie whose key has no session in the kvstore (expired/forged) is anonymous.
func TestAdminGuardRejectsStaleCookie(t *testing.T) {
	apitest.MockDB(t)

	resp := newWebAuthServer().Request("GET", "/admin/home",
		[]rweb.Header{{Key: "Cookie", Value: session.CookieName + "=not-a-real-session-key"}}, nil)
	if resp.Status() != http.StatusSeeOther || resp.Header("Location") != "/login" {
		t.Errorf("stale session cookie should redirect to /login, got %d %q",
			resp.Status(), resp.Header("Location"))
	}
	// Same fall-through leak check as TestAdminGuardRedirectsAnonymous.
	if strings.Contains(string(resp.Body()), "admin home") {
		t.Errorf("stale-cookie request must not run the admin handler, body: %s", resp.Body())
	}
}

// errNoRows mimics database/sql's sentinel by message: UserCreds returns the
// raw error and the handler treats any failure as invalid credentials.
type errNoRows struct{}

func (errNoRows) Error() string { return "sql: no rows in result set" }

// A login POST without a valid CSRF token never reaches credential handling.
func TestWebLoginMissingCSRFRedirectsToLogin(t *testing.T) {
	// No DB expectation: the token check runs before any query.
	apitest.MockDB(t)

	resp := newWebAuthServer().Request("POST", "/auth", formHdr,
		strings.NewReader("username=kim&password=secret"))
	if resp.Status() != http.StatusSeeOther || resp.Header("Location") != "/login" {
		t.Errorf("missing csrf should 303 to /login, got %d %q",
			resp.Status(), resp.Header("Location"))
	}
}

// ---- Role-based admin authorization ----

// expectActor arms the mock for authz.LoadActor's first two reads: the
// enabled user (id, legacy role) and their role assignments. roleIDs empty
// means no role_permissions query follows (LoadActor skips it). A SuperAdmin
// stops after the users read, since its roles are never consulted.
func expectActor(mock sqlmock.Sqlmock, userID int64, legacyRole int, roleIDs ...int64) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, role FROM users WHERE username = $1 AND enabled = $2`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role"}).AddRow(userID, legacyRole))
	if legacyRole == authz.SuperAdminRole {
		return
	}
	rows := sqlmock.NewRows([]string{"role_id"})
	for _, id := range roleIDs {
		rows.AddRow(id)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT role_id FROM user_roles WHERE user_id = $1`)).
		WillReturnRows(rows)
}

// expectRolePerms arms the role_permissions read that follows expectActor
// when the user holds roles.
func expectRolePerms(mock sqlmock.Sqlmock, roleID int64, perms ...authz.Permission) {
	rows := sqlmock.NewRows([]string{"role_id", "permission"})
	for _, p := range perms {
		rows.AddRow(roleID, string(p))
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT role_id, permission FROM role_permissions WHERE role_id IN (`)).
		WillReturnRows(rows)
}

// signedIn plants a session for username directly in the kvstore (skipping
// the login round trip) and returns the Cookie header that presents it.
func signedIn(t *testing.T, username string) []rweb.Header {
	t.Helper()
	key := auth.RandomKey()
	if err := (session.Session{Username: username}).Save(key); err != nil {
		t.Fatalf("could not save session: %v", err)
	}
	return []rweb.Header{{Key: "Cookie", Value: session.CookieName + "=" + key}}
}

// A signed-in member with no roles (e.g. a chat participant) is a user of the
// site, not of its admin: sent home, and the handler never runs.
func TestAdminGuardRejectsMemberWithoutRoles(t *testing.T) {
	mock := apitest.MockDB(t)
	expectActor(mock, 7, 9)

	resp := newWebAuthServer().Request("GET", "/admin/home", signedIn(t, "mem"), nil)
	if resp.Status() != http.StatusSeeOther || resp.Header("Location") != "/" {
		t.Errorf("member without roles should 303 to /, got %d %q", resp.Status(), resp.Header("Location"))
	}
	if strings.Contains(string(resp.Body()), "admin home") {
		t.Errorf("member request must not run the admin handler, body: %s", resp.Body())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// A session whose user was disabled or deleted since sign-in no longer holds.
func TestAdminGuardRejectsDisabledUser(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, role FROM users WHERE username = $1 AND enabled = $2`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role"}))

	resp := newWebAuthServer().Request("GET", "/admin/home", signedIn(t, "gone"), nil)
	if resp.Status() != http.StatusSeeOther || resp.Header("Location") != "/login" {
		t.Errorf("disabled user should 303 to /login, got %d %q", resp.Status(), resp.Header("Location"))
	}
	if strings.Contains(string(resp.Body()), "admin home") {
		t.Errorf("disabled user must not run the admin handler, body: %s", resp.Body())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// Per-route permissions: articles.read opens the list but not the delete.
func TestRequireEnforcesPermission(t *testing.T) {
	mock := apitest.MockDB(t)
	s := newWebAuthServer()
	cookie := signedIn(t, "reader")

	expectActor(mock, 11, 7, 3)
	expectRolePerms(mock, 3, authz.ArticlesRead)
	resp := s.Request("GET", "/admin/articles", cookie, nil)
	if resp.Status() != http.StatusOK || !strings.Contains(string(resp.Body()), "articles list") {
		t.Errorf("articles.read should open the list: status=%d body=%s", resp.Status(), resp.Body())
	}

	// Permissions are re-resolved per request, hence a second set of reads.
	expectActor(mock, 11, 7, 3)
	expectRolePerms(mock, 3, authz.ArticlesRead)
	resp = s.Request("POST", "/admin/articles/delete/1", cookie, nil)
	if resp.Status() != http.StatusSeeOther || resp.Header("Location") != "/admin/home" {
		t.Errorf("delete without articles.delete should 303 to /admin/home, got %d %q",
			resp.Status(), resp.Header("Location"))
	}
	if strings.Contains(string(resp.Body()), "deleted") {
		t.Errorf("denied delete must not run the handler, body: %s", resp.Body())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// SuperAdmin (legacy role 99) bypasses permission checks with no roles at all.
func TestRequireSuperAdminBypass(t *testing.T) {
	mock := apitest.MockDB(t)
	expectActor(mock, 1, authz.SuperAdminRole)

	resp := newWebAuthServer().Request("POST", "/admin/articles/delete/1", signedIn(t, "root"), nil)
	if resp.Status() != http.StatusOK || !strings.Contains(string(resp.Body()), "deleted") {
		t.Errorf("superadmin should pass any permission: status=%d body=%s", resp.Status(), resp.Body())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
