package menu

import (
	"net/url"
	"strings"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/resource/authz"
)

// Nav filtering for admin links.
//
// Menus are data: a site's admin submenu is stored in the database and can
// link to any /admin URL. Before this, every signed-in user saw every link,
// so an Editor holding only articles.* was offered Users, Roles and Giving,
// each of which bounced them to the dashboard with a warning. Each link is
// now shown only if the viewer holds the permission its route requires.
//
// Like the dashboard cards (page/admin_home.go), this is a convenience, not
// the access control: every admin route is wrapped in auth_controller.Require
// in router_rweb.go, and that is what refuses a request. The mapping below
// mirrors that router by convention and has to be kept in step by hand. If it
// drifts, the nav shows a dead link or hides a live one. It never grants
// anything.
//
// URL → permission, by path segment under /admin:
//
//	/admin, /admin/home, /admin/logout  → admin access (any admin permission)
//	/admin/<res>                        → <res>.read
//	/admin/<res>/new                    → <res>.create
//	/admin/<res>/edit/:id               → <res>.update
//	/admin/pages/:id                    → pages.read     (preview)
//	/admin/sermons/import               → sermons.create
//	/admin/sermons/cleanup              → sermons.update
//	/admin/giving[/csv[/summary]]       → charges.read
//	/admin/<unknown>                    → admin access
//	/debug/...                          → SuperAdmin
//	anything else (public, external)    → always shown

// adminResourcePerms maps the first path segment under /admin to its catalog
// resource prefix. Giving is the one resource whose URL and permission names
// differ.
var adminResourcePerms = map[string]string{
	"pages":    "pages",
	"menus":    "menus",
	"articles": "articles",
	"sermons":  "sermons",
	"events":   "events",
	"users":    "users",
	"roles":    "roles",
	"giving":   "charges",
}

// linkPermitted reports whether a nav link should be shown to actor. A nil
// actor (anonymous, or a lookup that failed) sees no admin links.
func linkPermitted(actor *authz.Actor, rawURL string) bool {
	path, ok := localPath(rawURL)
	if !ok {
		return true // external or unparseable: not an admin route
	}

	if path == "/debug" || strings.HasPrefix(path, "/debug/") {
		return actor.IsSuper()
	}

	prefix := config.AdminPrefix
	if path != prefix && !strings.HasPrefix(path, prefix+"/") {
		return true // public route
	}

	// Split what follows the prefix: "/articles/edit/12" → [articles edit 12]
	segs := strings.FieldsFunc(strings.TrimPrefix(path, prefix), func(r rune) bool { return r == '/' })
	if len(segs) == 0 || segs[0] == "home" || segs[0] == "logout" {
		return actor.HasAdminAccess()
	}

	res, known := adminResourcePerms[segs[0]]
	if !known {
		// A route a site added, or one this table doesn't know yet. Hiding
		// it could strand a real screen, so any admin sees it and the route's
		// own guard decides.
		return actor.HasAdminAccess()
	}

	action := authz.ActRead
	if len(segs) > 1 {
		switch segs[1] {
		case "new":
			action = authz.ActCreate
		case "edit":
			action = authz.ActUpdate
		case "import":
			if res == "sermons" {
				action = authz.ActCreate
			}
		case "cleanup":
			if res == "sermons" {
				action = authz.ActUpdate
			}
		}
		// Everything else ("/pages/:id" preview, "/giving/csv") is a read.
	}
	return actor.Can(authz.Permission(res + "." + action))
}

// localPath returns the cleaned path of a same-site link. ok is false for a
// link to another host, or one that won't parse.
//
// Only the path decides access, so the query and fragment are dropped and a
// trailing slash is trimmed ("/admin/users/" is the users list). An absolute
// URL with a host is treated as external even if it names this site: the
// host isn't known at render time, and admin links in menus are relative.
func localPath(rawURL string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host != "" || u.Scheme != "" {
		return "", false
	}
	p := u.Path
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return strings.ToLower(p), true
}
