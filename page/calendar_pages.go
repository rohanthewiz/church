// Pages based on calendar
package page

import (
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/calendar"
	"github.com/rohanthewiz/church/util/stringops"
)

// CalendarPage returns a hardwired page that hosts the FullCalendar widget.
// The widget itself fetches event JSON from the /calendar endpoint, so this
// page is just a container — it provides the page chrome (head, scripts,
// menu) plus a single FullCalendar module wired into the main slot.
//
// Mirrors the EventsList / SermonsList pattern: when no DB row exists for
// slug "calendar", PageHandlerRWeb falls back to this presenter so the
// /pages/calendar URL always resolves without requiring an admin to seed
// a page row first.
func CalendarPage() (*Page, error) {
	title := "Calendar"
	pgdef := Presenter{Title: title, Slug: stringops.Slugify(title)}
	modPres := module.Presenter{
		Opts: module.Opts{
			Title:        "Calendar",
			ModuleType:   calendar.ModuleTypeFullCalendar,
			Published:    true,
			IsMainModule: true,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return pageFromPresenter(pgdef), nil
}
