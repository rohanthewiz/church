package event

// Contract tests for /api/v1/events consumed by church_mobile
// (Dart mirrors: lib/src/models/event.dart — ChurchEvent + EventRecurrence).
// See resource/apiv1/apitest for why these exist and how the DB is stubbed.
//
// Recurrence *math* is covered by recurrence_test.go; here we only freeze the
// wire shapes: the list envelope and the detail's structured recurrence object.

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/rohanthewiz/church/resource/apiv1/apitest"
	"github.com/rohanthewiz/rweb"
)

func newEventAPIServer() *rweb.Server {
	s := apitest.NewServer()
	api := s.Group("/api/v1")
	api.Get("/events", APIEventsRWeb)
	api.Get("/events/:id", APIEventRWeb)
	return s
}

var eventCols = []string{
	"id", "title", "slug", "published", "summary", "body",
	"event_date", "event_time", "event_location",
	"contact_person", "contact_phone", "contact_email", "contact_url", "categories",
}

var recurrenceCols = []string{"event_id", "freq", "weekday", "week", "until"}

func eventRow(rows *sqlmock.Rows) *sqlmock.Rows {
	return rows.AddRow(
		int64(9), "Prayer Meeting", "prayer-meeting", true, "Weekly gathering", "<p>All welcome</p>",
		time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC), "7:30 PM", "Fellowship Hall",
		"Jane Doe", "555-0100", "jane@example.org", "https://example.org", []byte(`{prayer}`),
	)
}

func TestAPIEventsListContract(t *testing.T) {
	mock := apitest.MockDB(t)
	// WindowedEvents: the window query, then the (empty) recurrence rules
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events"`)).
		WillReturnRows(eventRow(sqlmock.NewRows(eventCols)))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT event_id, freq, weekday, week, until FROM event_recurrences`)).
		WillReturnRows(sqlmock.NewRows(recurrenceCols))

	status, doc := apitest.GetJSON(t, newEventAPIServer(),
		"/api/v1/events?from=2026-08-01&to=2026-08-31")
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	apitest.WantKeys(t, doc, "events", "limit", "offset")

	events := doc["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	evt := events[0].(map[string]any)
	apitest.WantKeys(t, evt, "id", "title", "slug", "summary", "event_date",
		"event_time", "event_location", "contact_person", "contact_phone",
		"contact_email", "contact_url", "categories", "recurring")
	if id, ok := evt["id"].(float64); !ok || id != 9 {
		t.Errorf("id must be numeric 9, got %T %v", evt["id"], evt["id"])
	}
	// event_date is a plain date; the app composes it with free-form event_time
	if evt["event_date"] != "2026-08-15" {
		t.Errorf("event_date must be YYYY-MM-DD, got %v", evt["event_date"])
	}
	if evt["recurring"] != false {
		t.Errorf("one-time event must report recurring=false, got %v", evt["recurring"])
	}
	if _, hasBody := evt["body"]; hasBody {
		t.Error("list DTOs must omit body")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPIEventsBadWindowIsJSON(t *testing.T) {
	apitest.MockDB(t) // validation precedes any DB touch
	s := newEventAPIServer()

	status, doc := apitest.GetJSON(t, s, "/api/v1/events?from=08/01/2026")
	apitest.WantError(t, status, 400, doc)

	status, doc = apitest.GetJSON(t, s, "/api/v1/events?to=Aug-31")
	apitest.WantError(t, status, 400, doc)
}

// The detail endpoint must carry the structured recurrence rule exactly as
// the Dart EventRecurrence model reads it: freq/weekday/week/until/desc.
func TestAPIEventDetailRecurrenceContract(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events"`)).
		WithArgs(int64(9)).
		WillReturnRows(eventRow(sqlmock.NewRows(eventCols)))
	// Second Saturday monthly, open-ended (until NULL)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT event_id, freq, weekday, week, until FROM event_recurrences`)).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows(recurrenceCols).AddRow(int64(9), "monthly", 6, 2, nil))

	status, doc := apitest.GetJSON(t, newEventAPIServer(), "/api/v1/events/9")
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	if doc["body"] != "<p>All welcome</p>" {
		t.Errorf("detail must include body, got %v", doc["body"])
	}
	if doc["recurring"] != true {
		t.Errorf("recurring must be true, got %v", doc["recurring"])
	}
	if desc, _ := doc["recurrence_desc"].(string); desc == "" {
		t.Error("recurrence_desc must be populated for recurring events")
	}

	rec, ok := doc["recurrence"].(map[string]any)
	if !ok {
		t.Fatalf("detail must include the structured recurrence object, got %v", doc["recurrence"])
	}
	apitest.WantKeys(t, rec, "freq", "weekday", "desc")
	if rec["freq"] != "monthly" || rec["weekday"].(float64) != 6 || rec["week"].(float64) != 2 {
		t.Errorf("recurrence rule mismatch: %v", rec)
	}
	if _, hasUntil := rec["until"]; hasUntil {
		t.Error("open-ended series must omit until (omitempty)")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAPIEventNotFoundIsJSON(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events"`)).
		WillReturnRows(sqlmock.NewRows(eventCols))

	status, doc := apitest.GetJSON(t, newEventAPIServer(), "/api/v1/events/404")
	apitest.WantError(t, status, 404, doc)
}
