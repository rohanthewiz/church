package page

import (
	"strings"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/authz"
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
//
// The dashboard offers only what the viewer can use: a card shows if the
// viewer may open its listURL, and its "+ New" action if they may open its
// newURL. The permission for each URL comes from authz.AdminRoutes, the table
// the router takes its guards from, so the cards cannot drift from the
// routes. This filtering is a convenience, not the access control: every
// linked route is wrapped in auth_controller.Require, which is what refuses a
// request.
type dashCard struct {
	title   string
	desc    string
	listURL string
	newURL  string // empty = no "+ New" action (e.g. utility screens)
}

// canOpen reports whether actor may open the admin URL u, by the permission
// of the GET route it matches. A URL the table lacks is refused, so a card
// pointing at a route that doesn't exist stays hidden instead of offering a
// dead link.
func canOpen(actor *authz.Actor, u string) bool {
	perm, ok := authz.AdminURLPerm(strings.TrimPrefix(u, config.AdminPrefix))
	if !ok {
		return false
	}
	if perm == authz.AdminAccess {
		return actor.HasAdminAccess()
	}
	return actor.Can(perm)
}

func (m *ModuleAdminDashboard) Render(params map[string]map[string]string, loggedIn bool) string {
	allCards := []dashCard{
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
		{"Roles", "Define roles from any combination of permissions, then assign them to users.",
			"/admin/roles", "/admin/roles/new"},
		{"Giving", "Giving received through Stripe, by month for each year, with CSV export.",
			"/admin/giving", ""},
		// Utility screens: no list behind them, so the card follows the
		// permission of the screen's own route.
		{"Sermon Import", "Bulk-import sermons from uploaded files.",
			"/admin/sermons/import", ""},
		{"Sermon Cleanup", "Reclaim disk by removing locally-cached sermon copies.",
			"/admin/sermons/cleanup", ""},
	}

	// The viewer's permissions arrive through render params (the module has no
	// request context); see authz.ParamValue.
	actor := authz.FromParams(params)
	var cards []dashCard
	for _, c := range allCards {
		if !canOpen(actor, c.listURL) {
			continue
		}
		if c.newURL != "" && !canOpen(actor, c.newURL) {
			c.newURL = "" // hides "+ New"
		}
		cards = append(cards, c)
	}

	b := element.NewBuilder()

	b.DivClass("af-wrap af-wrap--wide").R(
		b.H3("class", "af-page-title").T("Admin Dashboard"),
		b.Wrap(func() {
			// Reachable only by an admin whose roles open no area on the
			// dashboard, e.g. a role granting only a permission with no screen.
			if len(cards) == 0 {
				b.PClass("af-help").T("Your roles don't include access to any admin area yet. Ask an administrator to assign you a role.")
			}
		}),
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
