package admin

import (
	"encoding/json"
	"os"
	"slices"
	"strings"

	"github.com/rohanthewiz/church/config"
	theDB "github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/article"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/resource/calendar"
	"github.com/rohanthewiz/church/resource/content"
	"github.com/rohanthewiz/church/resource/event"
	"github.com/rohanthewiz/church/resource/sermon"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	"github.com/vattle/sqlboiler/queries/qm"
	"gopkg.in/nullbio/null.v6"
)

// bootstrapMenuItem mirrors menu.MenuItemDef for JSON serialization.
// Defined locally to avoid import coupling between admin and menu packages.
type bootstrapMenuItem struct {
	Label       string `json:"label"`
	Url         string `json:"url"`
	SubMenuSlug string `json:"sub_menu_slug"`
}

// Bootstrap seeds the database with essential resources (superadmin, menus,
// home page, welcome article) so a fresh install is immediately usable.
// Every step is idempotent — existing resources are never overwritten.
func Bootstrap() {
	bootstrapSuperAdmin()
	bootstrapRoles()
	bootstrapMenus()
	bootstrapHomePage()
	bootstrapCalendarPage()
	bootstrapWelcomeArticle()
}

// bootstrapRoles creates the default roles (Administrator, Publisher, Editor)
// on a site's first boot with roles support, assigning them to existing users
// by legacy role. A failure is logged, not fatal: the site still serves, and
// SuperAdmin (which needs no role) can still reach the admin area. The likely
// cause on Postgres is the roles migration not having been run.
func bootstrapRoles() {
	dbH, err := theDB.Db()
	if err != nil {
		logger.LogErr(err, "Bootstrap: error obtaining DB handle for roles")
		return
	}
	if err := authz.EnsureDefaultRoles(dbH); err != nil {
		logger.LogErr(err, "Bootstrap: could not ensure default roles")
	}
}

// bootstrapSuperAdmin creates a superadmin user from environment variables
// or config if no superadmin exists yet. Returns true if a superadmin was
// created, false if one already exists or credentials were not provided.
func bootstrapSuperAdmin() bool {
	dbH, err := theDB.Db()
	if err != nil {
		logger.LogErr(err, "Bootstrap: error obtaining DB handle")
		return false
	}
	exists, err := user.SuperAdminsExist(dbH)
	if err != nil {
		logger.LogErr(err, "Bootstrap: error checking for superadmin")
		return false
	}
	if exists {
		return false
	}

	// Try config first, then fall back to env vars
	adminUser := strings.TrimSpace(config.Options.Bootstrap.AdminUser)
	adminPass := strings.TrimSpace(config.Options.Bootstrap.AdminPass)

	if adminUser == "" {
		adminUser = strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_USER"))
	}
	if adminPass == "" {
		adminPass = strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_PASS"))
	}

	if adminUser == "" || adminPass == "" {
		logger.Log("Info", "Bootstrap: no admin credentials provided. "+
			"Set BOOTSTRAP_ADMIN_USER and BOOTSTRAP_ADMIN_PASS env vars, "+
			"or use the /super endpoint with the token in token.txt")
		return false
	}

	err = CreateSuperUser(adminUser, adminPass)
	if err != nil {
		logger.LogErr(err, "Bootstrap: failed to create superadmin")
		return false
	}

	logger.Log("Info", "Bootstrap: created superadmin user", "username", adminUser)
	return true
}

// bootstrapMenus creates the three core menus (main-menu, admin-submenu,
// footer-menu) with exact slugs so the navigation system can find them.
// Inserts directly at the model level to bypass random slug generation.
func bootstrapMenus() {
	// Define all menus to bootstrap. Each entry specifies the exact slug,
	// display title, admin visibility, and menu items.
	menus := []struct {
		slug    string
		title   string
		isAdmin bool
		items   []bootstrapMenuItem
	}{
		{
			slug:  "main-menu",
			title: "Main Menu",
			items: []bootstrapMenuItem{
				{Label: "Home", Url: "/"},
				// The public list routes, not /pages/<slug>: bootstrap creates
				// only the "home" page, so /pages/articles etc. would 500 on a
				// fresh install. A site that builds its own pages for these can
				// repoint the links in the menu editor.
				{Label: "Articles", Url: "/articles"},
				{Label: "Sermons", Url: "/sermons"},
				{Label: "Events", Url: "/events"},
				// /calendar itself is the FullCalendar JSON feed, not a page;
				// bootstrapCalendarPage creates the page that renders it.
				{Label: "Calendar", Url: "/pages/calendar"},
				{Label: "Admin", SubMenuSlug: "admin-submenu"},
			},
		},
		{
			slug:    "admin-submenu",
			title:   "Admin Submenu",
			isAdmin: true,
			items: []bootstrapMenuItem{
				{Label: "Dashboard", Url: "/admin/home"},
				{Label: "Pages", Url: "/admin/pages"},
				{Label: "Menus", Url: "/admin/menus"},
				{Label: "Articles", Url: "/admin/articles"},
				{Label: "Sermons", Url: "/admin/sermons"},
				{Label: "Events", Url: "/admin/events"},
				{Label: "Users", Url: "/admin/users"},
				{Label: "Roles", Url: "/admin/roles"},
				{Label: "Giving", Url: "/admin/giving"},
				{Label: "Logout", Url: "/admin/logout"},
			},
		},
		{
			slug:  "footer-menu",
			title: "Footer Menu",
			// Login/Logout is appended dynamically by buildMenu(),
			// so we only need static items here.
			items: []bootstrapMenuItem{
				{Label: "Home", Url: "/"},
			},
		},
	}

	dbH, err := theDB.Db()
	if err != nil {
		logger.LogErr(err, "Bootstrap: cannot get DB handle for menus")
		return
	}

	for _, m := range menus {
		existing, err := models.MenuDefs(dbH, qm.Where("slug = ?", m.slug)).One()
		if err == nil && existing != nil {
			// The menu exists. If it is still exactly as bootstrap left it
			// (updated_by tells us no admin has customized it), refresh its
			// items so nav entries added in newer framework versions (e.g.
			// admin Sermons/Events) appear on existing installs too. A menu
			// touched by any human is never overwritten.
			if existing.UpdatedBy == "bootstrap" {
				itemsJSON, jerr := json.Marshal(m.items)
				if jerr != nil {
					logger.LogErr(serr.Wrap(jerr), "Bootstrap: error marshaling menu items", "slug", m.slug)
					continue
				}
				if !menuItemsEqual(existing.Items.JSON, m.items) {
					existing.Items = null.NewJSON(itemsJSON, true)
					if uerr := existing.Update(dbH); uerr != nil {
						logger.LogErr(serr.Wrap(uerr), "Bootstrap: error refreshing menu items", "slug", m.slug)
					} else {
						logger.Log("Info", "Bootstrap: refreshed uncustomized menu", "slug", m.slug)
					}
				}
			}
			continue
		}

		itemsJSON, err := json.Marshal(m.items)
		if err != nil {
			logger.LogErr(serr.Wrap(err), "Bootstrap: error marshaling menu items", "slug", m.slug)
			continue
		}

		model := &models.MenuDef{
			Title:     m.title,
			Slug:      m.slug,
			Published: true,
			IsAdmin:   m.isAdmin,
			UpdatedBy: "bootstrap",
			Items:     null.NewJSON(itemsJSON, true),
		}

		err = model.Insert(dbH)
		if err != nil {
			logger.LogErr(serr.Wrap(err), "Bootstrap: error inserting menu", "slug", m.slug)
			continue
		}
		logger.Log("Info", "Bootstrap: created menu", "slug", m.slug)
	}
}

// bootstrapHomePage creates the home page with slug "home" exactly, containing
// modules for recent sermons, upcoming events, and a blog articles section.
// Inserts directly at the model level to set the exact slug.
func bootstrapHomePage() {
	dbH, err := theDB.Db()
	if err != nil {
		logger.LogErr(err, "Bootstrap: cannot get DB handle for home page")
		return
	}

	exists, err := models.Pages(dbH, qm.Where("slug = ?", "home")).Exists()
	if err != nil {
		logger.LogErr(serr.Wrap(err), "Bootstrap: error checking home page existence")
		return
	}
	if exists {
		return
	}

	// Module definitions mirror page.Home() in page/homepage.go
	modules := []module.Presenter{
		{
			Opts: module.Opts{
				ModuleType:   sermon.ModuleTypeRecentSermons,
				Title:        "Recent Sermons",
				Published:    true,
				LayoutColumn: "left",
				Limit:        8,
			},
		},
		{
			Opts: module.Opts{
				ModuleType:   event.ModuleTypeUpcomingEvents,
				Title:        "Upcoming Events",
				Published:    true,
				LayoutColumn: "left",
				Limit:        8,
			},
		},
		{
			Opts: module.Opts{
				ModuleType:   article.ModuleTypeArticlesBlog,
				Title:        "Homepage Articles",
				Published:    true,
				IsMainModule: true,
				Limit:        4,
			},
		},
	}

	modulesJSON, err := json.Marshal(modules)
	if err != nil {
		logger.LogErr(serr.Wrap(err), "Bootstrap: error marshaling home page modules")
		return
	}

	model := &models.Page{
		Title:              "Home",
		Slug:               "home",
		Published:          true,
		IsHome:             true,
		UpdatedBy:          "bootstrap",
		AvailablePositions: []string{"left", "center"},
		Data:               null.NewJSON(modulesJSON, true),
	}

	err = model.Insert(dbH)
	if err != nil {
		logger.LogErr(serr.Wrap(err), "Bootstrap: error inserting home page")
		return
	}
	logger.Log("Info", "Bootstrap: created home page")
}

// menuItemsEqual reports whether a menu's stored items JSON decodes to the
// same items bootstrap would write. Comparing raw bytes instead never matches
// on Postgres, because JSONB returns its own key order and spacing, not what
// was written; every boot would then rewrite every uncustomized menu and log
// it as a refresh. Stored JSON that does not decode counts as different, so
// bootstrap rewrites it with good items.
func menuItemsEqual(stored []byte, want []bootstrapMenuItem) bool {
	var got []bootstrapMenuItem
	if err := json.Unmarshal(stored, &got); err != nil {
		return false
	}
	return slices.Equal(got, want)
}

// bootstrapCalendarPage creates the page with slug "calendar" that the default
// main menu's Calendar item links to (/pages/calendar), holding the
// FullCalendar module. Without it the link would fall to the hardwired
// page.Calendar, which works but can't be edited in the page builder. A site
// that already has a "calendar" page keeps it untouched.
func bootstrapCalendarPage() {
	dbH, err := theDB.Db()
	if err != nil {
		logger.LogErr(err, "Bootstrap: cannot get DB handle for calendar page")
		return
	}

	exists, err := models.Pages(dbH, qm.Where("slug = ?", page.CalendarSlug)).Exists()
	if err != nil {
		logger.LogErr(serr.Wrap(err), "Bootstrap: error checking calendar page existence")
		return
	}
	if exists {
		return
	}

	// Module definition mirrors page.Calendar() in page/calendar_page.go
	modules := []module.Presenter{
		{
			Opts: module.Opts{
				ModuleType:   calendar.ModuleTypeFullCalendar,
				Title:        "Calendar",
				Published:    true,
				IsMainModule: true,
				LayoutColumn: "center",
			},
		},
	}

	modulesJSON, err := json.Marshal(modules)
	if err != nil {
		logger.LogErr(serr.Wrap(err), "Bootstrap: error marshaling calendar page modules")
		return
	}

	model := &models.Page{
		Title:              "Calendar",
		Slug:               page.CalendarSlug,
		Published:          true,
		UpdatedBy:          "bootstrap",
		AvailablePositions: []string{"center"},
		Data:               null.NewJSON(modulesJSON, true),
	}

	if err = model.Insert(dbH); err != nil {
		logger.LogErr(serr.Wrap(err), "Bootstrap: error inserting calendar page")
		return
	}
	logger.Log("Info", "Bootstrap: created calendar page")
}

// bootstrapWelcomeArticle creates an initial article so the home page has
// content to display. Only runs if no articles exist in the database.
func bootstrapWelcomeArticle() {
	dbH, err := theDB.Db()
	if err != nil {
		logger.LogErr(err, "Bootstrap: cannot get DB handle for welcome article")
		return
	}

	exists, err := models.Articles(dbH).Exists()
	if err != nil {
		logger.LogErr(serr.Wrap(err), "Bootstrap: error checking for existing articles")
		return
	}
	if exists {
		return
	}

	pres := article.Presenter{
		Content: content.Content{
			Title:   "Welcome to Our Church",
			Summary: "We are glad you are here. Learn more about our community and upcoming activities.",
			Body: `<p>Welcome to our church website! We are a welcoming community dedicated to ` +
				`worship, fellowship, and service.</p>` +
				`<p>Feel free to browse our sermons, upcoming events, and articles. ` +
				`If you have any questions, don't hesitate to reach out.</p>`,
			Published:  true,
			UpdatedBy:  "bootstrap",
			Categories: []string{"general"},
		},
	}

	err = pres.UpsertArticle(dbH)
	if err != nil {
		logger.LogErr(serr.Wrap(err), "Bootstrap: error creating welcome article")
		return
	}
	logger.Log("Info", "Bootstrap: created welcome article")
}
