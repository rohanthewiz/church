package auth_controller

// Web auth-flow tests: login success/failure, session-cookie issuance, and the
// AdminGuard redirect — the highest-risk untested path called out in
// ai_docs/fable_platform_analysis.md. Handlers run through a real rweb router
// (Server.Request, in-process); only the DB is stubbed (go-sqlmock via
// db.SetHandleForTesting through apitest.MockDB). Sessions use the real
// in-process kvstore, and password verification runs the real scrypt path.
//
// Note: resource/auth's init() loads cfg/random_seeds.txt relative to the
// test package dir, hence the committed cfg/ fixture (same workaround as the
// apitoken and resource contract-test packages).

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/rohanthewiz/church/resource/apiv1/apitest"
	"github.com/rohanthewiz/church/resource/auth"
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
	ad.Get("/home", func(ctx rweb.Context) error {
		return ctx.WriteHTML("admin home")
	})
	return s
}

var formHdr = []rweb.Header{{Key: "Content-Type", Value: "application/x-www-form-urlencoded"}}

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
	resp := s.Request("POST", "/auth", formHdr, strings.NewReader("username=kim&password=secret"))

	if resp.Status() != http.StatusSeeOther {
		t.Fatalf("login status = %d, want 303 (body: %s)", resp.Status(), resp.Body())
	}
	if loc := resp.Header("Location"); loc != "/" {
		t.Errorf("successful login should redirect to /, got %q", loc)
	}
	cookie := sessionCookie(t, resp)

	// The cookie must actually work: the same session passes the AdminGuard.
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
		strings.NewReader("username=kim&password=wrong"))

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
		strings.NewReader("username=nobody&password=whatever"))

	if resp.Status() != http.StatusSeeOther || resp.Header("Location") != "/login" {
		t.Errorf("unknown user should 303 to /login, got %d %q",
			resp.Status(), resp.Header("Location"))
	}
}

func TestWebLoginMissingFieldsRedirectsToLogin(t *testing.T) {
	// No DB expectation on purpose: blank credentials must be rejected before
	// any query runs.
	apitest.MockDB(t)

	resp := newWebAuthServer().Request("POST", "/auth", formHdr, strings.NewReader("username=kim"))
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
}

// errNoRows mimics database/sql's sentinel by message: UserCreds returns the
// raw error and the handler treats any failure as invalid credentials.
type errNoRows struct{}

func (errNoRows) Error() string { return "sql: no rows in result set" }
