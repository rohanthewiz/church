package chat

// Contract tests for /api/v1/chat/* consumed by church_mobile (and, since
// the DTO is shared, by the web widget). These freeze the message envelope,
// the moderation 422 shape, and the Bearer guard placement — before the
// Flutter side grows a chat client against them.
//
// Note: resource/auth's init() loads cfg/random_seeds.txt relative to the
// test package dir, hence the committed cfg/ fixture (same workaround as the
// apitoken/sermon/article/event test packages).

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
func newChatAPIServer() *rweb.Server {
	s := apitest.NewServer()
	api := s.Group("/api/v1")
	api.Get("/chat/messages", APIChatMessagesRWeb)
	api.Post("/chat/messages", apitoken.APIGuard(APIChatPostRWeb))
	api.Post("/chat/messages/:id/keep", apitoken.APIGuard(APIChatKeepRWeb))
	api.Delete("/chat/messages/:id", apitoken.APIGuard(APIChatDeleteRWeb))
	return s
}

var jsonHdr = []rweb.Header{{Key: "Content-Type", Value: "application/json"}}

func bearer(token string) []rweb.Header {
	return []rweb.Header{
		{Key: "Authorization", Value: "Bearer " + token},
		{Key: "Content-Type", Value: "application/json"},
	}
}

var msgCols = []string{"id", "channel", "user_id", "username", "display_name", "body", "keep", "created_at"}

// expectBearerUser queues the guard's token lookup (SELECT + best-effort
// last_used touch) resolving to a user with the given id and role.
func expectBearerUser(mock sqlmock.Sqlmock, userId int64, role int) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.username")).
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "username", "first_name", "last_name", "email_address", "role"}).
			AddRow(userId, "kim", "Kim", "Lee", "kim@example.com", role))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE api_tokens SET last_used_at")).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestAPIChatMessagesContract(t *testing.T) {
	mock := apitest.MockDB(t)
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, channel, user_id")).
		WillReturnRows(sqlmock.NewRows(msgCols).
			AddRow(int64(12), "prayer-wall", int64(7), "kim", "Kim L", "Praying for you!", false, now).
			AddRow(int64(11), "prayer-wall", int64(8), "joe", "Joe P", "Amen", true, now.Add(-time.Minute)))

	status, doc := apitest.GetJSON(t, newChatAPIServer(), "/api/v1/chat/messages?channel=prayer-wall")
	if status != 200 {
		t.Fatalf("status = %d, want 200 (doc: %v)", status, doc)
	}
	apitest.WantKeys(t, doc, "channel", "messages", "limit", "has_more")

	msgs, _ := doc["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages length = %d, want 2", len(msgs))
	}
	first, _ := msgs[0].(map[string]any)
	apitest.WantKeys(t, first, "id", "channel", "username", "display_name", "body", "keep", "created_at")
	// Initial window must arrive oldest → newest (append order for clients)
	if id, _ := first["id"].(float64); id != 11 {
		t.Errorf("messages must be ascending by id; first id = %v, want 11", first["id"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPIChatMessagesInvalidChannel(t *testing.T) {
	apitest.MockDB(t) // no queries expected — rejected before the DB
	status, doc := apitest.GetJSON(t, newChatAPIServer(), "/api/v1/chat/messages?channel=No%20Good!")
	apitest.WantError(t, status, 400, doc)
}

func TestAPIChatPostContract(t *testing.T) {
	mock := apitest.MockDB(t)
	expectBearerUser(mock, 7, 9) // RegisteredUser may post
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO chat_messages")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))

	body, _ := json.Marshal(map[string]string{"channel": "community", "body": "Hello church family"})
	status, doc := apitest.RequestJSON(t, newChatAPIServer(),
		"POST", "/api/v1/chat/messages", bearer("tok"), string(body))
	if status != 201 {
		t.Fatalf("status = %d, want 201 (doc: %v)", status, doc)
	}
	msg, _ := doc["message"].(map[string]any)
	apitest.WantKeys(t, msg, "id", "channel", "username", "display_name", "body", "keep", "created_at")
	if msg["display_name"] != "Kim Lee" {
		t.Errorf("display_name should compose first+last, got %v", msg["display_name"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPIChatPostRequiresAuth(t *testing.T) {
	apitest.MockDB(t)
	status, doc := apitest.RequestJSON(t, newChatAPIServer(),
		"POST", "/api/v1/chat/messages", jsonHdr, `{"channel":"community","body":"hi"}`)
	apitest.WantError(t, status, 401, doc)
}

// The moderation reason must surface via the standard error shape so the app
// can show it inline.
func TestAPIChatPostModerationReject(t *testing.T) {
	mock := apitest.MockDB(t)
	expectBearerUser(mock, 21, 9) // distinct user id — keep limiter state isolated

	body, _ := json.Marshal(map[string]string{"channel": "community", "body": "well shit happens"})
	status, doc := apitest.RequestJSON(t, newChatAPIServer(),
		"POST", "/api/v1/chat/messages", bearer("tok"), string(body))
	apitest.WantError(t, status, 422, doc)
}

// Keep/delete are editor-or-above; a RegisteredUser bearer must get 403.
func TestAPIChatModerationRequiresEditor(t *testing.T) {
	mock := apitest.MockDB(t)
	expectBearerUser(mock, 7, 9) // RegisteredUser
	status, doc := apitest.RequestJSON(t, newChatAPIServer(),
		"POST", "/api/v1/chat/messages/42/keep", bearer("tok"), `{"keep":true}`)
	apitest.WantError(t, status, 403, doc)
}

func TestAPIChatKeepContract(t *testing.T) {
	mock := apitest.MockDB(t)
	expectBearerUser(mock, 3, 7) // Author/Editor role may moderate
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, channel, user_id")).
		WillReturnRows(sqlmock.NewRows(msgCols).
			AddRow(int64(42), "community", int64(7), "kim", "Kim L", "keep me", false, time.Now()))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE chat_messages SET keep")).
		WillReturnResult(sqlmock.NewResult(0, 1))

	status, doc := apitest.RequestJSON(t, newChatAPIServer(),
		"POST", "/api/v1/chat/messages/42/keep", bearer("tok"), `{"keep":true}`)
	if status != 200 {
		t.Fatalf("status = %d, want 200 (doc: %v)", status, doc)
	}
	apitest.WantKeys(t, doc, "ok", "id", "keep")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
