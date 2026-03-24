package admin

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/rohanthewiz/church/config"
	theDB "github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/article"
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
	bootstrapMenus()
	bootstrapHomePage()
	bootstrapWelcomeArticle()
}

// bootstrapSuperAdmin creates a superadmin user from environment variables
// or config if no superadmin exists yet. Returns true if a superadmin was
// created, false if one already exists or credentials were not provided.
func bootstrapSuperAdmin() bool {
	exists, err := user.SuperAdminsExist()
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
				{Label: "Articles", Url: "/pages/articles"},
				{Label: "Sermons", Url: "/pages/sermons"},
				{Label: "Events", Url: "/pages/events"},
				{Label: "Calendar", Url: "/calendar"},
				{Label: "Admin", SubMenuSlug: "admin-submenu"},
			},
		},
		{
			slug:    "admin-submenu",
			title:   "Admin Submenu",
			isAdmin: true,
			items: []bootstrapMenuItem{
				{Label: "Dashboard", Url: "/admin/home"},
				{Label: "Users", Url: "/admin/users"},
				{Label: "Articles", Url: "/admin/articles"},
				{Label: "Pages", Url: "/admin/pages"},
				{Label: "Menus", Url: "/admin/menus"},
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
		exists, err := models.MenuDefs(dbH, qm.Where("slug = ?", m.slug)).Exists()
		if err != nil {
			logger.LogErr(serr.Wrap(err), "Bootstrap: error checking menu existence", "slug", m.slug)
			continue
		}
		if exists {
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

	err = pres.UpsertArticle()
	if err != nil {
		logger.LogErr(serr.Wrap(err), "Bootstrap: error creating welcome article")
		return
	}
	logger.Log("Info", "Bootstrap: created welcome article")
}
