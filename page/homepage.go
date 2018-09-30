package page

import (
	"github.com/rohanthewiz/church/chweb/resource/article"
	"github.com/rohanthewiz/church/chweb/resource/sermon"
	"github.com/rohanthewiz/church/chweb/resource/event"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/util/stringops"
)

// Let's keep this as a hardwired home page in case we lose the DB
func Home() (*Page, error) {
	title := "Home"
	pgdef := Presenter{
		Title:              title,
		Slug:               stringops.Slugify(title),
		AvailablePositions: []string{"left", "center"},
	}
	modPres1 := module.Presenter{
		Opts: module.Opts{
			ModuleType: sermon.ModuleTypeRecentSermons,
			Title: "Recent Sermons",
			Published: true,
			LayoutColumn: "left",
			Limit: 8,
		},
	}
	modPres2 := module.Presenter{
		Opts: module.Opts{
			ModuleType: event.ModuleTypeUpcomingEvents,
			Title: "Upcoming Events",
			Published: true,
			LayoutColumn: "left",
			Limit: 8,
		},
	}
	modPres3 := module.Presenter{
		Opts: module.Opts{
			ModuleType: article.ModuleTypeArticlesBlog,
			Title: "Homepage Articles",
			Published: true,
			IsMainModule: true,
			Limit: 4,
		},
	}
	pgdef.Modules = []module.Presenter{modPres1, modPres2, modPres3}
	return pageFromPresenter(pgdef), nil
}
