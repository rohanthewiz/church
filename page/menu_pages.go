package page

import (
	"github.com/rohanthewiz/church/chweb/util/stringops"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/resource/menu"
)

func MenuForm() (*Page, error) {
	const title = "Menu Form"
	pgdef := Presenter{
		Title: title,
		Slug: stringops.Slugify(title),
		IsAdmin: true,
	}
	modulePres1 := module.Presenter{
		Opts: module.Opts{
			Title:      "Menu Form",
			ModuleType: menu.ModuleTypeMenuForm,
			IsAdmin:    true,
			Published:    true,
			IsMainModule: true,
		},
	}
	pgdef.Modules = []module.Presenter{modulePres1}
	return pageFromPresenter(pgdef), nil
}

func MenusList() (*Page, error) {
	const title = "Menus List"
	pgdef := Presenter{Title: title,
		Slug: stringops.Slugify(title),
		IsAdmin: true,
	}
	modPres := module.Presenter{
		Opts: module.Opts{
			Title: "Menus List",
			ModuleType: menu.ModuleTypeMenusList,
			IsAdmin: true,
			Published: true,
			IsMainModule: true,
			Limit: 20,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return  pageFromPresenter(pgdef), nil
}