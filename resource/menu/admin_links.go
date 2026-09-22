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
// in router_rweb.go, and that is what refuses a request. The permission each
// link needs comes from authz.AdminRoutes, the same table the router takes
// its guards from, so the nav cannot drift from the routes. It never grants
// anything.
//
// URL → what the viewer needs:
//
//	/debug/...                          → SuperAdmin
//	/admin (bare prefix)                → admin access (any admin permission)
//	/admin/<path> matching a GET route  → that route's permission, from
//	                                      authz.AdminRoutes (e.g.
//	                                      /admin/articles/edit/5 → articles.update;
//	                                      /admin/home → admin access)
//	/admin/<unknown>                    → admin access
//	anything else (public, external)    → always shown

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

	perm, known := authz.AdminURLPerm(strings.TrimPrefix(path, prefix))
	if !known {
		// The bare prefix, a route a site added, or one the table doesn't
		// know. Hiding it could strand a real screen, so any admin sees it
		// and the route's own guard decides.
		return actor.HasAdminAccess()
	}
	if perm == authz.AdminAccess {
		return actor.HasAdminAccess()
	}
	return actor.Can(perm)
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
