package main

// End-to-end check for role-based admin access.
//
// Boots an embedded bytdb exactly as a site binary does (db.InitDB), seeds
// the default roles through the real bootstrap function, and wires the admin
// area with the production RegisterAdminRoutes. It then drives the HTTP
// handlers in-process (rweb Server.Request), so every request passes through
// the real guard, decorators, handlers, page renders and SQL:
//
//	sessions ─► AdminGuardRWeb ─► Require(perm) ─► handler ─► module render ─► bytdb
//
// Sessions are planted straight into the in-process kvstore rather than
// logging in, because login is covered by auth_controller's tests. Outcomes
// are asserted on database state where possible, not on flash text.
//
// Run from the church module root (resource/auth loads cfg/ relative to the
// working directory):  go run ./test_scripts/roles_smoke

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rohanthewiz/church"
	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/rweb"
)

var failures int

// lastBody is the most recent response body, shown when a check fails so a
// render error is visible without a debugger.
var lastBody string

func expect(label string, cond bool, detail string) {
	if !cond {
		failures++
		fmt.Printf("FAIL  %s: %s\n", label, detail)
		if lastBody != "" {
			snip := lastBody
			if len(snip) > 400 {
				snip = snip[:400]
			}
			fmt.Printf("      body: %s\n", snip)
		}
		return
	}
	fmt.Printf("pass  %s\n", label)
}

func must(label string, err error) {
	if err != nil {
		fmt.Printf("FATAL %s: %v\n", label, err)
		os.Exit(1)
	}
}

// signIn plants a session for username and returns its cookie header.
func signIn(username string) []rweb.Header {
	key := auth.RandomKey()
	must("save session", session.Session{Username: username}.Save(key))
	return []rweb.Header{{Key: "Cookie", Value: session.CookieName + "=" + key}}
}

// post submits a urlencoded form with a fresh CSRF token.
func post(s *rweb.Server, who []rweb.Header, path string, vals url.Values) rweb.Response {
	tok, err := app.GenerateFormToken()
	must("csrf token", err)
	vals.Set("csrf", tok)
	hdrs := append([]rweb.Header{{Key: "Content-Type", Value: "application/x-www-form-urlencoded"}}, who...)
	return s.Request("POST", path, hdrs, strings.NewReader(vals.Encode()))
}

func main() {
	tmpDir, err := os.MkdirTemp("", "roles_smoke")
	must("temp dir", err)
	defer os.RemoveAll(tmpDir)

	must("InitDB", db.InitDB(db.DBOpts{DBType: db.DBTypes.BytDB, File: filepath.Join(tmpDir, "church.db")}))
	defer db.CloseDB()
	dbH, err := db.Db()
	must("Db", err)

	// Legacy users: ann=Admin(1), ed=Editor(7), root=SuperAdmin(99),
	// uman=RegisteredUser(9) who will get a users-only role.
	ids := map[string]int64{}
	for _, u := range []struct {
		name string
		role int
	}{{"ann", 1}, {"ed", 7}, {"root", 99}, {"uman", 9}} {
		var id int64
		err := dbH.QueryRow(`INSERT INTO users (updated_by, enabled, role, username, email_address, first_name)
			VALUES ('smoke', true, $1, $2, $3, $2) RETURNING id`, u.role, u.name, u.name+"@smoke.test").Scan(&id)
		must("seed user "+u.name, err)
		ids[u.name] = id
	}
	must("EnsureDefaultRoles", authz.EnsureDefaultRoles(dbH))

	// Site mains load config.Options at startup; the page template reads it
	// (theme), so the harness supplies an empty one.
	config.Options = &config.EnvConfig{}
	page.RegisterModules()
	s := rweb.NewServer(rweb.ServerOptions{})
	church.RegisterAdminRoutes(s)

	root, ann, ed, uman := signIn("root"), signIn("ann"), signIn("ed"), signIn("uman")
	roleID := func(name string) int64 {
		roles, err := authz.ListRoles(dbH)
		must("ListRoles", err)
		for _, r := range roles {
			if r.Name == name {
				return r.ID
			}
		}
		return 0
	}
	actorCan := func(username string, p authz.Permission) bool {
		a, found, err := authz.LoadActor(dbH, username)
		must("LoadActor "+username, err)
		return found && a.Can(p)
	}

	// ---- Role Management screens ----
	r := s.Request("GET", "/admin/home", root, nil)
	body := string(r.Body())
	expect("superadmin dashboard renders Roles and Giving cards", r.Status() == 200 &&
		strings.Contains(body, "/admin/roles") && strings.Contains(body, "/admin/giving"),
		fmt.Sprintf("status %d", r.Status()))

	r = s.Request("GET", "/admin/roles", root, nil)
	lastBody = string(r.Body())
	body = string(r.Body())
	expect("roles list shows the three default roles", r.Status() == 200 &&
		strings.Contains(body, "Administrator") && strings.Contains(body, "Publisher") && strings.Contains(body, "Editor"),
		fmt.Sprintf("status %d", r.Status()))

	r = s.Request("GET", "/admin/roles/new", root, nil)
	lastBody = string(r.Body())
	expect("role form renders the permission matrix", r.Status() == 200 &&
		strings.Contains(string(r.Body()), `name="perm:articles.publish"`) &&
		strings.Contains(string(r.Body()), `name="perm:charges.read"`),
		fmt.Sprintf("status %d", r.Status()))

	r = post(s, root, "/admin/roles", url.Values{
		"role_id": {"0"}, "role_name": {"Greeter"}, "role_description": {"Front door"},
		"perm:events.publish": {"on"},
		"perm:charges.create": {"on"}, // not in the catalog: must be ignored
	})
	greeter := roleID("Greeter")
	g, _, _ := authz.GetRole(dbH, greeter)
	expect("create role from a permission combination", r.Status() == 303 && greeter != 0 &&
		len(g.Perms) == 2 && g.Perms.Has(authz.EventsPublish) && g.Perms.Has(authz.EventsRead),
		fmt.Sprintf("status %d, perms %v", r.Status(), g.Perms.Sorted()))

	r = post(s, root, "/admin/roles", url.Values{"role_id": {"0"}, "role_name": {"User Manager"},
		"perm:users.create": {"on"}, "perm:users.update": {"on"}, "perm:users.delete": {"on"}, "perm:users.enable": {"on"}})
	userMgr := roleID("User Manager")
	expect("create a users-only role", userMgr != 0, fmt.Sprintf("status %d", r.Status()))

	// ---- Multiple roles on the Users form ----
	r = s.Request("GET", "/admin/users/edit/"+strconv.FormatInt(ids["ed"], 10), root, nil)
	lastBody = string(r.Body())
	expect("user form lists roles as checkboxes", r.Status() == 200 &&
		strings.Contains(string(r.Body()), `name="role:`+strconv.FormatInt(greeter, 10)+`"`),
		fmt.Sprintf("status %d", r.Status()))

	r = post(s, root, "/admin/users/update/"+strconv.FormatInt(ids["ed"], 10), url.Values{
		"user_id": {strconv.FormatInt(ids["ed"], 10)}, "username": {"ed"}, "email_address": {"ed@smoke.test"},
		"firstname": {"Ed"}, "role": {"7"}, "enabled": {"on"},
		"role:" + strconv.FormatInt(roleID("Editor"), 10): {"on"},
		"role:" + strconv.FormatInt(greeter, 10):          {"on"},
	})
	expect("assign two roles to one user (union applies)", r.Status() == 303 &&
		actorCan("ed", authz.EventsPublish) && actorCan("ed", authz.ArticlesUpdate),
		fmt.Sprintf("status %d loc %q", r.Status(), r.Header("Location")))

	assignUman := post(s, root, "/admin/users/update/"+strconv.FormatInt(ids["uman"], 10), url.Values{
		"user_id": {strconv.FormatInt(ids["uman"], 10)}, "username": {"uman"}, "email_address": {"uman@smoke.test"},
		"firstname": {"Uma"}, "role": {"9"}, "enabled": {"on"},
		"role:" + strconv.FormatInt(userMgr, 10): {"on"},
	})
	expect("assign User Manager to uman", assignUman.Status() == 303 && actorCan("uman", authz.UsersUpdate), "")

	// ---- Route enforcement ----
	r = s.Request("GET", "/admin/articles", uman, nil)
	lastBody = string(r.Body())
	expect("uman (users only) is refused articles", r.Status() == 303 && r.Header("Location") == "/admin/home" &&
		!strings.Contains(string(r.Body()), "ch-module-wrapper"), fmt.Sprintf("status %d loc %q", r.Status(), r.Header("Location")))
	r = s.Request("GET", "/admin/home", uman, nil)
	lastBody = string(r.Body())
	// Only the dashboard cards are asserted on. The site nav's Admin submenu
	// is menu content and still lists every admin link (each one refused by
	// its route guard), so the whole page can't be used here.
	body = string(r.Body())
	if i := strings.Index(body, `class="af-dash"`); i >= 0 {
		body = body[i:]
	}
	expect("uman's dashboard shows Users, hides Articles and Roles", r.Status() == 200 &&
		strings.Contains(body, `href="/admin/users"`) && !strings.Contains(body, `href="/admin/articles"`) &&
		!strings.Contains(body, `href="/admin/roles"`), fmt.Sprintf("status %d users=%v articles=%v roles=%v", r.Status(),
		strings.Contains(body, `href="/admin/users"`), strings.Contains(body, `href="/admin/articles"`),
		strings.Contains(body, `href="/admin/roles"`)))
	r = s.Request("GET", "/admin/giving", ann, nil)
	lastBody = string(r.Body())
	expect("Administrator reads giving records", r.Status() == 200 &&
		strings.Contains(string(r.Body()), "Giving Records"), fmt.Sprintf("status %d", r.Status()))

	// ---- Giving by month, year navigation, CSV export ----
	now := time.Now()
	for i, g := range []struct {
		at    time.Time
		name  string
		cents int64
	}{
		{time.Date(now.Year(), now.Month(), 1, 10, 0, 0, 0, time.Local), "<b>Kim</b>", 250000},
		{time.Date(now.Year()-1, time.March, 5, 10, 0, 0, 0, time.Local), "Lee", 1000},
	} {
		_, err := dbH.Exec(`INSERT INTO charges (created_at, customer_name, payment_token, paid, amount_paid)
			VALUES ($1, $2, $3, true, $4)`, g.at, g.name, "smoke_tok_"+strconv.Itoa(i), g.cents)
		must("seed charge", err)
	}
	thisMonth := now.Format("January 2006")
	r = s.Request("GET", "/admin/giving", ann, nil)
	lastBody = string(r.Body())
	body = string(r.Body())
	expect("giving defaults to YTD grouped by month with a link back one year", r.Status() == 200 &&
		strings.Contains(body, "Year to date") && strings.Contains(body, thisMonth) &&
		strings.Contains(body, "$2,500.00") && strings.Contains(body, fmt.Sprintf("?year=%d", now.Year()-1)),
		fmt.Sprintf("status %d", r.Status()))
	expect("giving escapes donor-supplied names", !strings.Contains(body, "<b>Kim</b>") &&
		strings.Contains(body, "&lt;b&gt;Kim&lt;/b&gt;"), "raw donor markup reached the page")

	r = s.Request("GET", fmt.Sprintf("/admin/giving?year=%d", now.Year()-1), ann, nil)
	lastBody = string(r.Body())
	body = string(r.Body())
	expect("previous year is the earliest: no further back link, a forward link", r.Status() == 200 &&
		strings.Contains(body, "Full year") && strings.Contains(body, "$10.00") &&
		!strings.Contains(body, fmt.Sprintf("?year=%d", now.Year()-2)) &&
		strings.Contains(body, fmt.Sprintf("?year=%d", now.Year())), fmt.Sprintf("status %d", r.Status()))

	r = s.Request("GET", "/admin/giving/csv", ann, nil)
	lastBody = string(r.Body())
	body = string(r.Body())
	expect("CSV export downloads with headings", r.Status() == 200 &&
		strings.HasPrefix(r.Header("Content-Type"), "text/csv") &&
		strings.Contains(r.Header("Content-Disposition"), fmt.Sprintf("giving-%d-ytd.csv", now.Year())) &&
		strings.Contains(body, "Date,Month,Name,Email,Amount,Refunded,Net,Status") &&
		strings.Contains(body, "2500.00") && !strings.Contains(body, "Lee"),
		fmt.Sprintf("status %d type %q disp %q", r.Status(), r.Header("Content-Type"), r.Header("Content-Disposition")))

	r = s.Request("GET", "/admin/giving/csv", uman, nil)
	lastBody = string(r.Body())
	expect("CSV export is refused without charges.read", r.Status() == 303 && !strings.Contains(string(r.Body()), "Date,Month"),
		fmt.Sprintf("status %d", r.Status()))

	// ---- No escalation ----
	annID := strconv.FormatInt(ids["ann"], 10)
	post(s, uman, "/admin/users/update/"+annID, url.Values{
		"user_id": {annID}, "username": {"ann"}, "email_address": {"ann@smoke.test"}, "firstname": {"Ann"},
		"role": {"1"}, "enabled": {"on"}, "password": {"takeover1"}, "password_confirm": {"takeover1"},
	})
	var annHash string
	_ = dbH.QueryRow(`SELECT encrypted_password FROM users WHERE id = $1`, ids["ann"]).Scan(&annHash)
	expect("users.update can't take over a more powerful account", annHash == "" && actorCan("ann", authz.RolesUpdate),
		"ann's password was set")

	r = s.Request("GET", "/admin/users", uman, nil)
	lastBody = string(r.Body())
	expect("users list locks rows the viewer can't manage", r.Status() == 200 &&
		strings.Contains(string(r.Body()), "locked"), fmt.Sprintf("status %d", r.Status()))

	umanID := strconv.FormatInt(ids["uman"], 10)
	post(s, uman, "/admin/users/update/"+umanID, url.Values{
		"user_id": {umanID}, "username": {"uman"}, "email_address": {"uman@smoke.test"}, "firstname": {"Uma"},
		"role": {"9"}, "enabled": {"on"},
		"role:" + strconv.FormatInt(userMgr, 10):                 {"on"},
		"role:" + strconv.FormatInt(roleID("Administrator"), 10): {"on"},
	})
	expect("can't assign yourself a role beyond your permissions", !actorCan("uman", authz.RolesUpdate) &&
		actorCan("uman", authz.UsersUpdate), "uman gained Administrator")

	post(s, uman, "/admin/users/update/"+umanID, url.Values{
		"user_id": {umanID}, "username": {"uman"}, "email_address": {"uman@smoke.test"}, "firstname": {"Uma"},
		"role": {"99"}, "enabled": {"on"}, "role:" + strconv.FormatInt(userMgr, 10): {"on"},
	})
	a, _, _ := authz.LoadActor(dbH, "uman")
	expect("only a SuperAdmin can grant SuperAdmin", a != nil && !a.IsSuper(), "uman became SuperAdmin")

	r = s.Request("GET", "/admin/roles/edit/"+strconv.FormatInt(roleID("Administrator"), 10), signIn("ed"), nil)
	lastBody = string(r.Body())
	expect("editor without roles.update is refused the role form", r.Status() == 303, fmt.Sprintf("status %d", r.Status()))

	// ---- Publish is its own permission ----
	r = post(s, ed, "/admin/articles", url.Values{
		"article_id": {"0"}, "article_title": {"Draft by Ed"}, "article_summary": {"s"}, "article_body": {"b"},
		"categories": {"news"}, "published": {"on"},
	})
	var published bool
	perr := dbH.QueryRow(`SELECT published FROM articles WHERE title = $1`, "Draft by Ed").Scan(&published)
	expect("editor's new article is saved but not published", perr == nil && !published,
		fmt.Sprintf("status %d err %v published %v", r.Status(), perr, published))

	r = s.Request("GET", "/admin/articles/new", ed, nil)
	lastBody = string(r.Body())
	expect("editor's article form disables the publish switch", r.Status() == 200 &&
		strings.Contains(string(r.Body()), "Publishing requires the articles.publish permission"),
		fmt.Sprintf("status %d", r.Status()))

	// ---- Role changes apply on the next request ----
	must("remove ed's roles", authz.SetUserRoles(dbH, ids["ed"], nil))
	r = s.Request("GET", "/admin/home", ed, nil)
	lastBody = string(r.Body())
	expect("revoking every role ends admin access immediately", r.Status() == 303 && r.Header("Location") == "/",
		fmt.Sprintf("status %d loc %q", r.Status(), r.Header("Location")))

	fmt.Println()
	if failures > 0 {
		fmt.Printf("%d check(s) FAILED\n", failures)
		os.Exit(1)
	}
	fmt.Println("all role checks passed")
}
