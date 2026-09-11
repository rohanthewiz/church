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

var eventLocationCols = []string{"event_id", "latitude", "longitude"}

// The sidecar-table probes, spelled once. Both are ordered expectations, so a
// handler that stopped asking — or asked in a different order — fails here
// rather than quietly serving events with no maps: EventPoints' error is
// logged and not fatal, which is right for a live site and is exactly the
// shape of failure a test can pass straight through.
const (
	recurrenceProbe = `SELECT event_id, freq, weekday, week, until FROM event_recurrences`
	locationProbe   = `SELECT event_id, latitude, longitude FROM event_locations`
)

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
	mock.ExpectQuery(regexp.QuoteMeta(recurrenceProbe)).
		WillReturnRows(sqlmock.NewRows(recurrenceCols))
	// This event has no point of its own — the ordinary case, and the one the
	// client turns into "show the church's map if the location names it".
	mock.ExpectQuery(regexp.QuoteMeta(locationProbe)).
		WillReturnRows(sqlmock.NewRows(eventLocationCols))

	status, doc := apitest.GetJSON(t, newEventAPIServer(),
		"/api/v1/events?from=2026-08-01&to=2026-08-31")
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	apitest.WantKeys(t, doc, "events", "limit", "offset", "has_more")
	if hasMore, _ := doc["has_more"].(bool); hasMore {
		t.Error("has_more must be false when the window fit in one page")
	}

	events := doc["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	evt := events[0].(map[string]any)
	apitest.WantKeys(t, evt, "id", "title", "slug", "summary", "event_date",
		"event_time", "event_location", "contact_person", "contact_phone",
		"contact_email", "contact_url", "categories", "recurring", "location")
	// location is present and non-null even for an event that has none, which
	// is the contract's rule (see EventLocationAPI): the client maps it into a
	// struct with no optional fields.
	loc, ok := evt["location"].(map[string]any)
	if !ok {
		t.Fatalf("location must be an object, got %T %v", evt["location"], evt["location"])
	}
	apitest.WantKeys(t, loc, "configured", "latitude", "longitude")
	if loc["configured"] != false {
		t.Errorf("an event with no point must report configured=false, got %v", loc["configured"])
	}
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

// Events page in memory over the expanded window (no SQL probe), so has_more
// is simply "occurrences remain past this page": two rows at limit=1 must
// yield one event and has_more=true.
func TestAPIEventsHasMore(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events"`)).
		WillReturnRows(eventRow(eventRow(sqlmock.NewRows(eventCols))))
	mock.ExpectQuery(regexp.QuoteMeta(recurrenceProbe)).
		WillReturnRows(sqlmock.NewRows(recurrenceCols))
	mock.ExpectQuery(regexp.QuoteMeta(locationProbe)).
		WillReturnRows(sqlmock.NewRows(eventLocationCols))

	status, doc := apitest.GetJSON(t, newEventAPIServer(),
		"/api/v1/events?from=2026-08-01&to=2026-08-31&limit=1")
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	if hasMore, _ := doc["has_more"].(bool); !hasMore {
		t.Error("has_more must be true when occurrences remain past the page")
	}
	if events := doc["events"].([]any); len(events) != 1 {
		t.Errorf("page must honor limit=1, got %d events", len(events))
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
	mock.ExpectQuery(regexp.QuoteMeta(locationProbe)).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows(eventLocationCols))
	// Second Saturday monthly, open-ended (until NULL)
	mock.ExpectQuery(regexp.QuoteMeta(recurrenceProbe)).
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

// An event with a point of its own, which is the half of the feature the
// fallback exists for: an off-site event the church's own coordinates would
// map to the wrong address entirely.
//
// The numbers are asserted as numbers rather than as strings. JSON has one
// number type and the client reads these into a float64 pair, so a server that
// ever quoted them would deserialize as zero — the Gulf of Guinea — which is
// the one wrong answer that looks like a right one.
func TestAPIEventDetailCarriesItsOwnPoint(t *testing.T) {
	mock := apitest.MockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events"`)).
		WithArgs(int64(9)).
		WillReturnRows(eventRow(sqlmock.NewRows(eventCols)))
	mock.ExpectQuery(regexp.QuoteMeta(locationProbe)).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows(eventLocationCols).AddRow(int64(9), 38.7223, -9.1393))
	mock.ExpectQuery(regexp.QuoteMeta(recurrenceProbe)).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows(recurrenceCols))

	status, doc := apitest.GetJSON(t, newEventAPIServer(), "/api/v1/events/9")
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	loc, ok := doc["location"].(map[string]any)
	if !ok {
		t.Fatalf("detail must carry a location object, got %T %v", doc["location"], doc["location"])
	}
	apitest.WantKeys(t, loc, "configured", "latitude", "longitude")
	if loc["configured"] != true {
		t.Errorf("an event with a point must report configured=true, got %v", loc["configured"])
	}
	lat, ok := loc["latitude"].(float64)
	if !ok || lat != 38.7223 {
		t.Errorf("latitude must be the number 38.7223, got %T %v", loc["latitude"], loc["latitude"])
	}
	lng, ok := loc["longitude"].(float64)
	if !ok || lng != -9.1393 {
		t.Errorf("longitude must be the number -9.1393, got %T %v", loc["longitude"], loc["longitude"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// Every occurrence of a recurring series is held at the series' place. The
// point belongs to the base event and an occurrence is not a row, so a series
// whose base row sits outside the requested window must still carry its point
// into every occurrence inside it.
func TestAPIEventsExpandedOccurrencesCarryTheSeriesPoint(t *testing.T) {
	mock := apitest.MockDB(t)
	// The window query finds nothing: the series is anchored before it.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events"`)).
		WillReturnRows(sqlmock.NewRows(eventCols))
	mock.ExpectQuery(regexp.QuoteMeta(recurrenceProbe)).
		WillReturnRows(sqlmock.NewRows(recurrenceCols).AddRow(int64(9), "weekly", 6, 0, nil))
	// So the id that must reach the location query is the RULE's event id, not
	// a row the window returned — there were none.
	mock.ExpectQuery(regexp.QuoteMeta(locationProbe)).
		WillReturnRows(sqlmock.NewRows(eventLocationCols).AddRow(int64(9), 10.5, -20.25))
	// The base event, fetched by id for the expansion.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events"`)).
		WillReturnRows(eventRow(sqlmock.NewRows(eventCols)))

	status, doc := apitest.GetJSON(t, newEventAPIServer(),
		"/api/v1/events?from=2026-08-01&to=2026-08-31")
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	events := doc["events"].([]any)
	if len(events) == 0 {
		t.Fatal("the series should have produced occurrences in the window")
	}
	for i, raw := range events {
		loc, ok := raw.(map[string]any)["location"].(map[string]any)
		if !ok {
			t.Fatalf("occurrence %d has no location object", i)
		}
		if loc["configured"] != true {
			t.Errorf("occurrence %d lost the series point: %v", i, loc)
		}
		if lat, _ := loc["latitude"].(float64); lat != 10.5 {
			t.Errorf("occurrence %d latitude = %v, want 10.5", i, loc["latitude"])
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
