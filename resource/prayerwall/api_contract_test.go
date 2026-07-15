package prayerwall

// Contract tests for /api/v1/prayer-requests consumed by church_mobile —
// freezing the request envelope, the ownership "mine" flag, and the Bearer
// guard placement.
//
// Note: resource/auth's init() loads cfg/random_seeds.txt relative to the
// test package dir, hence the committed cfg/ fixture (same workaround as the
// other API test packages).

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/rohanthewiz/church/resource/apitoken"
	"github.com/rohanthewiz/church/resource/apiv1/apitest"
	"github.com/rohanthewiz/rweb"
)

// Routes registered exactly as in router_rweb.go so paths are part of the test.
func newPrayerAPIServer() *rweb.Server {
	s := apitest.NewServer()
	api := s.Group("/api/v1")
	api.Get("/prayer-requests", APIPrayerRequestsRWeb)
	api.Post("/prayer-requests", apitoken.APIGuard(APIPrayerPostRWeb))
	api.Post("/prayer-requests/:id/answered", apitoken.APIGuard(APIPrayerAnsweredRWeb))
	api.Delete("/prayer-requests/:id", apitoken.APIGuard(APIPrayerDeleteRWeb))
	return s
}

var jsonHdr = []rweb.Header{{Key: "Content-Type", Value: "application/json"}}

func bearer(token string) []rweb.Header {
	return []rweb.Header{
		{Key: "Authorization", Value: "Bearer " + token},
		{Key: "Content-Type", Value: "application/json"},
	}
}

var reqCols = []string{"id", "user_id", "username", "display_name", "title", "body",
	"answered", "answered_note", "published", "created_at", "updated_at"}

func expectBearerUser(mock sqlmock.Sqlmock, userId int64, role int) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.username")).
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "username", "first_name", "last_name", "email_address", "role"}).
			AddRow(userId, "kim", "Kim", "Lee", "kim@example.com", role))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE api_tokens SET last_used_at")).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestAPIPrayerRequestsContract(t *testing.T) {
	mock := apitest.MockDB(t)
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, username")).
		WillReturnRows(sqlmock.NewRows(reqCols).
			AddRow(int64(2), int64(7), "kim", "Kim L", "Healing", "Please pray for my mom",
				true, "She recovered!", true, now, now).
			AddRow(int64(1), int64(8), "joe", "Joe P", "New job", "Interview on Friday",
				false, "", true, now.Add(-time.Hour), now.Add(-time.Hour)))

	status, doc := apitest.GetJSON(t, newPrayerAPIServer(), "/api/v1/prayer-requests")
	if status != 200 {
		t.Fatalf("status = %d, want 200 (doc: %v)", status, doc)
	}
	apitest.WantKeys(t, doc, "prayer_requests", "limit", "offset", "has_more")

	reqs, _ := doc["prayer_requests"].([]any)
	if len(reqs) != 2 {
		t.Fatalf("prayer_requests length = %d, want 2", len(reqs))
	}
	first, _ := reqs[0].(map[string]any)
	apitest.WantKeys(t, first, "id", "username", "display_name", "title", "body",
		"answered", "answered_note", "created_at", "mine")
	// The DTO must not leak other members' user ids
	if _, present := first["user_id"]; present {
		t.Error("prayer request DTO must not carry user_id")
	}
	// Anonymous view: nothing is "mine"
	if first["mine"] != false {
		t.Errorf("mine should be false for anonymous viewer, got %v", first["mine"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPIPrayerPostContract(t *testing.T) {
	mock := apitest.MockDB(t)
	expectBearerUser(mock, 7, 9)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO prayer_requests")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(5)))

	body, _ := json.Marshal(map[string]string{"title": "Travel", "body": "Safe travels for the youth group"})
	status, doc := apitest.RequestJSON(t, newPrayerAPIServer(),
		"POST", "/api/v1/prayer-requests", bearer("tok"), string(body))
	if status != 201 {
		t.Fatalf("status = %d, want 201 (doc: %v)", status, doc)
	}
	req, _ := doc["prayer_request"].(map[string]any)
	apitest.WantKeys(t, req, "id", "username", "display_name", "title", "body",
		"answered", "answered_note", "created_at", "mine")
	if req["mine"] != true {
		t.Errorf("a just-posted request must be mine, got %v", req["mine"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPIPrayerPostRequiresAuth(t *testing.T) {
	apitest.MockDB(t)
	status, doc := apitest.RequestJSON(t, newPrayerAPIServer(),
		"POST", "/api/v1/prayer-requests", jsonHdr, `{"title":"x","body":"y"}`)
	apitest.WantError(t, status, 401, doc)
}

func TestAPIPrayerPostValidation(t *testing.T) {
	mock := apitest.MockDB(t)
	expectBearerUser(mock, 7, 9)
	status, doc := apitest.RequestJSON(t, newPrayerAPIServer(),
		"POST", "/api/v1/prayer-requests", bearer("tok"), `{"title":"","body":""}`)
	apitest.WantError(t, status, 422, doc)
}

func TestAPIPrayerAnsweredRequiresEditor(t *testing.T) {
	mock := apitest.MockDB(t)
	expectBearerUser(mock, 7, 9) // RegisteredUser
	status, doc := apitest.RequestJSON(t, newPrayerAPIServer(),
		"POST", "/api/v1/prayer-requests/5/answered", bearer("tok"), `{"answered":true}`)
	apitest.WantError(t, status, 403, doc)
}

// A member may withdraw their own request but nobody else's.
func TestAPIPrayerDeleteOwnership(t *testing.T) {
	mock := apitest.MockDB(t)
	expectBearerUser(mock, 8, 9) // Joe (id 8), RegisteredUser
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, username")).
		WillReturnRows(sqlmock.NewRows(reqCols).
			AddRow(int64(9), int64(7), "kim", "Kim L", "Healing", "body", false, "", true, now, now))

	status, doc := apitest.RequestJSON(t, newPrayerAPIServer(),
		"DELETE", "/api/v1/prayer-requests/9", bearer("tok"), "")
	apitest.WantError(t, status, 403, doc)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
