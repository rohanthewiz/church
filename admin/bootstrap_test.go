package admin

import (
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/rohanthewiz/church/config"
	theDB "github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/internal/testdb"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/page"
	"github.com/vattle/sqlboiler/queries/qm"
)

var sampleItems = []bootstrapMenuItem{
	{Label: "Home", Url: "/"},
	{Label: "Admin", SubMenuSlug: "admin-submenu"},
}

func TestMenuItemsEqual(t *testing.T) {
	// The same items with the key order and spacing Postgres JSONB returns:
	// shorter keys first, a space after each colon and comma.
	reformatted := `[{"url": "/", "label": "Home", "sub_menu_slug": ""}, ` +
		`{"url": "", "label": "Admin", "sub_menu_slug": "admin-submenu"}]`
	if !menuItemsEqual([]byte(reformatted), sampleItems) {
		t.Error("reformatted but identical items compared unequal")
	}

	changed := `[{"label":"Home","url":"/calendar","sub_menu_slug":""},` +
		`{"label":"Admin","url":"","sub_menu_slug":"admin-submenu"}]`
	if menuItemsEqual([]byte(changed), sampleItems) {
		t.Error("items with a changed url compared equal")
	}
	if menuItemsEqual([]byte(`[{"label":"Home","url":"/","sub_menu_slug":""}]`), sampleItems) {
		t.Error("a shorter item list compared equal")
	}
	if menuItemsEqual([]byte(`not json`), sampleItems) {
		t.Error("undecodable stored items compared equal")
	}
}

// TestMenuItemsEqualAfterJSONB round-trips marshaled items through a real
// Postgres jsonb cast, which is where the byte comparison broke. It reads
// only (a SELECT of a literal), so any database will do. Set
// CHURCH_TEST_PG_DSN to run it, e.g.
// postgres://devuser:secret@localhost:5432/church_test?sslmode=disable
func TestMenuItemsEqualAfterJSONB(t *testing.T) {
	dsn := os.Getenv("CHURCH_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("CHURCH_TEST_PG_DSN not set")
	}
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	written, err := json.Marshal(sampleItems)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := conn.QueryRow(`SELECT $1::jsonb::text`, string(written)).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	// The premise of the fix: JSONB does not hand back the bytes written.
	if stored == string(written) {
		t.Logf("note: jsonb returned the written bytes unchanged: %s", stored)
	}
	if !menuItemsEqual([]byte(stored), sampleItems) {
		t.Fatalf("items did not survive the jsonb round trip:\n written %s\n stored  %s", written, stored)
	}
}

// TestBootstrapCalendarPageAndMenus runs the real Bootstrap twice, as
// consecutive boots of a site would, on each backend (internal/testdb).
// Postgres is where the per-boot menu rewrite (N-052) was seen.
func TestBootstrapCalendarPageAndMenus(t *testing.T) {
	if testing.Short() {
		t.Skip("boots a database; skipped with -short")
	}
	testdb.Each(t, bootstrapCalendarPageAndMenus)
}

func bootstrapCalendarPageAndMenus(t *testing.T) {
	dbH, err := theDB.Db()
	if err != nil {
		t.Fatal(err)
	}
	// Bootstrap reads admin credentials from config; none are set, so it
	// skips creating a SuperAdmin.
	config.Options = &config.EnvConfig{}

	mainMenuItems := func() string {
		t.Helper()
		m, err := models.MenuDefs(dbH, qm.Where("slug = ?", "main-menu")).One()
		if err != nil {
			t.Fatal(err)
		}
		return string(m.Items.JSON)
	}
	calendarPages := func() int64 {
		t.Helper()
		n, err := models.Pages(dbH, qm.Where("slug = ?", page.CalendarSlug)).Count()
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	Bootstrap()
	if n := calendarPages(); n != 1 {
		t.Fatalf("calendar pages after first boot = %d, want 1", n)
	}
	if items := mainMenuItems(); !strings.Contains(items, `"/pages/calendar"`) {
		t.Fatalf("main menu does not link the calendar page: %s", items)
	}

	// Stored JSON comes back normalized (bytdb sorts keys and compacts;
	// Postgres JSONB reorders and adds spaces), so a byte comparison would
	// see a change on every boot. Detect a rewrite through updated_at, which
	// the model's Update stamps: plant a past value and it must survive.
	// Reformatting the stored items first exercises the comparison against
	// bytes that differ from what bootstrap marshals.
	var items []bootstrapMenuItem
	if err := json.Unmarshal([]byte(mainMenuItems()), &items); err != nil {
		t.Fatal(err)
	}
	reformatted, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	past := time.Date(2001, time.February, 3, 4, 5, 6, 0, time.UTC)
	if _, err := dbH.Exec(`UPDATE menu_defs SET items = $1, updated_at = $2 WHERE slug = 'main-menu'`,
		string(reformatted), past); err != nil {
		t.Fatal(err)
	}
	Bootstrap()
	if n := calendarPages(); n != 1 {
		t.Fatalf("calendar pages after second boot = %d, want 1", n)
	}
	m, err := models.MenuDefs(dbH, qm.Where("slug = ?", "main-menu")).One()
	if err != nil {
		t.Fatal(err)
	}
	if !m.UpdatedAt.Time.Equal(past) {
		t.Fatalf("second boot rewrote an unchanged menu (updated_at %v, planted %v)", m.UpdatedAt.Time, past)
	}

	// A menu bootstrap wrote under an older framework version (Calendar
	// pointing at the JSON feed) is still refreshed.
	old := strings.Replace(mainMenuItems(), `"/pages/calendar"`, `"/calendar"`, 1)
	if _, err := dbH.Exec(`UPDATE menu_defs SET items = $1 WHERE slug = 'main-menu'`, old); err != nil {
		t.Fatal(err)
	}
	Bootstrap()
	if items := mainMenuItems(); !strings.Contains(items, `"/pages/calendar"`) {
		t.Fatalf("an outdated uncustomized menu was not refreshed: %s", items)
	}
}
