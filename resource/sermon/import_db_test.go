package sermon

// sermon.Import against real databases: the legacy source it reads through
// the pg2 config, and the site database it writes to, on both backends.
//
//	legacy "sermons" (throwaway Postgres, testdb.EmptyPostgres)
//	        │  config.Options.PG2 ──► db.InitDB2 ──► sqlGetSermons
//	        ▼
//	Import() ──► Presenter.Upsert ──► site DB (testdb.Each: bytdb, postgres)
//
// The legacy table is created here from the columns sqlGetSermons selects.
// The real legacy schema is not in this repo; what matters is the shape the
// query needs: text[] for scripture_refs and categories (it applies
// array_to_string), and a date for date_taught, which lib/pq returns as a
// time.Time that database/sql formats as RFC 3339 when scanning into a
// string (hence Import's split on "T").
//
// The source is always Postgres, so without CHURCH_TEST_PG_DSN both subtests
// skip, the bytdb one included.

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/internal/testdb"
	"github.com/rohanthewiz/church/models"
	"github.com/vattle/sqlboiler/queries/qm"
)

func TestImportFromLegacyDB(t *testing.T) {
	testdb.Each(t, func(t *testing.T) {
		legacy := testdb.EmptyPostgres(t)
		src, err := sql.Open("postgres", legacy.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer src.Close()

		// Import checks length with len() (bytes) and blanks, rather than
		// truncates, a summary or body over 300.
		long := strings.Repeat("x", 301)
		_, err = src.Exec(`
			CREATE TABLE sermons (
				name text, summary text, scripture_refs text[], "text" text, teacher text,
				date_taught date, place_taught text, audio_link text, categories text[]
			);
			INSERT INTO sermons VALUES
				('Grace Abounds', 'Short summary', '{"John 3:16","Rom 5:20"}', 'Body text', 'Pastor Ray',
				 '2015-03-08', 'Main Hall', '/sermons/2015/grace.mp3', '{faith,grace}'),
				('Long Notes', '` + long + `', '{}', '` + long + `', 'Pastor Ray',
				 '2016-11-20', 'Main Hall', '', '{}');`)
		if err != nil {
			t.Fatalf("create legacy table: %v", err)
		}

		prev := config.Options
		cfg := &config.EnvConfig{}
		cfg.PG2.Host, cfg.PG2.Port, cfg.PG2.User = legacy.Host, legacy.Port, legacy.User
		cfg.PG2.Word, cfg.PG2.Database = legacy.Word, legacy.Database
		config.Options = cfg
		t.Cleanup(func() { config.Options = prev })

		if got := string(Import()); got != `{"success": true, "count": 2}` {
			t.Fatalf("Import() = %s", got)
		}

		dbH, err := db.Db()
		if err != nil {
			t.Fatal(err)
		}
		find := func(title string) *models.Sermon {
			t.Helper()
			s, err := models.Sermons(dbH, qm.Where("title = ?", title)).One()
			if err != nil {
				t.Fatalf("imported sermon %q not found: %v", title, err)
			}
			return s
		}

		g := find("Grace Abounds")
		if g.Slug.String == "" || !g.Published || g.UpdatedBy != "Importer" || g.Teacher != "Pastor Ray" ||
			g.PlaceTaught.String != "Main Hall" || g.Summary.String != "Short summary" || g.Body.String != "Body text" {
			t.Errorf("Grace Abounds fields = slug %q published %v by %q teacher %q place %q summary %q body %q",
				g.Slug.String, g.Published, g.UpdatedBy, g.Teacher, g.PlaceTaught.String, g.Summary.String, g.Body.String)
		}
		// Only the calendar date survives: Import keeps what precedes "T" and
		// the presenter stamps 11:00 server time on it.
		if d := g.DateTaught.Format("2006-01-02"); d != "2015-03-08" {
			t.Errorf("date_taught = %s, want 2015-03-08", d)
		}
		// Legacy links are site-relative (/sermons/<rest>); transformAudioLink
		// drops the first path segment and rehomes the rest under mediasave.
		if g.AudioLink.String != "http://mediasave.org/cema/2015/grace.mp3" {
			t.Errorf("audio_link = %q", g.AudioLink.String)
		}
		if strings.Join(g.ScriptureRefs, "|") != "John 3:16|Rom 5:20" || strings.Join(g.Categories, "|") != "faith|grace" {
			t.Errorf("scripture_refs = %v, categories = %v", g.ScriptureRefs, g.Categories)
		}

		n := find("Long Notes")
		if n.Summary.String != "" || n.Body.String != "" {
			t.Errorf("over-long summary/body should be blanked, got %d / %d bytes", len(n.Summary.String), len(n.Body.String))
		}
		// Empty legacy arrays come through as one "" element, which the
		// presenter trims away; no audio link stays NULL.
		if len(n.ScriptureRefs) != 0 || len(n.Categories) != 0 || n.AudioLink.Valid {
			t.Errorf("empty fields = refs %v, categories %v, audio %v", n.ScriptureRefs, n.Categories, n.AudioLink)
		}
	})
}
