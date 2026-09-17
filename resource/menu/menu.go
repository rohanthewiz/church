package menu

import (
	"strings"

	theDB "github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/authz"
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
//
// glob is the page's params["_global"]. It carries the viewer's username and,
// on admin pages, their encoded permissions (authz.ParamKey), which decide
// which admin links the nav offers (see admin_links.go). It may be nil.
func RenderNav(slug string, loggedIn bool, glob map[string]string) string {
	// RenderNav is invoked from the page template, which has no DB context, so
	// this is the boundary where the handle is fetched. A nil executor is a
	// legitimate state here (fresh install, DB down): menuDefFromSlug then
	// falls back to the hardwired menu definitions so the site stays usable.
	var exec theDB.Executor
	if dbH, err := theDB.Db(); err != nil {
		logger.LogErr(err, "Could not obtain DB handle for menu render; using hardwired menus", "slug", slug)
	} else {
		exec = dbH
	}
	nr := &navRender{exec: exec, loggedIn: loggedIn, glob: glob}
	html, _, _ := nr.buildMenu(slug)

	b := element.NewBuilder()
	b.Nav("id", slug).T(html)
	return b.String()
}

// navRender holds what one nav render needs across its recursive submenu
// calls: the DB handle, the viewer, and the viewer's actor once resolved.
type navRender struct {
	exec     theDB.Executor
	loggedIn bool
	glob     map[string]string

	actor         *authz.Actor
	actorResolved bool
}

// viewer returns the actor whose permissions filter admin links. It is
// resolved lazily, on the first link that needs it, so a nav with no admin
// links (and every anonymous render) costs no query.
//
// Sources, in order:
//  1. Not signed in: nil, which sees no admin links.
//  2. Admin pages: the permissions AdminGuardRWeb already resolved, passed in
//     render params. No query.
//  3. Public pages resolve no actor, so a signed-in viewer's permissions are
//     loaded by username. That is up to one lookup per nav render (main and
//     footer each render separately), for signed-in viewers only.
//
// A failed lookup yields nil: admin links are hidden rather than offered to
// someone whose permissions are unknown. They reappear once the DB is back.
func (nr *navRender) viewer() *authz.Actor {
	if nr.actorResolved {
		return nr.actor
	}
	nr.actorResolved = true

	if !nr.loggedIn {
		return nil
	}
	if nr.glob[authz.ParamKey] != "" {
		nr.actor = authz.FromParams(map[string]map[string]string{"_global": nr.glob})
		return nr.actor
	}
	username := nr.glob["username"]
	if username == "" || nr.exec == nil {
		return nil
	}
	actor, found, err := authz.LoadActor(nr.exec, username)
	if err != nil {
		logger.LogErr(err, "Could not load permissions for nav; hiding admin links", "username", username)
		return nil
	}
	if found {
		nr.actor = actor
	}
	return nr.actor
}

// permitted reports whether a link is shown to the viewer. The actor is only
// resolved for admin links (linkPermitted returns true for public ones before
// using it), so this checks with a nil actor first and resolves only if that
// is refused.
func (nr *navRender) permitted(url string) bool {
	if linkPermitted(nil, url) {
		return true
	}
	return linkPermitted(nr.viewer(), url)
}

// buildMenu renders the menu slug as a <ul>. shown and hidden count the items
// rendered and the items dropped by permission filtering, so a parent can omit
// a submenu that filtering left empty, e.g. the Admin dropdown for a signed-in
// chat member with no admin permissions.
func (nr *navRender) buildMenu(slug string) (html string, shown, hidden int) {
	menuDef, err := menuDefFromSlug(nr.exec, slug)
	if err != nil {
		ser := serr.Wrap(err, "Error obtaining menu def by slug")
		logger.LogErr(ser, "Error building menu from slug", "slug", slug)
		return "", 0, 0
	}

	b := element.NewBuilder()

	b.Ul().R(
		b.Wrap(func() {
			// logger.LogAsync("Debug", "In buildMenu", "Menu definition", fmt.Sprintf("%#v\n", menuDef))

			currentPage := "abc" // todo - set this in the menu edit interface

			for _, item := range menuDef.Items {
				// Attributes rather than a class string, so an inactive item
				// renders a bare <li> as before instead of class="".
				var liAttrs []string
				if strings.ToLower(item.Label) == currentPage {
					liAttrs = []string{"class", "menuitem-active"}
				}

				if strings.TrimSpace(item.SubMenuSlug) != "" { // we have a submenu specified
					submenuDef, err := menuDefFromSlug(nr.exec, item.SubMenuSlug)
					if err != nil {
						logger.LogErr(err, "Could not obtain a menu def from slug", "slug",
							item.SubMenuSlug)
					}
					if !nr.loggedIn && submenuDef.IsAdmin {
						continue
					} // authr

					subHTML, subShown, subHidden := nr.buildMenu(item.SubMenuSlug)
					// Every item filtered away: drop the dropdown rather than
					// show a label that opens nothing. A submenu that was empty
					// to begin with (hidden == 0) renders as before.
					if subShown == 0 && subHidden > 0 {
						hidden++
						continue
					}
					b.Li(liAttrs...).R(
						b.A("href", "#").T(item.Label),
						b.T(subHTML),
					)
					shown++
				} else {
					if !nr.permitted(item.Url) {
						hidden++
						continue
					}
					b.Li(liAttrs...).R(
						b.A("href", item.Url).T(item.Label),
					)
					shown++
				}
			}

			if slug == "footer-menu" {
				if nr.loggedIn {
					b.Li().R(
						b.A("href", "/logout").T("Logout"),
					)
				} else {
					b.Li().R(
						b.A("href", "/login").T("Login"),
					)
				}
			}
		}),
	)

	return b.String(), shown, hidden
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
