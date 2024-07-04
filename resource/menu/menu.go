package menu

import (
	"strings"

	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// Menu's are instantiated and rendered off of this
type Menu struct {
	Title          string // label in parent menu
	Slug           string // guid - randomized from title
	Published      bool
	IsAdmin        bool
	Items          []MenuItem
	ActiveItemSlug string // Slug of the current item
}

type MenuItem struct {
	TopLevelItem *MenuItem // so we can indicate current menu item on menu bar
	Label        string
	Url          string
	SubMenu      *Menu
	ParentMenu   *Menu
}

// Menus are built from the top down
// For a given menu definition we get its itemDefinitions / presenters
// And render recursively
func RenderNav(slug string, loggedIn bool) string {
	b := element.NewBuilder()
	b.Ele("nav", "id", slug).R(buildMenu(slug, loggedIn))
	return b.String()
}

func buildMenu(slug string, loggedIn bool) string {
	menuDef, err := menuDefFromSlug(slug)
	if err != nil {
		ser := serr.Wrap(err, "Error obtaining menu def by slug")
		logger.LogErr(ser, "Error building menu from slug", "slug", slug)
		return ""
	}

	sb := &strings.Builder{}
	w := sb.WriteString

	w("<ul>")
	// logger.LogAsync("Debug", "In buildMenu", "Menu definition", fmt.Sprintf("%#v\n", menuDef))

	currentPage := "abc" // todo - set this in the menu edit interface

	for _, item := range menuDef.Items {
		if strings.TrimSpace(item.SubMenuSlug) != "" { // we have a submenu specified
			submenuDef, err := menuDefFromSlug(item.SubMenuSlug)
			if err != nil {
				logger.LogErr(err, "Could not obtain a menu def from slug", "slug",
					item.SubMenuSlug)
			}
			if !loggedIn && submenuDef.IsAdmin {
				continue
			} // authr

			if strings.ToLower(item.Label) == currentPage {
				w(`<li class="menuitem-active">`)
			} else {
				w(`<li>`)
			}

			w(`<a href="#">`)
			w(item.Label)
			w(`</a>`)
			w(buildMenu(item.SubMenuSlug, loggedIn))
		} else {
			if strings.ToLower(item.Label) == currentPage {
				w(`<li class="menuitem-active">`)
			} else {
				w(`<li>`)
			}
			w(`<a href="`)
			w(item.Url)
			w(`">`)
			w(item.Label)
			w(`</a>`)
		}

		w(`</li>`)
	}

	e := element.New

	if slug == "footer-menu" {
		if loggedIn {
			e(sb, "li").R(
				e(sb, "a", "href", "/logout").R("Logout"),
			)
		} else {
			e(sb, "li").R(
				e(sb, "a", "href", "/login").R("Login"),
			)
		}
	}

	w("</ul>")

	return sb.String()
}

// Menus are built from the top down
// For a given menu definition we get its itemDefinitions / presenters
// We instantiate the renderable menu obj
// We then instantiate menu items and add them to the menu,
// after instantiating and linking any submenus to the menuitem
// So this function is called recursively when building complex menus
// func PopulateMenu(slug string) *Menu {
//	menuDef, err := menuDefFromSlug(slug)
//	if err != nil {
//		logger.LogErr(serr.Wrap(err, "When populating menu"))
//	}
//	aMenu := &Menu{}
//	var subMenu *Menu
//	for _, item := range menuDef.Items {
//		subMenu = nil
//		if item.SubMenuSlug != "" {
//			subMenu = PopulateMenu(item.SubMenuSlug)
//		}
//		menuItem := MenuItem{
//			Label: item.Label,
//			Url: item.Url,
//		}
//		if subMenu != nil {
//			menuItem.SubMenu = subMenu
//		}
//		menuItem.ParentMenu = aMenu  // track the items parent too
//		aMenu.Items = append(aMenu.Items, menuItem)
//	}
//
//	return aMenu
// }
