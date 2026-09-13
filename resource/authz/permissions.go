// Package authz is role-based access control for the admin area.
//
// Model:
//
//	users ──< user_roles >── roles ──< role_permissions
//	                                     (permission = "<resource>.<action>")
//
// A role is any named combination of permissions from the fixed catalog
// below, and a user may hold any number of roles. A user's effective
// permission set is the union over their roles. SuperAdmin (the legacy
// users.role = 99) bypasses the check entirely: it is how a fresh site is
// bootstrapped (see admin.bootstrapSuperAdmin and /super), and it guarantees
// that no role edit can lock every administrator out of the site.
//
// The legacy numeric users.role column is intentionally kept. It is part of
// the mobile app's /auth/me contract (role, role_name), and editor-or-above
// (1–7, 99) still moderates chat and the prayer wall. Moderation can also be
// granted through a role (chat.moderate; see CanModerate in moderation.go).
// Admin screen access no longer derives from it.
//
// The catalog is code, not data: permissions only mean something where a
// handler checks them, so a permission a site could invent in the database
// would be one nothing enforces. Roles (the combinations) are data.
package authz

import (
	"sort"
	"strings"
)

// Permission names one action on one resource, e.g. "articles.publish".
// Stored verbatim in role_permissions.permission, so these strings are a
// persistence format: rename one and existing grants silently stop matching.
type Permission string

// Actions. Publish and Enable are the same idea (make the thing live) under
// the name each resource already uses in its form: content is "published",
// menus historically also "published" but the brief calls it enable, and
// users are "enabled".
const (
	ActCreate   = "create"
	ActRead     = "read"
	ActUpdate   = "update"
	ActDelete   = "delete"
	ActPublish  = "publish"
	ActEnable   = "enable"
	ActModerate = "moderate"
)

// Every permission a handler checks. Kept as named constants so a typo at a
// call site is a compile error rather than a check that can never pass.
const (
	MenusCreate Permission = "menus.create"
	MenusRead   Permission = "menus.read"
	MenusUpdate Permission = "menus.update"
	MenusDelete Permission = "menus.delete"
	MenusEnable Permission = "menus.enable"

	PagesCreate  Permission = "pages.create"
	PagesRead    Permission = "pages.read"
	PagesUpdate  Permission = "pages.update"
	PagesDelete  Permission = "pages.delete"
	PagesPublish Permission = "pages.publish"

	ArticlesCreate  Permission = "articles.create"
	ArticlesRead    Permission = "articles.read"
	ArticlesUpdate  Permission = "articles.update"
	ArticlesDelete  Permission = "articles.delete"
	ArticlesPublish Permission = "articles.publish"

	SermonsCreate  Permission = "sermons.create"
	SermonsRead    Permission = "sermons.read"
	SermonsUpdate  Permission = "sermons.update"
	SermonsDelete  Permission = "sermons.delete"
	SermonsPublish Permission = "sermons.publish"

	EventsCreate  Permission = "events.create"
	EventsRead    Permission = "events.read"
	EventsUpdate  Permission = "events.update"
	EventsDelete  Permission = "events.delete"
	EventsPublish Permission = "events.publish"

	UsersCreate Permission = "users.create"
	UsersRead   Permission = "users.read"
	UsersUpdate Permission = "users.update"
	UsersDelete Permission = "users.delete"
	UsersEnable Permission = "users.enable"

	// Roles are not in the original resource list, but managing them has to
	// be gated by something: without it, anyone who can reach the Role
	// Management screen could write themselves a role holding everything.
	RolesCreate Permission = "roles.create"
	RolesRead   Permission = "roles.read"
	RolesUpdate Permission = "roles.update"
	RolesDelete Permission = "roles.delete"

	// Giving records are written only by Stripe (webhook / receipt), never by
	// an admin, so read is the only action there is to grant.
	ChargesRead Permission = "charges.read"

	// Pin and delete chat messages; mark prayer requests answered or remove
	// them. This is moderation of the public site, not an admin screen, so it
	// does not by itself open the admin area (see Resource.SiteOnly).
	ChatModerate Permission = "chat.moderate"
)

// Resource is one row of the permission matrix on the role form.
type Resource struct {
	Key     string   // permission prefix, e.g. "articles"
	Label   string   // shown in the matrix
	Actions []string // in matrix column order

	// SiteOnly marks permissions used on the public site rather than in the
	// admin area. Holding only these doesn't grant admin access: a chat
	// moderator is a member with extra buttons, not an admin with an empty
	// dashboard.
	SiteOnly bool
}

// Perm builds the permission for one of this resource's actions.
func (r Resource) Perm(action string) Permission {
	return Permission(r.Key + "." + action)
}

// catalog is the permission matrix, in display order. Slices are spelled out
// per resource rather than appended to a shared CRUD slice: append onto a
// shared backing array can let one resource's extra action overwrite
// another's.
var catalog = []Resource{
	{Key: "pages", Label: "Pages", Actions: []string{ActCreate, ActRead, ActUpdate, ActDelete, ActPublish}},
	{Key: "menus", Label: "Menus", Actions: []string{ActCreate, ActRead, ActUpdate, ActDelete, ActEnable}},
	{Key: "articles", Label: "Articles", Actions: []string{ActCreate, ActRead, ActUpdate, ActDelete, ActPublish}},
	{Key: "sermons", Label: "Sermons", Actions: []string{ActCreate, ActRead, ActUpdate, ActDelete, ActPublish}},
	{Key: "events", Label: "Events", Actions: []string{ActCreate, ActRead, ActUpdate, ActDelete, ActPublish}},
	{Key: "users", Label: "Users", Actions: []string{ActCreate, ActRead, ActUpdate, ActDelete, ActEnable}},
	{Key: "roles", Label: "Roles", Actions: []string{ActCreate, ActRead, ActUpdate, ActDelete}},
	{Key: "charges", Label: "Giving (charges)", Actions: []string{ActRead}},
	{Key: "chat", Label: "Chat & prayer wall", Actions: []string{ActModerate}, SiteOnly: true},
}

// validPerms indexes the catalog for O(1) validation of submitted and stored
// permission strings.
var validPerms = func() map[Permission]bool {
	m := map[Permission]bool{}
	for _, r := range catalog {
		for _, a := range r.Actions {
			m[r.Perm(a)] = true
		}
	}
	return m
}()

// siteOnlyPerms indexes the permissions of SiteOnly resources, which
// Actor.HasAdminAccess ignores.
var siteOnlyPerms = func() map[Permission]bool {
	m := map[Permission]bool{}
	for _, r := range catalog {
		if !r.SiteOnly {
			continue
		}
		for _, a := range r.Actions {
			m[r.Perm(a)] = true
		}
	}
	return m
}()

// Catalog returns the resources in display order. A copy, so a caller can't
// reorder the shared definition.
func Catalog() []Resource {
	out := make([]Resource, len(catalog))
	copy(out, catalog)
	return out
}

// AllPermissions returns every catalog permission.
func AllPermissions() Set {
	s := Set{}
	for p := range validPerms {
		s[p] = struct{}{}
	}
	return s
}

// Valid reports whether p is in the catalog.
func Valid(p Permission) bool { return validPerms[p] }

// Normalize drops anything not in the catalog and adds <resource>.read
// wherever the set holds any other action on that resource.
//
// The read implication exists because read is what opens the resource's list
// screen, and every other action is reached from that list: a role with
// "articles.update" but not "articles.read" could edit an article only by
// typing its URL. Rather than let an admin build a role that is useless in
// that confusing way, saving the role fills in the read.
func Normalize(in Set) Set {
	out := Set{}
	for p := range in {
		if !Valid(p) {
			continue
		}
		out[p] = struct{}{}
		if res, _, ok := strings.Cut(string(p), "."); ok {
			read := Permission(res + "." + ActRead)
			if Valid(read) {
				out[read] = struct{}{}
			}
		}
	}
	return out
}

// Set is a permission set. A map-of-empty-struct rather than a slice because
// every use is membership or subset testing.
type Set map[Permission]struct{}

// NewSet builds a set from permissions.
func NewSet(perms ...Permission) Set {
	s := Set{}
	for _, p := range perms {
		s[p] = struct{}{}
	}
	return s
}

// Has reports membership.
func (s Set) Has(p Permission) bool {
	_, ok := s[p]
	return ok
}

// SubsetOf reports whether every permission in s is also in other.
func (s Set) SubsetOf(other Set) bool {
	for p := range s {
		if !other.Has(p) {
			return false
		}
	}
	return true
}

// Sorted returns the permissions in lexical order, for stable output.
func (s Set) Sorted() []Permission {
	out := make([]Permission, 0, len(s))
	for p := range s {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
