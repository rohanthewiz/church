package menu

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/church/util/stringops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// MenuDef is an interim struct that sits between the form layer and the
// database row. MenuItemDef slices round-trip through the DB as JSONB —
// the conversion happens in menuDefFromModel / modelFromMenuDef below.
type MenuDef struct {
	Id        string
	CreatedAt string
	UpdatedAt string
	UpdatedBy string
	Title     string // label in parent menu
	Slug      string // guid - derived from title
	Published bool
	IsAdmin   bool
	Items     []MenuItemDef
}

// MenuItemDef is the shape of each element inside the Items JSONB blob.
// These tags determine the on-disk JSON field names; renaming them would
// silently invalidate existing rows, so they are kept stable.
type MenuItemDef struct {
	Label       string `json:"label"`
	Url         string `json:"url"`
	SubMenuSlug string `json:"sub_menu_slug"`
}

func (m *MenuDef) CreateSlug() {
	if m.Title == "" {
		logger.Log("Warn", "Title should be set before Slug")
		return
	}
	m.Slug = stringops.SlugWithRandomString(m.Title)
}

func menuDefFromSlug(slug string) (pres MenuDef, err error) {
	m, err := findModelBySlug(slug)
	if err != nil {
		// Fall back to hardwired menu definitions so the site is usable
		// before any menus have been created in the database.
		if def, ok := hardwiredMenuDef(slug); ok {
			logger.Log("Info", "Menu not found in DB, using hardwired fallback", "slug", slug)
			return def, nil
		}
		return pres, serr.Wrap(err, "Error finding menuDef by slug")
	}
	pres, err = menuDefFromModel(m)
	if err != nil {
		return pres, serr.Wrap(err, "Error in menuDef from model")
	}
	return
}

// hardwiredMenuDef returns a default menu definition for known slugs.
// This keeps the site functional when the database has no menu entries yet.
func hardwiredMenuDef(slug string) (MenuDef, bool) {
	switch slug {
	case "main-menu":
		return MenuDef{
			Title:     "Main Menu",
			Slug:      "main-menu",
			Published: true,
			Items: []MenuItemDef{
				{Label: "Home", Url: "/"},
				{Label: "Articles", Url: "/pages/articles"},
				{Label: "Sermons", Url: "/pages/sermons"},
				{Label: "Events", Url: "/pages/events"},
				{Label: "Calendar", Url: "/calendar"},
				// Admin dropdown — only shown when logged in because
				// the submenu has IsAdmin: true (see buildMenu filtering).
				{Label: "Admin", SubMenuSlug: "admin-submenu"},
			},
		}, true
	case "admin-submenu":
		return MenuDef{
			Title:     "Admin Submenu",
			Slug:      "admin-submenu",
			Published: true,
			IsAdmin:   true,
			Items: []MenuItemDef{
				{Label: "Dashboard", Url: "/admin/home"},
				{Label: "Users", Url: "/admin/users"},
				{Label: "Articles", Url: "/admin/articles"},
				{Label: "Pages", Url: "/admin/pages"},
				{Label: "Menus", Url: "/admin/menus"},
				{Label: "Logout", Url: "/admin/logout"},
			},
		}, true
	case "footer-menu":
		// The footer-menu buildMenu logic already appends Login/Logout,
		// so we only need static items here.
		return MenuDef{
			Title:     "Footer Menu",
			Slug:      "footer-menu",
			Published: true,
			Items: []MenuItemDef{
				{Label: "Home", Url: "/"},
			},
		}, true
	default:
		return MenuDef{}, false
	}
}

func menuDefFromModel(m *model.MenuDef) (pres MenuDef, err error) {
	pres.Id = fmt.Sprintf("%d", m.ID)
	if m.CreatedAt.Valid {
		pres.CreatedAt = m.CreatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	if m.UpdatedAt.Valid {
		pres.UpdatedAt = m.UpdatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	pres.UpdatedBy = m.UpdatedBy
	pres.Published = m.Published
	pres.IsAdmin = m.IsAdmin
	pres.Title = m.Title
	pres.Slug = m.Slug

	// Items is jsonb; nil when NULL. An empty/NULL column becomes an empty
	// slice so downstream rendering code can range without a nil guard.
	if len(m.Items) > 0 {
		if err = json.Unmarshal(m.Items, &pres.Items); err != nil {
			return pres, serr.Wrap(err, "Error unmarshalling menu items")
		}
	}
	if pres.Items == nil {
		pres.Items = []MenuItemDef{}
	}
	return
}

func modelFromMenuDef(pres MenuDef) (m *model.MenuDef, create_op bool, err error) {
	m = findModelByIdOrCreate(pres.Id)
	if m.ID < 1 {
		create_op = true
	}

	if updatedBy := strings.TrimSpace(pres.UpdatedBy); updatedBy != "" {
		m.UpdatedBy = updatedBy
	}

	if title := strings.TrimSpace(pres.Title); title != "" {
		m.Title = title
	} else {
		return nil, create_op, serr.Wrap(errors.New("Menu title should not be blank"))
	}

	// Slug is write-once on create to preserve external references.
	if create_op {
		pres.CreateSlug()
		m.Slug = pres.Slug
	}
	m.Published = pres.Published
	m.IsAdmin = pres.IsAdmin

	itemsAsJsonBytes, err := json.Marshal(pres.Items)
	if err != nil {
		return nil, create_op, serr.Wrap(err, "Error marshalling menuDef items")
	}
	m.Items = itemsAsJsonBytes
	return
}
