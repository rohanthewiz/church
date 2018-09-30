package menu

import (
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	"bytes"
	"strings"
	"github.com/rohanthewiz/element"
)

// Menu's are instantiated and rendered off of this
type Menu struct {
	Title string // label in parent menu
	Slug string  // guid - randomized from title
	Published    bool
	IsAdmin bool
	Items []MenuItem
	ActiveItemSlug string  // Slug of the current item
}

type MenuItem struct {
	TopLevelItem *MenuItem // so we can indicate current menu item on menu bar
	Label string
	Url   string
	SubMenu *Menu
	ParentMenu *Menu
}

// Menus are built from the top down
// For a given menu definition we get its itemDefinitions / presenters
// And render recursively
func RenderNav(slug string, loggedIn bool) string {
	return element.New("nav", "id", slug).R(buildMenu(slug, loggedIn))
}

func buildMenu(slug string, loggedIn bool) string {
	out := new(bytes.Buffer)
	menuDef, err := menuDefFromSlug(slug)
	if err != nil {
		ser := serr.Wrap(err, "Error obtaining menu def by slug")
		logger.LogErr(ser, "Error building menu from slug", "slug", slug)
		return ""
	}
	ows := out.WriteString
	e := element.New
	ows("<ul>")
	//logger.LogAsync("Debug", "In buildMenu", "Menu definition", fmt.Sprintf("%#v\n", menuDef))

	currentPage := "abc" // todo - set this in the menu edit interface

	for _, item := range menuDef.Items {
		if strings.TrimSpace(item.SubMenuSlug) != "" { // we have a submenu specified
			submenuDef, err := menuDefFromSlug(item.SubMenuSlug)
			if err != nil { logger.LogErrAsync(err, "Could not obtain a menu def from slug", "slug",
					item.SubMenuSlug)
			}
			if !loggedIn && submenuDef.IsAdmin { continue } // authr

			if strings.ToLower(item.Label) == currentPage {
				ows(`<li class="menuitem-active">`)
			} else {
				ows(`<li>`)
			}

			ows(`<a href="#">`)
			ows(item.Label); ows(`</a>`)
			ows(buildMenu(item.SubMenuSlug, loggedIn))
		} else {
			if strings.ToLower(item.Label) == currentPage {
				ows(`<li class="menuitem-active">`)
			} else {
				ows(`<li>`)
			}
			ows(`<a href="`); ows(item.Url); ows(`">`)
			ows(item.Label); ows(`</a>`)
		}

		ows(`</li>`)
	}
	if slug == "footer-menu" {
		if loggedIn {
			ows(e("li").R(
				e("a", "href", "/logout").R("Logout"),
			))
		} else {
			ows(e("li").R(
				e("a", "href", "/login").R("Login"),
			))
		}
	}

	ows("</ul>")
	return out.String()
}

// Menus are built from the top down
// For a given menu definition we get its itemDefinitions / presenters
// We instantiate the renderable menu obj
// We then instantiate menu items and add them to the menu,
// after instantiating and linking any submenus to the menuitem
// So this function is called recursively when building complex menus
//func PopulateMenu(slug string) *Menu {
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
//}
