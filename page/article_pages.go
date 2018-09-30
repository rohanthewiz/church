package page

import (
	"github.com/rohanthewiz/church/chweb/resource/article"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/util/stringops"
)

// Full view of a single article
func ArticleShow() (*Page, error) {
	const title = "Articles Show"
	pgdef := Presenter{
		Title: title,
		Slug: stringops.Slugify(title),
		AvailablePositions: []string{"center", "right"},
	}
	modPres := module.Presenter{
		Opts: module.Opts{
			ModuleType: article.ModuleTypeSingleArticle,
			Title: "Single Article",
			Published: true,
			IsMainModule: true,
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
	pgdef.Modules = []module.Presenter{modPres, modPres2}
	return  pageFromPresenter(pgdef), nil
}

func ArticlesList() (*Page, error) {
	const title = "Articles List"
	pgdef := Presenter{
		Title: title,
		Slug: stringops.Slugify(title),
	}
	modPres := module.Presenter{
		Opts: module.Opts{
			ModuleType: article.ModuleTypeArticlesList,
			Title: "Articles List",
			Published: true,
			Limit: 20,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return  pageFromPresenter(pgdef), nil
}

func AdminArticlesList() (*Page, error) {
	const title = "Articles List - Admin"
	pgdef := Presenter{
		Title: title, Slug: stringops.Slugify(title),
		IsAdmin: true,
	}
	modPres := module.Presenter{
		Opts: module.Opts{
			ModuleType: article.ModuleTypeArticlesList,
			IsAdmin: true,
			Title: "Admin Articles List",
			Published: true,
			IsMainModule: true,
			Limit: 20,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return  pageFromPresenter(pgdef), nil
}

func ArticleForm() (*Page, error) {
	const title = "Article Form"
	pgdef := Presenter{
		Title:              title,
		Slug: stringops.Slugify(title),
		AvailablePositions: []string{"center", "right"},
		IsAdmin: true,
	}
	modPres1 := module.Presenter{
		Opts: module.Opts{
			ModuleType: article.ModuleTypeArticleForm,
			Title: "Article Form",
			Published: true,
			IsMainModule: true,
		},
	}
	modPres2 := module.Presenter{
		Opts: module.Opts{
			ModuleType: article.ModuleTypeRecentArticles,
			Title: "Recent Articles",
			LayoutColumn: "right",
			Published: true,
			Limit: 8,
		},
	}
	pgdef.Modules = []module.Presenter{modPres1, modPres2}
	return  pageFromPresenter(pgdef), nil
}
