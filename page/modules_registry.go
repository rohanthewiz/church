package page

import (
	"github.com/rohanthewiz/church/chweb/resource/article"
	"github.com/rohanthewiz/church/chweb/resource/event"
	"github.com/rohanthewiz/church/chweb/resource/sermon"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/resource/menu"
	"github.com/rohanthewiz/church/chweb/resource/user"
	"strings"
	"github.com/rohanthewiz/church/chweb/slick_carousel"
	"sort"
	"github.com/rohanthewiz/church/chweb/resource/content"
	"github.com/rohanthewiz/church/chweb/resource/calendar"
)

const numOfModules = 15
var modulesRegistry map[string]func(module.Presenter) (module.Module, error)
var moduleTypeToName map[string]module.Name
var moduleContentBy map[string]content.ModuleContentType // How to filter module content

// Todo - refactor to a place like string utils
// Registry of modules need to ensure an entry here though
var singularToPlural map[string]string

// Map module creator functions to moduleType
// Register modules once before the web server starts
func RegisterModules() {
	modulesRegistry = make(map[string]func(module.Presenter) (module.Module, error), numOfModules)
	moduleTypeToName = make(map[string]module.Name, numOfModules)
	moduleContentBy = make(map[string]content.ModuleContentType, numOfModules / 2)
	singularToPlural = make(map[string]string, numOfModules / 2) // half number of modules is a fair capacity estimate

	singularToPlural["article"] = "articles"
	addToRegistry(article.ModuleTypeArticleForm, "article", article.NewModuleArticleForm)
	moduleContentBy[article.ModuleTypeArticleForm] = content.ModuleContentByForm
	addToRegistry(article.ModuleTypeArticlesList, "article", article.NewModuleArticlesList)
	moduleContentBy[article.ModuleTypeArticlesList] = content.ModuleContentByPagination
	addToRegistry(article.ModuleTypeArticlesBlog, "article", article.NewModuleArticlesBlog)
	moduleContentBy[article.ModuleTypeArticlesBlog] = content.ModuleContentByMultiId
	addToRegistry(article.ModuleTypeRecentArticles, "article", article.NewModuleRecentArticles)
	moduleContentBy[article.ModuleTypeRecentArticles] = content.ModuleContentByPagination
	addToRegistry(article.ModuleTypeSingleArticle, "article", article.NewModuleSingleArticle)
	moduleContentBy[article.ModuleTypeSingleArticle] = content.ModuleContentBySingleId

	singularToPlural["event"] = "events"
	addToRegistry(event.ModuleTypeSingleEvent, "event", event.NewModuleSingleEvent)
	moduleContentBy[event.ModuleTypeSingleEvent] = content.ModuleContentBySingleId
	addToRegistry(event.ModuleTypeEventForm, "event", event.NewModuleEventForm)
	moduleContentBy[event.ModuleTypeEventForm] = content.ModuleContentByForm
	addToRegistry(event.ModuleTypeEventsList, "event", event.NewModuleEventsList)
	moduleContentBy[event.ModuleTypeEventsList] = content.ModuleContentByPagination
	addToRegistry(event.ModuleTypeUpcomingEvents, "event", event.NewModuleUpcomingEvents)
	moduleContentBy[event.ModuleTypeUpcomingEvents] = content.ModuleContentByPagination

	singularToPlural["sermon"] = "sermons"
	addToRegistry(sermon.ModuleTypeRecentSermons, "sermon", sermon.NewModuleRecentSermons)
	moduleContentBy[sermon.ModuleTypeRecentSermons] = content.ModuleContentByPagination
	addToRegistry(sermon.ModuleTypeSermonForm, "sermon", sermon.NewModuleSermonForm)
	moduleContentBy[sermon.ModuleTypeSermonForm] = content.ModuleContentByForm
	addToRegistry(sermon.ModuleTypeSermonsList, "sermon", sermon.NewModuleSermonsList)
	moduleContentBy[sermon.ModuleTypeRecentSermons] = content.ModuleContentByPagination
	addToRegistry(sermon.ModuleTypeSingleSermon, "sermon", sermon.NewModuleSingleSermon)
	moduleContentBy[sermon.ModuleTypeSingleSermon] = content.ModuleContentBySingleId

	singularToPlural["page"] = "pages"
	addToRegistry(ModuleTypeLoginForm, "", NewModuleLoginForm)
	addToRegistry(ModuleTypePagesList, "page", NewModulePagesList)
	moduleContentBy[ModuleTypePagesList] = content.ModuleContentByPagination
	addToRegistry(ModuleTypePageForm, "page", NewModulePageForm)
	moduleContentBy[ModuleTypePageForm] = content.ModuleContentByForm

	singularToPlural["user"] = "users"
	addToRegistry(user.ModuleTypeUsersList, "user", user.NewModuleUsersList)
	moduleContentBy[user.ModuleTypeUsersList] = content.ModuleContentByPagination
	addToRegistry(user.ModuleTypeUserForm, "user", user.NewModuleUserForm)
	moduleContentBy[user.ModuleTypeUserForm] = content.ModuleContentByForm

	singularToPlural["menu"] = "menus"
	addToRegistry(menu.ModuleTypeMenusList, "menu", menu.NewModuleMenusList)
	moduleContentBy[menu.ModuleTypeMenusList] = content.ModuleContentByPagination
	addToRegistry(menu.ModuleTypeMenuForm, "menu", menu.NewModuleMenuForm)
	moduleContentBy[menu.ModuleTypeMenuForm] = content.ModuleContentByForm

	//singularToPlural["calendar"] = "calendars"
	addToRegistry(calendar.ModuleTypeFullCalendar, "calendar", calendar.NewModuleFullCalendar)

	addToRegistry(slick_carousel.ModuleTypeSlickCarousel, "carousel", slick_carousel.NewModuleSlickCarousel)
	moduleContentBy[slick_carousel.ModuleTypeSlickCarousel] = content.ModuleContentByMultiId
}

// List Available Modules for dynamic pages
func availableModuleTypes() (types []string) {
	for k := range moduleTypeToName {
		lwrModType := strings.ToLower(k)
		if strings.Contains(lwrModType, "form") ||
			strings.Contains(lwrModType, "user") ||
			strings.Contains(lwrModType, "page") ||
			strings.Contains(lwrModType, "menu") {
			continue
		}
		types = append(types, k)
		sort.Strings(types)
	}

	return
}

func addToRegistry(moduleType, singularName string, fun func(module.Presenter) (module.Module, error)) {
	moduleTypeToName[moduleType] = module.Name{Singular: singularName, Plural: singularToPlural[singularName]}
	modulesRegistry[moduleType] = fun
}
