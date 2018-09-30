// Pages based on event
package page

import (
	"github.com/rohanthewiz/church/chweb/module"
	//"github.com/rohanthewiz/church/util/string_util"
	"github.com/rohanthewiz/church/chweb/util/stringops"
	"github.com/rohanthewiz/church/chweb/resource/article"
)
// Possible format for page slug: title(snakecased).yyyy-mmdd-randstr

// The Skinny on pages
// Get page presenter by slug from the db (for dynamic pages)
// A Page presenter will have multiple module presenters specific to the page
// Modules Presenters are just a representation of a registered module specified by ModuleType
// If page is unpublished (redirect to home)
// Note: modules will not have a controller, since a module can't exist outside of a page.

// Hardwired Login Page
func LoginPage() (*Page, error) {
	const title = "Login Page"
	pgdef := Presenter{
		Title: title,
		//Slug: stringops.Slugify(title),
		AvailablePositions: []string{"left", "center", "right"}, // we want to squash the login form in center
		IsAdmin: false,
	}
	modulePres1 := module.Presenter{
		Opts: module.Opts{
			Title:      "Login Form",
			ModuleType: ModuleTypeLoginForm,
			LayoutColumn: "center",
			IsAdmin:    false,
			Published:    true,
			IsMainModule: true,
		},
	}
	pgdef.Modules = []module.Presenter{modulePres1}
	return pageFromPresenter(pgdef), nil
}

// Hardwired Page Form Page
func PageForm() (*Page, error) {
	const title = "Page Form"
	pgdef := Presenter{
		Title: title,
		Slug: stringops.Slugify(title),
		IsAdmin: true,
		AvailablePositions: []string{"center", "right"},
	}
	modulePres1 := module.Presenter{
		Opts: module.Opts{
			Title:      "Page Form",
			ModuleType: ModuleTypePageForm,
			IsAdmin:    true,
			Published:    true,
			IsMainModule: true,
			//ItemId:     1,
		},
	}
	modPres2 := module.Presenter{
		Opts: module.Opts{
			ModuleType: article.ModuleTypeRecentArticles,
			Title: "Recent Articles",
			LayoutColumn: "right",
			Published: true,
			IsAdmin: true,
			Limit: 10,
		},
	}
	pgdef.Modules = []module.Presenter{modulePres1, modPres2}
	return pageFromPresenter(pgdef), nil
}

func PagesList() (*Page, error) {
	const title = "Pages List"
	pgdef := Presenter{Title: title,
		Slug: stringops.Slugify(title),
		IsAdmin: true,
	}
	modPres := module.Presenter{
		Opts: module.Opts{
			Title: "Pages List",
			ModuleType: ModuleTypePagesList,
			IsAdmin: true,
			Published: true,
			IsMainModule: true,
			Limit: 20,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return  pageFromPresenter(pgdef), nil
}
