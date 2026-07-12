package feed

// Contract test for /api/v1/feed, the mobile home screen's single-request
// aggregate (Dart mirror: lib/src/models/feed.dart). The key promise is
// graceful degradation: a failing section becomes an empty array — the app
// renders what it gets and never sees a hard failure for one bad section.

import (
	"errors"
	"regexp"
	"testing"

	"github.com/rohanthewiz/church/resource/apiv1/apitest"
	"github.com/rohanthewiz/rweb"
)

func newFeedAPIServer() *rweb.Server {
	s := apitest.NewServer()
	s.Group("/api/v1").Get("/feed", APIFeedRWeb)
	return s
}

func TestAPIFeedDegradesSectionsIndependently(t *testing.T) {
	mock := apitest.MockDB(t)
	// Sections query in handler order: articles, sermons, events. Fail them
	// all — the response must still be 200 with three empty arrays.
	errDown := errors.New("pq: the database system is starting up")
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "articles"`)).WillReturnError(errDown)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "sermons"`)).WillReturnError(errDown)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events"`)).WillReturnError(errDown)

	status, doc := apitest.GetJSON(t, newFeedAPIServer(), "/api/v1/feed")
	if status != 200 {
		t.Fatalf("feed must degrade, not fail: status = %d, want 200", status)
	}
	apitest.WantKeys(t, doc, "articles", "sermons", "events")
	for _, section := range []string{"articles", "sermons", "events"} {
		arr, ok := doc[section].([]any)
		if !ok {
			t.Errorf("%s must be an array even on failure, got %T", section, doc[section])
			continue
		}
		if len(arr) != 0 {
			t.Errorf("failed %s section must be empty, got %v", section, arr)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
