package page

import (
	"github.com/rohanthewiz/church/chweb/util/stringops"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/resource/user"
)

func UserForm() (*Page, error) {
	title := "User Form"
	pgdef := Presenter{
		Title: title, Slug: stringops.Slugify(title),
		IsAdmin: true,
		AvailablePositions: []string{"center"}, //, "right"
	}
	modulePres1 := module.Presenter{
		Opts: module.Opts{
			Title:      "Show User",
			ModuleType: user.ModuleTypeUserForm,
			IsAdmin:    true,
			Published:    true,
			IsMainModule: true,
			//LayoutColumn: "center",
			//ItemId:     1, // Item id is passed via URL Param
		},
	}
	//modulePres2 := module.Presenter{
	//	Opts: module.Opts{
	//		Title:      "Recent Users",
	//		ModuleType: user.ModuleTypeRecentUsers,
	//		Published:    true,
	//		LayoutColumn: "right",
	//		Limit: 8,
	//	},
	//}
	pgdef.Modules = []module.Presenter{modulePres1} //, modulePres2
	return pageFromPresenter(pgdef), nil
}

func UsersList() (*Page, error) {
	title := "Users List"
	pgdef := Presenter{Title: title, Slug: stringops.Slugify(title), IsAdmin: true}
	modPres := module.Presenter{
		Opts: module.Opts{
			Title: "Users List",
			ModuleType: user.ModuleTypeUsersList,
			IsAdmin: true,
			Published: true,
			IsMainModule: true,
			Limit: 25,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return  pageFromPresenter(pgdef), nil
}
