// Pages based on event
package page

import (
	"github.com/rohanthewiz/church/chweb/resource/event"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/util/stringops"
)
// Possible format for page slug: title(snakecased).yyyy-mmdd-randstr

// The Skinny on pages
// Get page presenter by slug from the db (for dynamic pages)
// A Page presenter will have multiple module presenters specific to the page
// Modules Presenters are just a representation of a registered module specified by ModuleType
// If page is unpublished (redirect to home)
// So, module will not have a controller, since a module can't exist outside of a page.


// This is the reference page showing most all available options in comments
func EventWithUpcomingEvents() (*Page, error) {
	title := "Event Show with Upcoming Events"
	pgdef := Presenter{
		Slug: stringops.Slugify(title),
		//IsAdmin: false,
		AvailablePositions: []string{"center", "right"},
	}
	modulePres1 := module.Presenter{
		Opts: module.Opts{
			Title:      "Show Event",
			ModuleType: event.ModuleTypeSingleEvent,
			//IsAdmin:    false,
			Published:    true,
			IsMainModule: true,
			//LayoutColumn: "center",  // center is the default
			//ItemIds:     []int64{1},
		},
	}
	modulePres2 := module.Presenter{
		Opts: module.Opts{
			Title:      "Upcoming Events",
			ModuleType: event.ModuleTypeUpcomingEvents,
			IsAdmin:    false,
			Published:    true,
			//IsMainModule: false,
			LayoutColumn: "right",
			//LayoutOrder:  1,
			Limit: 8,
			//Offset: 0,
			Ascending: true,
		},
	}
	pgdef.Modules = []module.Presenter{modulePres1, modulePres2}
	return  pageFromPresenter(pgdef), nil
}

func EventForm() (*Page, error) {
	title := "Event Form"
	pgdef := Presenter{
		Title: title, Slug: stringops.Slugify(title),
		IsAdmin: true,
		AvailablePositions: []string{"center", "right"},
	}
	modulePres1 := module.Presenter{
		Opts: module.Opts{
			Title:      "Show Event",
			ModuleType: event.ModuleTypeEventForm,
			IsAdmin:    true,
			Published:    true,
			IsMainModule: true,
			//LayoutColumn: "center",
			//ItemId:     1, // Item id is passed via URL Param
		},
	}
	modulePres2 := module.Presenter{
		Opts: module.Opts{
			Title:      "Upcoming Events",
			ModuleType: event.ModuleTypeUpcomingEvents,
			Published:    true,
			LayoutColumn: "right",
			Limit: 8,
		},
	}
	pgdef.Modules = []module.Presenter{modulePres1, modulePres2}
	return pageFromPresenter(pgdef), nil
}
// --------------------------------------------------------------
// A Simple and single Event Show
func EventShow() (*Page, error) {
	title := "Event Show"
	pgdef := Presenter{Title: title, Slug: stringops.Slugify(title)}
	modPres := module.Presenter{
		Opts: module.Opts{
			Title: "Show Event",
			ModuleType: event.ModuleTypeSingleEvent,
			Published: true,
			IsMainModule: true,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return pageFromPresenter(pgdef), nil
}

func EventsList() (*Page, error) {
	title := "Events List"
	pgdef := Presenter{Title: title, Slug: stringops.Slugify(title)}
	modPres := module.Presenter{
		Opts: module.Opts{
			Title: "Events List",
			ModuleType: event.ModuleTypeEventsList,
			Published: true,
			IsMainModule: true,
			Limit: 20,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return  pageFromPresenter(pgdef), nil
}

func AdminEventsList() (*Page, error) {
	title := "Events List - Admin"
	pgdef := Presenter{Title: title, Slug: stringops.Slugify(title), IsAdmin: true}
	modPres := module.Presenter{
		Opts: module.Opts{
			Title: "Admin Events List",
			ModuleType: event.ModuleTypeEventsList,
			IsAdmin: true,
			Published: true,
			IsMainModule: true,
			Limit: 20,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return  pageFromPresenter(pgdef), nil
}
