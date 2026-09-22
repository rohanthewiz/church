package page

import (
	"errors"
	"strings"
	"testing"

	"github.com/rohanthewiz/church/module"
)

// A module whose constructor fails must leave a visible message in its slot.
// The substitute error module was once built without Published, and Render
// skips unpublished modules, so the slot rendered empty.
func TestAddModulesShowsErrorModuleOnBuildFailure(t *testing.T) {
	RegisterModules()
	const failing = "test_failing_module"
	modulesRegistry[failing] = func(module.Presenter) (module.Module, error) {
		return nil, errors.New("simulated build failure")
	}
	t.Cleanup(func() { delete(modulesRegistry, failing) })

	pg := pageFromPresenter(Presenter{
		Title:              "Broken",
		AvailablePositions: []string{"center"},
		Modules: []module.Presenter{
			{Opts: module.Opts{ModuleType: failing, Published: true}},
		},
	})

	out := pg.Render("center", nil, false)
	if !strings.Contains(out, "something isn't quite right") {
		t.Fatalf("error module did not render in the failed module's slot; got %q", out)
	}
}

// The hardwired calendar page holds the FullCalendar module, which fetches
// its events from the /calendar feed rather than rendering them itself.
func TestCalendarPageRendersFullCalendar(t *testing.T) {
	RegisterModules()
	pg := Calendar()
	if pg.GetSlug() != CalendarSlug {
		t.Fatalf("slug = %q, want %q", pg.GetSlug(), CalendarSlug)
	}
	out := pg.Render("center", nil, false)
	if !strings.Contains(out, "ch-calendar") || !strings.Contains(out, "events: '/calendar'") {
		t.Fatalf("calendar page did not render the FullCalendar module; got %q", out)
	}
}
