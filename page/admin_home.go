package page

import (
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/util/stringops"
	"github.com/rohanthewiz/element"
)

// The admin dashboard: a card per admin area, replacing the old
// "Hello Administrator!" plain-text stub at /admin/home. Cards render off the
// shared admin stylesheet (template/admin_css.go .af-dash*), so this module
// carries no styling of its own and re-skins with the site theme like every
// other admin screen.

type ModuleAdminDashboard struct {
	module.Presenter
}

const ModuleTypeAdminDashboard = "admin_dashboard"

func NewModuleAdminDashboard(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleAdminDashboard)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

// dashCard is one admin area: where it lives, what it is for, and its actions.
type dashCard struct {
	title   string
	desc    string
	listURL string
	newURL  string // empty = no "+ New" action (e.g. utility screens)
}

func (m *ModuleAdminDashboard) Render(params map[string]map[string]string, loggedIn bool) string {
	cards := []dashCard{
		{"Pages", "Build site pages from modules — articles, lists, calendars and more.",
			"/admin/pages", "/admin/pages/new"},
		{"Menus", "Site navigation: the main menu, footer menu and submenus.",
			"/admin/menus", "/admin/menus/new"},
		{"Articles", "Write and publish articles and announcements.",
			"/admin/articles", "/admin/articles/new"},
		{"Sermons", "Manage sermon recordings, scripture references and categories.",
			"/admin/sermons", "/admin/sermons/new"},
		{"Events", "One-time and recurring events; they feed the site calendar.",
			"/admin/events", "/admin/events/new"},
		{"Users", "Admin and editor accounts and their roles.",
			"/admin/users", "/admin/users/new"},
		{"Sermon Import", "Bulk-import sermons from uploaded files.",
			"/admin/sermons/import", ""},
		{"Sermon Cleanup", "Reclaim disk by removing locally-cached sermon copies.",
			"/admin/sermons/cleanup", ""},
	}

	b := element.NewBuilder()

	b.DivClass("af-wrap af-wrap--wide").R(
		b.H3("class", "af-page-title").T("Admin Dashboard"),
		b.DivClass("af-dash").R(
			b.Wrap(func() {
				for _, c := range cards {
					b.AClass("af-dash__card", "href", c.listURL).R(
						b.DivClass("af-dash__title").T(c.title),
						b.DivClass("af-dash__desc").T(c.desc),
						b.Wrap(func() {
							if c.newURL != "" {
								b.DivClass("af-dash__actions").R(
									// Real nested navigation would be an <a> in an <a>
									// (invalid HTML), so the quick action is a button
									// that navigates via JS
									b.Button("type", "button", "class", "af-btn",
										"onclick", "event.preventDefault(); event.stopPropagation(); window.location='"+c.newURL+"';").
										T("+ New"),
								)
							}
						}),
					)
				}
			}),
		),
	)

	return b.String()
}

// AdminHome returns the hardwired admin dashboard page for /admin/home.
func AdminHome() (*Page, error) {
	const title = "Admin Home"
	pgdef := Presenter{
		Title:   title,
		Slug:    stringops.Slugify(title),
		IsAdmin: true,
	}
	modPres := module.Presenter{
		Opts: module.Opts{
			Title:        "Admin Dashboard",
			ModuleType:   ModuleTypeAdminDashboard,
			IsAdmin:      true,
			Published:    true,
			IsMainModule: true,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return pageFromPresenter(pgdef), nil
}
