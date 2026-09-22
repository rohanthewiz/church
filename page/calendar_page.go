package page

import (
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/calendar"
)

// CalendarSlug is the slug of the page the default main menu's Calendar item
// links to (/pages/calendar). admin.bootstrapCalendarPage creates a DB row
// with it so admins can rearrange the page; Calendar below is the hardwired
// stand-in when that row does not exist.
const CalendarSlug = "calendar"

// Calendar is the hardwired calendar page, served by the page controller when
// no "calendar" row exists (e.g. the bootstrap failed, or an admin deleted the
// page). It keeps the menu's Calendar link from 404ing, just as Home keeps "/"
// working without a DB page.
//
// The module draws its events from the /calendar JSON feed itself; that feed
// is what the menu used to link to directly, showing visitors a bare `[]`.
func Calendar() *Page {
	return pageFromPresenter(Presenter{
		Title:              "Calendar",
		Slug:               CalendarSlug,
		AvailablePositions: []string{"center"},
		Modules: []module.Presenter{
			{
				Opts: module.Opts{
					ModuleType:   calendar.ModuleTypeFullCalendar,
					Title:        "Calendar",
					Published:    true,
					IsMainModule: true,
					LayoutColumn: "center",
				},
			},
		},
	})
}
