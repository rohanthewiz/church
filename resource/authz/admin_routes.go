package authz

import "strings"

// The admin route → permission table.
//
// This is the one place that says which permission each admin route needs.
// Three consumers read it, and before it existed each kept its own hand-synced
// copy:
//
//	                    ┌─► router_rweb.go RegisterAdminRoutes — wraps each
//	                    │   handler in auth_controller.Require(perm); a route
//	                    │   missing from the table panics at startup
//	 AdminRoutes ───────┼─► menu.linkPermitted — shows a nav link only if the
//	 (this file)        │   viewer holds the permission its GET route needs
//	                    └─► page admin dashboard — shows a card and its
//	                        "+ New" action the same way
//
// Only the router's use is access control. The nav and the dashboard use the
// table to avoid offering dead links, so a mistake there shows or hides a link
// but never grants anything. Because all three read the same rows, they can no
// longer drift apart.
//
// Paths are relative to config.AdminPrefix (authz does not import config) and
// use rweb's ":name" syntax for a path parameter. Perm AdminAccess marks a
// route any admin may use.
//
// Convention: list = read, new form + create POST = create, edit form +
// update POST = update, delete POST = delete. Publish/enable are field-level
// and are resolved inside the upsert handlers (ResolveFlag), not here.

// AdminAccess is the Perm of a route open to any admin. It is the empty
// permission, which auth_controller.Require treats as "admin access alone".
const AdminAccess Permission = ""

// AdminRoute is one admin route and the permission its handler requires.
type AdminRoute struct {
	Method string // "GET" or "POST"
	Path   string // relative to config.AdminPrefix, e.g. "/articles/edit/:id"
	Perm   Permission
}

// AdminRoutes lists every permission-guarded admin route. Adding a route to
// RegisterAdminRoutes without a row here fails at startup.
var AdminRoutes = []AdminRoute{
	{"GET", "/home", AdminAccess},
	{"GET", "/logout", AdminAccess},

	{"GET", "/users", UsersRead},
	{"GET", "/users/new", UsersCreate},
	{"POST", "/users", UsersCreate},
	{"GET", "/users/edit/:id", UsersUpdate},
	{"POST", "/users/update/:id", UsersUpdate},
	{"POST", "/users/delete/:id", UsersDelete},

	{"GET", "/roles", RolesRead},
	{"GET", "/roles/new", RolesCreate},
	{"POST", "/roles", RolesCreate},
	{"GET", "/roles/edit/:id", RolesUpdate},
	{"POST", "/roles/update/:id", RolesUpdate},
	{"POST", "/roles/delete/:id", RolesDelete},

	// Giving records are read-only (Stripe writes charges, not admins). The
	// CSV exports expose the same donor data as the page, so they take the
	// same permission.
	{"GET", "/giving", ChargesRead},
	{"GET", "/giving/csv", ChargesRead},
	{"GET", "/giving/csv/summary", ChargesRead},

	{"GET", "/articles", ArticlesRead},
	{"GET", "/articles/new", ArticlesCreate},
	{"POST", "/articles", ArticlesCreate},
	{"GET", "/articles/edit/:id", ArticlesUpdate},
	{"POST", "/articles/update/:id", ArticlesUpdate},
	{"POST", "/articles/delete/:id", ArticlesDelete},

	{"GET", "/sermons", SermonsRead},
	{"GET", "/sermons/new", SermonsCreate},
	{"GET", "/sermons/import", SermonsCreate},
	{"POST", "/sermons/import", SermonsCreate},
	{"POST", "/sermons", SermonsCreate},
	{"GET", "/sermons/edit/:id", SermonsUpdate},
	{"POST", "/sermons/update/:id", SermonsUpdate},
	{"POST", "/sermons/delete/:id", SermonsDelete},
	// The cleanup tool removes local cached copies, never sermons, so it
	// rides update rather than delete.
	{"GET", "/sermons/cleanup", SermonsUpdate},
	{"POST", "/sermons/cleanup", SermonsUpdate},

	{"GET", "/events", EventsRead},
	{"GET", "/events/new", EventsCreate},
	{"POST", "/events", EventsCreate},
	{"GET", "/events/edit/:id", EventsUpdate},
	{"POST", "/events/update/:id", EventsUpdate},
	{"POST", "/events/delete/:id", EventsDelete},

	{"GET", "/pages", PagesRead},
	{"GET", "/pages/new", PagesCreate},
	{"POST", "/pages", PagesCreate},
	{"GET", "/pages/:id", PagesRead}, // preview
	{"GET", "/pages/edit/:id", PagesUpdate},
	{"POST", "/pages/update/:id", PagesUpdate},
	{"POST", "/pages/delete/:id", PagesDelete},

	{"GET", "/menus", MenusRead},
	{"GET", "/menus/new", MenusCreate},
	{"POST", "/menus", MenusCreate},
	{"GET", "/menus/edit/:id", MenusUpdate},
	{"POST", "/menus/update/:id", MenusUpdate},
	{"POST", "/menus/delete/:id", MenusDelete},
}

// AdminRoutePerm returns the permission of the route registered as method +
// path pattern, exactly as written in the table (":id", not a real id). It is
// the router's lookup; ok is false for a route the table lacks.
func AdminRoutePerm(method, pattern string) (perm Permission, ok bool) {
	for _, r := range AdminRoutes {
		if r.Method == method && r.Path == pattern {
			return r.Perm, true
		}
	}
	return "", false
}

// AdminURLPerm returns the permission needed to open a concrete admin URL
// path with GET, e.g. "/articles/edit/12" → articles.update. path is relative
// to config.AdminPrefix, already cleaned (no query, no trailing slash). ok is
// false when no GET route matches, e.g. a route a site added itself.
//
// A literal segment beats a parameter, so "/pages/new" resolves to the
// new-page form (pages.create) rather than the "/pages/:id" preview
// (pages.read). Among matches, the one with the most literal segments wins.
func AdminURLPerm(path string) (perm Permission, ok bool) {
	segs := splitPath(path)
	best := -1 // literal-segment count of the best match so far
	for _, r := range AdminRoutes {
		if r.Method != "GET" {
			continue
		}
		pat := splitPath(r.Path)
		if len(pat) != len(segs) {
			continue
		}
		literals, matched := 0, true
		for i, p := range pat {
			if strings.HasPrefix(p, ":") {
				continue // a parameter matches any one segment
			}
			if p != segs[i] {
				matched = false
				break
			}
			literals++
		}
		if matched && literals > best {
			best, perm, ok = literals, r.Perm, true
		}
	}
	return perm, ok
}

// splitPath splits "/articles/edit/12" into [articles edit 12], ignoring
// empty segments from leading, trailing or doubled slashes.
func splitPath(p string) []string {
	return strings.FieldsFunc(p, func(r rune) bool { return r == '/' })
}
