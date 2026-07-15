package sermon

// Contract tests for the /api/v1/sermons endpoints consumed by church_mobile
// (Dart mirror: lib/src/models/sermon.dart). These freeze key names, the list
// envelope, and the JSON error shape — the app hard-casts `id` and iterates
// arrays without null checks, so contract drift crashes phones at runtime.

import (
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/rohanthewiz/church/resource/apiv1/apitest"
	"github.com/rohanthewiz/rweb"
)

// Routes registered exactly as in router_rweb.go so paths are part of the test.
func newSermonAPIServer() *rweb.Server {
	s := apitest.NewServer()
	api := s.Group("/api/v1")
	api.Get("/sermons", APISermonsRWeb)
	api.Get("/sermons/:id", APISermonRWeb)
	return s
}

var sermonCols = []string{
	"id", "title", "slug", "published", "summary", "body", "audio_link",
	"date_taught", "place_taught", "teacher", "scripture_refs", "categories",
}

func sermonRow(rows *sqlmock.Rows) *sqlmock.Rows {
	return rows.AddRow(
		// Summary deliberately contains a reference: summary_refs offsets are
		// asserted against it below ("Grace in " is 9 bytes).
		int64(42), "On Grace", "on-grace", true, "Grace in Eph 2:8-9 explained", "<p>notes</p>",
		"/sermon-audio/2026/on-grace.mp3",
		time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC), "Main Hall", "Pastor A",
		// Postgres array literals — types.StringArray scans this wire form
		[]byte(`{"John 3:16","Rom 8:1"}`), []byte(`{}`),
	)
}

func TestAPISermonsListContract(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "sermons"`)).
		WillReturnRows(sermonRow(sqlmock.NewRows(sermonCols)))

	status, doc := apitest.GetJSON(t, newSermonAPIServer(), "/api/v1/sermons")
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	apitest.WantKeys(t, doc, "sermons", "limit", "offset")
	if doc["limit"].(float64) != 50 || doc["offset"].(float64) != 0 {
		t.Errorf("default paging should be limit=50 offset=0, got %v/%v", doc["limit"], doc["offset"])
	}

	sermons := doc["sermons"].([]any)
	if len(sermons) != 1 {
		t.Fatalf("want 1 sermon, got %d", len(sermons))
	}
	ser := sermons[0].(map[string]any)
	apitest.WantKeys(t, ser, "id", "title", "summary", "teacher", "place_taught",
		"date_taught", "scripture_refs", "scripture_ref_urls", "summary_refs",
		"categories", "audio_url")

	// id must be numeric — the Dart model does `json['id'] as int`
	if id, ok := ser["id"].(float64); !ok || id != 42 {
		t.Errorf("id must be numeric 42, got %T %v", ser["id"], ser["id"])
	}
	if _, hasBody := ser["body"]; hasBody {
		t.Error("list DTOs must omit body — payload leanness is contract")
	}
	if refs := ser["scripture_refs"].([]any); len(refs) != 2 || refs[0] != "John 3:16" {
		t.Errorf("scripture_refs should be a real array, got %v", refs)
	}
	// scripture_ref_urls is index-aligned with scripture_refs — the app makes
	// each ref chip tappable by position, so alignment is contract.
	urls, _ := ser["scripture_ref_urls"].([]any)
	if len(urls) != 2 ||
		urls[0] != "https://www.blueletterbible.org/nkjv/jhn/3/16/" ||
		urls[1] != "https://www.blueletterbible.org/nkjv/rom/8/1/" {
		t.Errorf("scripture_ref_urls should carry aligned BLB links, got %v", urls)
	}
	// summary_refs carries byte offsets into summary — the app splices
	// tappable spans by start/end, so these values are contract.
	sRefs, _ := ser["summary_refs"].([]any)
	if len(sRefs) != 1 {
		t.Fatalf("want 1 summary ref, got %v", ser["summary_refs"])
	}
	sr := sRefs[0].(map[string]any)
	apitest.WantKeys(t, sr, "book", "slug", "chapter", "verseStart", "verseEnd",
		"raw", "start", "end", "url")
	if sr["raw"] != "Eph 2:8-9" || sr["start"].(float64) != 9 || sr["end"].(float64) != 18 {
		t.Errorf("summary ref offsets wrong: %v", sr)
	}
	if sr["url"] != "https://www.blueletterbible.org/nkjv/eph/2/8-9/" {
		t.Errorf("summary ref url wrong: %v", sr["url"])
	}
	// empty DB array must serialize as [], never null — the app iterates blindly
	if cats, ok := ser["categories"].([]any); !ok || len(cats) != 0 {
		t.Errorf("empty categories must be [], got %T %v", ser["categories"], ser["categories"])
	}
	if ser["audio_url"] != "/sermon-audio/2026/on-grace.mp3" {
		t.Errorf("audio_url should pass through as stored, got %v", ser["audio_url"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPISermonDetailIncludesBody(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "sermons"`)).
		WithArgs(int64(42)).
		WillReturnRows(sermonRow(sqlmock.NewRows(sermonCols)))

	status, doc := apitest.GetJSON(t, newSermonAPIServer(), "/api/v1/sermons/42")
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	if doc["body"] != "<p>notes</p>" {
		t.Errorf("detail must include body, got %v", doc["body"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPISermonBadRequests(t *testing.T) {
	// Param validation happens before any DB touch — no expectations needed
	apitest.MockDB(t)
	s := newSermonAPIServer()

	status, doc := apitest.GetJSON(t, s, "/api/v1/sermons?year=abcd")
	apitest.WantError(t, status, 400, doc)

	status, doc = apitest.GetJSON(t, s, "/api/v1/sermons/not-a-number")
	apitest.WantError(t, status, 400, doc)
}

func TestAPISermonNotFoundIsJSON(t *testing.T) {
	mock := apitest.MockDB(t)
	// No rows: drafts and nonexistent ids answer identically
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "sermons"`)).
		WillReturnRows(sqlmock.NewRows(sermonCols))

	status, doc := apitest.GetJSON(t, newSermonAPIServer(), "/api/v1/sermons/9999")
	apitest.WantError(t, status, 404, doc)
}

// A DB outage must still answer in the JSON error shape — previously the raw
// error bubbled to rweb's HTML error page, which the app can't parse.
func TestAPISermonsInfraErrorIsJSON(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "sermons"`)).
		WillReturnError(errors.New("pq: the database system is starting up"))

	status, doc := apitest.GetJSON(t, newSermonAPIServer(), "/api/v1/sermons")
	apitest.WantError(t, status, 500, doc)
	if doc["error"] != "Could not load sermons" {
		t.Errorf("client-safe message expected, got %v", doc["error"])
	}
}
