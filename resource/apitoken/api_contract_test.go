package apitoken

// Contract tests for /api/v1/auth/* consumed by church_mobile. These freeze
// the login/me/logout JSON shapes, the Bearer guard's 401 behavior, and the
// login throttle — before the Flutter side grows a token store against them.
//
// Note: resource/auth's init() loads cfg/random_seeds.txt relative to the
// test package dir, hence the committed cfg/ fixture (same workaround as the
// sermon/article/event/feed test packages).

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/rohanthewiz/church/resource/apiv1/apitest"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/rweb"
)

// Routes registered exactly as in router_rweb.go so paths are part of the test.
func newAuthAPIServer() *rweb.Server {
	s := apitest.NewServer()
	api := s.Group("/api/v1")
	api.Post("/auth/login", APILoginRWeb)
	api.Get("/auth/me", APIGuard(APIMeRWeb))
	api.Post("/auth/logout", APIGuard(APILogoutRWeb))
	return s
}

var jsonHdr = []rweb.Header{{Key: "Content-Type", Value: "application/json"}}

func bearer(token string) []rweb.Header {
	return []rweb.Header{{Key: "Authorization", Value: "Bearer " + token}}
}

// Test credentials: a real scrypt hash so the handler's PasswordHash
// comparison runs for real — the mock only fakes the DB, not the crypto.
var (
	testSalt = auth.GenSalt("contract-test")
	testHash = auth.PasswordHash("secret", testSalt)
)

// userCols is the subset of users columns the tests return; SQLBoiler binds
// by returned column name, leaving unmentioned fields zero.
var userCols = []string{
	"id", "username", "encrypted_password", "encrypted_salt",
	"role", "enabled", "email_address", "first_name", "last_name",
}

func userRow(rows *sqlmock.Rows, username string) *sqlmock.Rows {
	return rows.AddRow(int64(7), username, testHash, testSalt,
		9, true, "kim@example.com", "Kim", "Lee")
}

func loginBody(username, password string) string {
	b, _ := json.Marshal(map[string]string{"username": username, "password": password})
	return string(b)
}

func TestAPILoginContract(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
		WillReturnRows(userRow(sqlmock.NewRows(userCols), "kim"))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO api_tokens")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	status, doc := apitest.RequestJSON(t, newAuthAPIServer(),
		"POST", "/api/v1/auth/login", jsonHdr, loginBody("kim", "secret"))
	if status != 200 {
		t.Fatalf("status = %d, want 200 (doc: %v)", status, doc)
	}
	apitest.WantKeys(t, doc, "token", "expires_at", "user")

	// The token is opaque contract: non-empty, hex, long enough to be unguessable
	token, _ := doc["token"].(string)
	if len(token) != tokenBytes*2 || !regexp.MustCompile("^[0-9a-f]+$").MatchString(token) {
		t.Errorf("token should be %d hex chars, got %q", tokenBytes*2, token)
	}
	// expires_at is RFC3339 and ~TokenTTL out — the app schedules re-login off it
	expStr, _ := doc["expires_at"].(string)
	exp, err := time.Parse(time.RFC3339, expStr)
	if err != nil {
		t.Errorf("expires_at must be RFC3339, got %q: %v", expStr, err)
	} else if d := time.Until(exp); d < TokenTTL-time.Hour || d > TokenTTL+time.Hour {
		t.Errorf("expires_at should be ~%v out, got %v", TokenTTL, d)
	}

	usr, _ := doc["user"].(map[string]any)
	apitest.WantKeys(t, usr, "id", "username", "first_name", "last_name", "email", "role", "role_name")
	if id, ok := usr["id"].(float64); !ok || id != 7 {
		t.Errorf("user.id must be numeric 7, got %T %v", usr["id"], usr["id"])
	}
	if usr["role_name"] != "RegisteredUser" {
		t.Errorf("role 9 should name RegisteredUser, got %v", usr["role_name"])
	}
	// The credential-free DTO is the contract — hashes must never appear
	for _, forbidden := range []string{"encrypted_password", "encrypted_salt", "password"} {
		if _, present := usr[forbidden]; present {
			t.Errorf("user DTO must not carry %q", forbidden)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// Form-encoded fallback keeps curl-style testing easy.
func TestAPILoginFormFallback(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
		WillReturnRows(userRow(sqlmock.NewRows(userCols), "kim-form"))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO api_tokens")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	formHdr := []rweb.Header{{Key: "Content-Type", Value: "application/x-www-form-urlencoded"}}
	status, doc := apitest.RequestJSON(t, newAuthAPIServer(),
		"POST", "/api/v1/auth/login", formHdr, "username=kim-form&password=secret")
	if status != 200 {
		t.Fatalf("status = %d, want 200 (doc: %v)", status, doc)
	}
}

func TestAPILoginFailures(t *testing.T) {
	s := newAuthAPIServer()

	// Missing fields → 400 before any DB touch
	mock := apitest.MockDB(t)
	status, doc := apitest.RequestJSON(t, s, "POST", "/api/v1/auth/login", jsonHdr,
		loginBody("nopass", ""))
	apitest.WantError(t, status, 400, doc)

	// Unknown user → 401, same message as bad password (no username oracle)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
		WillReturnRows(sqlmock.NewRows(userCols))
	status, doc = apitest.RequestJSON(t, s, "POST", "/api/v1/auth/login", jsonHdr,
		loginBody("ghost", "whatever"))
	apitest.WantError(t, status, 401, doc)
	unknownMsg := doc["error"]

	// Wrong password → 401 with the identical message
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
		WillReturnRows(userRow(sqlmock.NewRows(userCols), "kim-fail"))
	status, doc = apitest.RequestJSON(t, s, "POST", "/api/v1/auth/login", jsonHdr,
		loginBody("kim-fail", "wrong"))
	apitest.WantError(t, status, 401, doc)
	if doc["error"] != unknownMsg {
		t.Errorf("unknown-user and bad-password messages must match: %v vs %v", unknownMsg, doc["error"])
	}
}

// After maxLoginFails failures the throttle answers 429 without touching the
// DB (only maxLoginFails queries are expected below — the final attempt
// making one would fail ExpectationsWereMet).
func TestAPILoginRateLimit(t *testing.T) {
	mock := apitest.MockDB(t)
	s := newAuthAPIServer()

	for i := range maxLoginFails {
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
			WillReturnRows(sqlmock.NewRows(userCols))
		status, doc := apitest.RequestJSON(t, s, "POST", "/api/v1/auth/login", jsonHdr,
			loginBody("throttled", fmt.Sprintf("guess-%d", i)))
		apitest.WantError(t, status, 401, doc)
	}

	status, doc := apitest.RequestJSON(t, s, "POST", "/api/v1/auth/login", jsonHdr,
		loginBody("throttled", "one-more"))
	apitest.WantError(t, status, 429, doc)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// tokenUserCols matches the guard's JOIN select-list ordering.
var tokenUserCols = []string{"id", "username", "first_name", "last_name", "email_address", "role"}

func TestAPIMeContract(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta("FROM api_tokens t")).
		WillReturnRows(sqlmock.NewRows(tokenUserCols).
			AddRow(int64(7), "kim", "Kim", "Lee", "kim@example.com", 9))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE api_tokens SET last_used_at")).
		WillReturnResult(sqlmock.NewResult(0, 1))

	status, doc := apitest.RequestJSON(t, newAuthAPIServer(),
		"GET", "/api/v1/auth/me", bearer("sometoken"), "")
	if status != 200 {
		t.Fatalf("status = %d, want 200 (doc: %v)", status, doc)
	}
	usr, _ := doc["user"].(map[string]any)
	apitest.WantKeys(t, usr, "id", "username", "first_name", "last_name", "email", "role", "role_name")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPIGuardRejections(t *testing.T) {
	mock := apitest.MockDB(t)
	s := newAuthAPIServer()

	// No Authorization header → 401, no DB touch
	status, doc := apitest.RequestJSON(t, s, "GET", "/api/v1/auth/me", nil, "")
	apitest.WantError(t, status, 401, doc)

	// Wrong scheme → 401, no DB touch
	status, doc = apitest.RequestJSON(t, s, "GET", "/api/v1/auth/me",
		[]rweb.Header{{Key: "Authorization", Value: "Basic dXNlcjpwYXNz"}}, "")
	apitest.WantError(t, status, 401, doc)

	// Unknown/expired/disabled all present as an empty JOIN result → 401
	mock.ExpectQuery(regexp.QuoteMeta("FROM api_tokens t")).
		WillReturnRows(sqlmock.NewRows(tokenUserCols))
	status, doc = apitest.RequestJSON(t, s, "GET", "/api/v1/auth/me", bearer("expired"), "")
	apitest.WantError(t, status, 401, doc)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPILogout(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta("FROM api_tokens t")).
		WillReturnRows(sqlmock.NewRows(tokenUserCols).
			AddRow(int64(7), "kim", "Kim", "Lee", "kim@example.com", 9))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE api_tokens SET last_used_at")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Revocation must target the exact token that authenticated the request
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM api_tokens WHERE token_hash")).
		WithArgs(HashToken("livetoken")).
		WillReturnResult(sqlmock.NewResult(0, 1))

	status, doc := apitest.RequestJSON(t, newAuthAPIServer(),
		"POST", "/api/v1/auth/logout", bearer("livetoken"), "")
	if status != 200 {
		t.Fatalf("status = %d, want 200 (doc: %v)", status, doc)
	}
	if ok, _ := doc["ok"].(bool); !ok {
		t.Errorf(`logout should answer {"ok": true}, got %v`, doc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
