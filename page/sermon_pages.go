package page

import (
	"github.com/rohanthewiz/church/chweb/resource/sermon"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/util/stringops"
)

// Full view of a single sermon
func SermonShow() (*Page, error) {
	const title = "Sermons Show"
	pgdef := Presenter{
		Title: title,
		Slug: stringops.Slugify(title),
		AvailablePositions: []string{"center", "right"},
	}
	modPres1 := module.Presenter{
		Opts: module.Opts{
			ModuleType: sermon.ModuleTypeSingleSermon,
			Title: "Single Sermon",
			Published: true,
			IsMainModule: true,
		},
	}
	modPres2 := module.Presenter{
		Opts: module.Opts{
			ModuleType: sermon.ModuleTypeRecentSermons,
			LayoutColumn: "right",
			Title: "Recent Sermons",
			Published: true,
			IsMainModule: false,
			Limit: 10,
		},
	}

	pgdef.Modules = []module.Presenter{modPres1, modPres2}
	return  pageFromPresenter(pgdef), nil
}

func SermonsList() (*Page, error) {
	const title = "Sermons List"
	pgdef := Presenter{
		Title: title, Slug: stringops.Slugify(title),
	}
	modPres := module.Presenter{
		Opts: module.Opts{
			ModuleType: sermon.ModuleTypeSermonsList,
			Title: "Sermons List",
			Published: true,
			IsMainModule: true,
			//Limit: 20,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return pageFromPresenter(pgdef), nil
}

func AdminSermonsList() (*Page, error) {
	const title = "Sermons List - Admin"
	pgdef := Presenter{
		Title: title, Slug: stringops.Slugify(title),
		IsAdmin: true,
	}
	modPres := module.Presenter{
		Opts: module.Opts{
			ModuleType: sermon.ModuleTypeSermonsList,
			IsAdmin: true,
			Title: "Admin Sermons List",
			Published: true,
			IsMainModule: true,
			//Limit: 20,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return  pageFromPresenter(pgdef), nil
}

func SermonForm() (*Page, error) {
	const title = "Sermon Form"
	pgdef := Presenter{
		Title:              title,
		Slug: stringops.Slugify(title),
		AvailablePositions: []string{"center", "right"},
		IsAdmin: true,
	}
	modPres1 := module.Presenter{
		Opts: module.Opts{
			ModuleType: sermon.ModuleTypeSermonForm,
			Title: "Sermon Form",
			Published: true,
			IsMainModule: true,
		},
	}
	modPres2 := module.Presenter{
		Opts: module.Opts{
			ModuleType: sermon.ModuleTypeRecentSermons,
			Title: "Recent Sermons",
			LayoutColumn: "right",
			Published: true,
			Limit: 12,
		},
	}
	pgdef.Modules = []module.Presenter{modPres1, modPres2}
	return  pageFromPresenter(pgdef), nil
}
