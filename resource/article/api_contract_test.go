package article

// Contract tests for /api/v1/articles consumed by church_mobile
// (Dart mirror: lib/src/models/article.dart). See resource/apiv1/apitest for
// why these exist and how the DB is stubbed.

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/rohanthewiz/church/resource/apiv1/apitest"
	"github.com/rohanthewiz/rweb"
)

func newArticleAPIServer() *rweb.Server {
	s := apitest.NewServer()
	api := s.Group("/api/v1")
	api.Get("/articles", APIArticlesRWeb)
	api.Get("/articles/:id", APIArticleRWeb)
	return s
}

var articleCols = []string{
	"id", "title", "slug", "published", "summary", "body", "categories",
	"created_at", "updated_at",
}

func articleRow(rows *sqlmock.Rows) *sqlmock.Rows {
	return rows.AddRow(
		int64(7), "Welcome", "welcome", true, "A welcome note", "<p>Hi there</p>",
		[]byte(`{news}`),
		time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC),
		time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC),
	)
}

func TestAPIArticlesListContract(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "articles"`)).
		WillReturnRows(articleRow(sqlmock.NewRows(articleCols)))

	status, doc := apitest.GetJSON(t, newArticleAPIServer(), "/api/v1/articles")
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	apitest.WantKeys(t, doc, "articles", "limit", "offset")
	if doc["limit"].(float64) != 20 {
		t.Errorf("articles default limit is 20, got %v", doc["limit"])
	}

	arts := doc["articles"].([]any)
	if len(arts) != 1 {
		t.Fatalf("want 1 article, got %d", len(arts))
	}
	art := arts[0].(map[string]any)
	apitest.WantKeys(t, art, "id", "title", "slug", "summary", "categories",
		"created_at", "updated_at")
	if id, ok := art["id"].(float64); !ok || id != 7 {
		t.Errorf("id must be numeric 7, got %T %v", art["id"], art["id"])
	}
	if _, hasBody := art["body"]; hasBody {
		t.Error("list DTOs must omit body")
	}
	// Dart parses these with DateTime.tryParse — ISO8601 without zone
	if art["created_at"] != "2026-06-01T10:30:00" {
		t.Errorf("created_at should be ISO8601, got %v", art["created_at"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPIArticleDetailIncludesBody(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "articles"`)).
		WithArgs(int64(7)).
		WillReturnRows(articleRow(sqlmock.NewRows(articleCols)))

	status, doc := apitest.GetJSON(t, newArticleAPIServer(), "/api/v1/articles/7")
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	if doc["body"] != "<p>Hi there</p>" {
		t.Errorf("detail must include the HTML body, got %v", doc["body"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPIArticleBadIdAndNotFoundAreJSON(t *testing.T) {
	mock := apitest.MockDB(t)
	s := newArticleAPIServer()

	status, doc := apitest.GetJSON(t, s, "/api/v1/articles/nope")
	apitest.WantError(t, status, 400, doc)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "articles"`)).
		WillReturnRows(sqlmock.NewRows(articleCols))
	status, doc = apitest.GetJSON(t, s, "/api/v1/articles/9999")
	apitest.WantError(t, status, 404, doc)
}
