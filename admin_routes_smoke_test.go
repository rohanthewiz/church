package church_test

// End-to-end check of role-based admin access through the production route
// wiring. It replaces the old `go run ./test_scripts/roles_smoke` script, so
// `go test ./...` (and any CI running it) fails if a route loses its Require
// decorator.
//
// It boots an embedded bytdb exactly as a site binary does (db.InitDB), seeds
// the default roles through the real bootstrap function, and wires the admin
// and debug areas with the production RegisterAdminRoutes and
// RegisterDebugRoutes. It then drives the HTTP handlers in-process (rweb
// Server.Request), so every request passes through the real guard,
// decorators, handlers, page renders and SQL:
//
//	sessions ─► AdminGuardRWeb ─► Require(perm) ─► handler ─► module render ─► bytdb
//
// Sessions are planted straight into the in-process kvstore rather than
// logging in, because login is covered by auth_controller's tests. Outcomes
// are asserted on database state where possible, not on flash text.
//
// The checks share one database and run in order, because later checks depend
// on earlier state (a role created, then assigned, then revoked). A failed
// check is reported and the run continues, as the script did, so one
// regression doesn't hide the rest.
//
// resource/auth's init loads cfg/random_seeds.txt relative to the package
// directory; at the module root that is the committed cfg/ file.

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rohanthewiz/church"
	"github.com/rohanthewiz/church/app"
	authctlr "github.com/rohanthewiz/church/auth_controller"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/resource/chat"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/rweb"
)

func TestAdminRoutesSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("boots an embedded database and renders pages; skipped with -short")
	}

	// lastBody is the most recent response body, shown when a check fails so
	// a render error is visible without a debugger.
	var lastBody string
	check := func(label string, cond bool, detail string) {
		t.Helper()
		if cond {
			t.Logf("pass  %s", label)
			return
		}
		snip := lastBody
		if len(snip) > 400 {
			snip = snip[:400]
		}
		t.Errorf("FAIL  %s: %s\n      body: %s", label, detail, snip)
	}
	must := func(label string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}

	must("InitDB", db.InitDB(db.DBOpts{DBType: db.DBTypes.BytDB, File: filepath.Join(t.TempDir(), "church.db")}))
	t.Cleanup(db.CloseDB)
	dbH, err := db.Db()
	must("Db", err)

	// Legacy users: ann=Admin(1), ed=Editor(7), root=SuperAdmin(99),
	// uman=RegisteredUser(9) who will get a users-only role,
	// mo=RegisteredUser(9) who will get chat moderation only, and
	// rita=RegisteredUser(9) who will get read-only articles.
	ids := map[string]int64{}
	for _, u := range []struct {
		name string
		role int
	}{{"ann", 1}, {"ed", 7}, {"root", 99}, {"uman", 9}, {"mo", 9}, {"rita", 9}} {
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
	// Sermon audio uploads land here (IDrive itself stays disabled)
	sermonsDir := t.TempDir()
	config.Options.IDrive.LocalSermonsDir = sermonsDir
	page.RegisterModules()
	s := rweb.NewServer(rweb.ServerOptions{})
	church.RegisterAdminRoutes(s)
	church.RegisterDebugRoutes(s)
	// The chat moderation endpoints, wired as in ServeRWeb (session middleware
	// only; the handlers do their own identity and permission checks).
	cht := s.Group("/chat", authctlr.UseCustomContextRWeb)
	cht.Get("/messages", chat.ListMessagesRWeb)
	cht.Post("/keep/:id", chat.KeepMessageRWeb)

	// signIn plants a session for username and returns its cookie header.
	signIn := func(username string) []rweb.Header {
		key := auth.RandomKey()
		must("save session", session.Session{Username: username}.Save(key))
		return []rweb.Header{{Key: "Cookie", Value: session.CookieName + "=" + key}}
	}
	// post submits a urlencoded form with a fresh CSRF token.
	post := func(who []rweb.Header, path string, vals url.Values) rweb.Response {
		tok, err := app.GenerateFormToken()
		must("csrf token", err)
		vals.Set("csrf", tok)
		hdrs := append([]rweb.Header{{Key: "Content-Type", Value: "application/x-www-form-urlencoded"}}, who...)
		r := s.Request("POST", path, hdrs, strings.NewReader(vals.Encode()))
		lastBody = string(r.Body())
		return r
	}
	get := func(who []rweb.Header, path string) (rweb.Response, string) {
		r := s.Request("GET", path, who, nil)
		lastBody = string(r.Body())
		return r, lastBody
	}
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
	id := func(name string) string { return strconv.FormatInt(ids[name], 10) }
	roleField := func(rid int64) string { return authz.RoleFieldPrefix + strconv.FormatInt(rid, 10) }
	// userForm is a user-update post for a seeded user; add role boxes to it.
	userForm := func(name string, legacyRole int) url.Values {
		return url.Values{"user_id": {id(name)}, "username": {name}, "email_address": {name + "@smoke.test"},
			"firstname": {name}, "role": {strconv.Itoa(legacyRole)}, "enabled": {"on"}}
	}
	// roleForm is a role-update post holding every catalog permission except skip.
	roleForm := func(rid int64, name string, skip authz.Permission) url.Values {
		v := url.Values{"role_id": {strconv.FormatInt(rid, 10)}, "role_name": {name}}
		for _, res := range authz.Catalog() {
			for _, act := range res.Actions {
				if p := res.Perm(act); p != skip {
					v.Set(authz.PermFieldPrefix+string(p), "on")
				}
			}
		}
		return v
	}

	root, ann, ed, uman, mo, rita := signIn("root"), signIn("ann"), signIn("ed"), signIn("uman"), signIn("mo"), signIn("rita")

	// ---- Role Management screens ----
	r, body := get(root, "/admin/home")
	check("superadmin dashboard renders Roles and Giving cards", r.Status() == 200 &&
		strings.Contains(body, "/admin/roles") && strings.Contains(body, "/admin/giving"),
		fmt.Sprintf("status %d", r.Status()))

	r, body = get(root, "/admin/roles")
	check("roles list shows the three default roles", r.Status() == 200 &&
		strings.Contains(body, "Administrator") && strings.Contains(body, "Publisher") && strings.Contains(body, "Editor"),
		fmt.Sprintf("status %d", r.Status()))

	r, body = get(root, "/admin/roles/new")
	check("role form renders the permission matrix", r.Status() == 200 &&
		strings.Contains(body, `name="perm:articles.publish"`) &&
		strings.Contains(body, `name="perm:charges.read"`) &&
		strings.Contains(body, `name="perm:chat.moderate"`),
		fmt.Sprintf("status %d", r.Status()))

	r = post(root, "/admin/roles", url.Values{
		"role_id": {"0"}, "role_name": {"Greeter"}, "role_description": {"Front door"},
		"perm:events.publish": {"on"},
		"perm:charges.create": {"on"}, // not in the catalog: must be ignored
	})
	greeter := roleID("Greeter")
	g, _, _ := authz.GetRole(dbH, greeter)
	check("create role from a permission combination", r.Status() == 303 && greeter != 0 &&
		len(g.Perms) == 2 && g.Perms.Has(authz.EventsPublish) && g.Perms.Has(authz.EventsRead),
		fmt.Sprintf("status %d, perms %v", r.Status(), g.Perms.Sorted()))

	r = post(root, "/admin/roles", url.Values{"role_id": {"0"}, "role_name": {"User Manager"},
		"perm:users.create": {"on"}, "perm:users.update": {"on"}, "perm:users.delete": {"on"}, "perm:users.enable": {"on"}})
	userMgr := roleID("User Manager")
	check("create a users-only role", userMgr != 0, fmt.Sprintf("status %d", r.Status()))

	// ---- Multiple roles on the Users form ----
	r, body = get(root, "/admin/users/edit/"+id("ed"))
	check("user form lists roles as checkboxes", r.Status() == 200 &&
		strings.Contains(body, `name="`+roleField(greeter)+`"`),
		fmt.Sprintf("status %d", r.Status()))

	edForm := userForm("ed", 7)
	edForm.Set(roleField(roleID("Editor")), "on")
	edForm.Set(roleField(greeter), "on")
	r = post(root, "/admin/users/update/"+id("ed"), edForm)
	check("assign two roles to one user (union applies)", r.Status() == 303 &&
		actorCan("ed", authz.EventsPublish) && actorCan("ed", authz.ArticlesUpdate),
		fmt.Sprintf("status %d loc %q", r.Status(), r.Header("Location")))

	umanForm := userForm("uman", 9)
	umanForm.Set(roleField(userMgr), "on")
	r = post(root, "/admin/users/update/"+id("uman"), umanForm)
	check("assign User Manager to uman", r.Status() == 303 && actorCan("uman", authz.UsersUpdate), "")

	// ---- Route enforcement ----
	r, body = get(uman, "/admin/articles")
	check("uman (users only) is refused articles", r.Status() == 303 && r.Header("Location") == "/admin/home" &&
		!strings.Contains(body, "ch-module-wrapper"), fmt.Sprintf("status %d loc %q", r.Status(), r.Header("Location")))
	r, body = get(uman, "/admin/home")
	// The whole page includes the site nav, whose Admin submenu is filtered
	// by permission too (menu.linkPermitted).
	check("uman's nav offers Users, not Articles, Roles or Giving", r.Status() == 200 &&
		strings.Contains(body, `href="/admin/users"`) && !strings.Contains(body, `href="/admin/articles"`) &&
		!strings.Contains(body, `href="/admin/roles"`) && !strings.Contains(body, `href="/admin/giving"`),
		fmt.Sprintf("status %d", r.Status()))
	// Then the dashboard cards alone.
	if i := strings.Index(body, `class="af-dash"`); i >= 0 {
		body = body[i:]
	}
	check("uman's dashboard shows Users, hides Articles and Roles", r.Status() == 200 &&
		strings.Contains(body, `href="/admin/users"`) && !strings.Contains(body, `href="/admin/articles"`) &&
		!strings.Contains(body, `href="/admin/roles"`), fmt.Sprintf("status %d users=%v articles=%v roles=%v", r.Status(),
		strings.Contains(body, `href="/admin/users"`), strings.Contains(body, `href="/admin/articles"`),
		strings.Contains(body, `href="/admin/roles"`)))
	r, body = get(ann, "/admin/giving")
	check("Administrator reads giving records", r.Status() == 200 &&
		strings.Contains(body, "Giving Records"), fmt.Sprintf("status %d", r.Status()))

	// ---- Debug tools are SuperAdmin-only ----
	r, _ = get(ann, "/debug/show")
	check("an Administrator (not SuperAdmin) is refused /debug", r.Status() == 303 &&
		r.Header("Location") == "/admin/home", fmt.Sprintf("status %d loc %q", r.Status(), r.Header("Location")))
	r, _ = get(nil, "/debug/show")
	check("an anonymous visitor is sent to login from /debug", r.Status() == 303 &&
		r.Header("Location") == "/login", fmt.Sprintf("status %d loc %q", r.Status(), r.Header("Location")))
	r, _ = get(root, "/debug/show")
	check("a SuperAdmin opens /debug", r.Status() == 200, fmt.Sprintf("status %d", r.Status()))

	// ---- Giving by month, year navigation, CSV exports ----
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
	r, body = get(ann, "/admin/giving")
	check("giving defaults to YTD grouped by month with a link back one year", r.Status() == 200 &&
		strings.Contains(body, "Year to date") && strings.Contains(body, thisMonth) &&
		strings.Contains(body, "$2,500.00") && strings.Contains(body, fmt.Sprintf("?year=%d", now.Year()-1)),
		fmt.Sprintf("status %d", r.Status()))
	check("giving escapes donor-supplied names", !strings.Contains(body, "<b>Kim</b>") &&
		strings.Contains(body, "&lt;b&gt;Kim&lt;/b&gt;"), "raw donor markup reached the page")
	check("giving offers both exports for the year shown",
		strings.Contains(body, fmt.Sprintf("/admin/giving/csv?year=%d", now.Year())) &&
			strings.Contains(body, fmt.Sprintf("/admin/giving/csv/summary?year=%d", now.Year())), "export links missing")

	r, body = get(ann, fmt.Sprintf("/admin/giving?year=%d", now.Year()-1))
	check("previous year is the earliest: no further back link, a forward link", r.Status() == 200 &&
		strings.Contains(body, "Full year") && strings.Contains(body, "$10.00") &&
		!strings.Contains(body, fmt.Sprintf("?year=%d", now.Year()-2)) &&
		strings.Contains(body, fmt.Sprintf("?year=%d", now.Year())), fmt.Sprintf("status %d", r.Status()))

	r, body = get(ann, "/admin/giving/csv")
	check("CSV export downloads with headings", r.Status() == 200 &&
		strings.HasPrefix(r.Header("Content-Type"), "text/csv") &&
		strings.Contains(r.Header("Content-Disposition"), fmt.Sprintf("giving-%d-ytd.csv", now.Year())) &&
		strings.Contains(body, "Date,Month,Name,Email,Amount,Refunded,Net,Status") &&
		strings.Contains(body, "2500.00") && !strings.Contains(body, "Lee"),
		fmt.Sprintf("status %d type %q disp %q", r.Status(), r.Header("Content-Type"), r.Header("Content-Disposition")))

	r, body = get(ann, "/admin/giving/csv/summary")
	check("summary CSV downloads month totals and a Total row", r.Status() == 200 &&
		strings.HasPrefix(r.Header("Content-Type"), "text/csv") &&
		strings.Contains(r.Header("Content-Disposition"), fmt.Sprintf("giving-%d-ytd-summary.csv", now.Year())) &&
		strings.Contains(body, "Month,Gifts,Gross,Refunded,Net,Pending") &&
		strings.Contains(body, now.Format("2006-01")+",1,2500.00,0.00,2500.00,0") &&
		strings.Contains(body, "Total,1,2500.00,0.00,2500.00,0") && !strings.Contains(body, "Kim"),
		fmt.Sprintf("status %d disp %q", r.Status(), r.Header("Content-Disposition")))

	for _, path := range []string{"/admin/giving/csv", "/admin/giving/csv/summary"} {
		r, body = get(uman, path)
		check(path+" is refused without charges.read", r.Status() == 303 && !strings.Contains(body, "Month,"),
			fmt.Sprintf("status %d", r.Status()))
	}

	// ---- No escalation ----
	annForm := userForm("ann", 1)
	annForm.Set("password", "takeover1")
	annForm.Set("password_confirm", "takeover1")
	post(uman, "/admin/users/update/"+id("ann"), annForm)
	var annHash string
	_ = dbH.QueryRow(`SELECT encrypted_password FROM users WHERE id = $1`, ids["ann"]).Scan(&annHash)
	check("users.update can't take over a more powerful account", annHash == "" && actorCan("ann", authz.RolesUpdate),
		"ann's password was set")

	r, body = get(uman, "/admin/users")
	check("users list locks rows the viewer can't manage", r.Status() == 200 &&
		strings.Contains(body, "locked"), fmt.Sprintf("status %d", r.Status()))

	selfPromote := userForm("uman", 9)
	selfPromote.Set(roleField(userMgr), "on")
	selfPromote.Set(roleField(roleID("Administrator")), "on")
	post(uman, "/admin/users/update/"+id("uman"), selfPromote)
	check("can't assign yourself a role beyond your permissions", !actorCan("uman", authz.RolesUpdate) &&
		actorCan("uman", authz.UsersUpdate), "uman gained Administrator")

	superGrab := userForm("uman", 99)
	superGrab.Set(roleField(userMgr), "on")
	post(uman, "/admin/users/update/"+id("uman"), superGrab)
	a, _, _ := authz.LoadActor(dbH, "uman")
	check("only a SuperAdmin can grant SuperAdmin", a != nil && !a.IsSuper(), "uman became SuperAdmin")

	r, _ = get(signIn("ed"), "/admin/roles/edit/"+strconv.FormatInt(roleID("Administrator"), 10))
	check("editor without roles.update is refused the role form", r.Status() == 303, fmt.Sprintf("status %d", r.Status()))

	// ---- Role-manager lockout ----
	// ann is the only enabled, non-SuperAdmin account holding roles.update.
	adminRole := roleID("Administrator")
	annDrop := userForm("ann", 1) // no role boxes: would remove Administrator
	post(ann, "/admin/users/update/"+id("ann"), annDrop)
	check("the last role manager can't remove their own roles", actorCan("ann", authz.RolesUpdate),
		"ann lost roles.update")

	annDisable := userForm("ann", 1)
	annDisable.Del("enabled")
	annDisable.Set(roleField(adminRole), "on")
	post(ann, "/admin/users/update/"+id("ann"), annDisable)
	check("the last role manager can't disable their own account", actorCan("ann", authz.RolesUpdate),
		"ann was disabled")

	post(ann, "/admin/roles/update/"+strconv.FormatInt(adminRole, 10), roleForm(adminRole, "Administrator", authz.RolesUpdate))
	check("roles.update can't be taken out of the only role granting it", actorCan("ann", authz.RolesUpdate),
		"Administrator lost roles.update")

	post(ann, "/admin/roles/delete/"+strconv.FormatInt(adminRole, 10), url.Values{})
	check("the only role granting roles.update can't be deleted", roleID("Administrator") == adminRole,
		"Administrator was deleted")

	// A SuperAdmin is the recovery path and is exempt: take it away, then
	// put it back.
	post(root, "/admin/roles/update/"+strconv.FormatInt(adminRole, 10), roleForm(adminRole, "Administrator", authz.RolesUpdate))
	superRemoved := !actorCan("ann", authz.RolesUpdate)
	post(root, "/admin/roles/update/"+strconv.FormatInt(adminRole, 10), roleForm(adminRole, "Administrator", ""))
	check("a SuperAdmin may remove the last roles.update grant (and restore it)",
		superRemoved && actorCan("ann", authz.RolesUpdate), fmt.Sprintf("removed=%v", superRemoved))

	// ---- Publish is its own permission ----
	r = post(ed, "/admin/articles", url.Values{
		"article_id": {"0"}, "article_title": {"Draft by Ed"}, "article_summary": {"s"}, "article_body": {"b"},
		"categories": {"news"}, "published": {"on"},
	})
	var published bool
	perr := dbH.QueryRow(`SELECT published FROM articles WHERE title = $1`, "Draft by Ed").Scan(&published)
	check("editor's new article is saved but not published", perr == nil && !published,
		fmt.Sprintf("status %d err %v published %v", r.Status(), perr, published))

	r, body = get(ed, "/admin/articles/new")
	check("editor's article form disables the publish switch", r.Status() == 200 &&
		strings.Contains(body, "Publishing requires the articles.publish permission"),
		fmt.Sprintf("status %d", r.Status()))

	// ---- List actions follow permissions ----
	r = post(root, "/admin/roles", url.Values{"role_id": {"0"}, "role_name": {"Article Reader"},
		"perm:articles.read": {"on"}})
	ritaForm := userForm("rita", 9)
	ritaForm.Set(roleField(roleID("Article Reader")), "on")
	post(root, "/admin/users/update/"+id("rita"), ritaForm)

	r, body = get(rita, "/admin/articles")
	check("a read-only viewer sees the article list without +, Edit or Delete", r.Status() == 200 &&
		strings.Contains(body, "Draft by Ed") && !strings.Contains(body, `class="btn-add"`) &&
		!strings.Contains(body, "/admin/articles/edit/") && !strings.Contains(body, "/admin/articles/delete/"),
		fmt.Sprintf("status %d add=%v edit=%v delete=%v", r.Status(), strings.Contains(body, `class="btn-add"`),
			strings.Contains(body, "/admin/articles/edit/"), strings.Contains(body, "/admin/articles/delete/")))
	check("a read-only viewer's nav offers the dashboard and Articles only", strings.Contains(body, `href="/admin/home"`) &&
		strings.Contains(body, `href="/admin/articles"`) && !strings.Contains(body, `href="/admin/users"`) &&
		!strings.Contains(body, `href="/admin/pages"`) && !strings.Contains(body, `href="/admin/giving"`), "")

	r, body = get(ed, "/admin/articles")
	check("an Editor gets + and Edit but no Delete", r.Status() == 200 &&
		strings.Contains(body, `class="btn-add"`) && strings.Contains(body, "/admin/articles/edit/") &&
		!strings.Contains(body, "/admin/articles/delete/"), fmt.Sprintf("status %d", r.Status()))

	r, body = get(root, "/admin/articles")
	check("a SuperAdmin gets Delete", r.Status() == 200 && strings.Contains(body, "/admin/articles/delete/"),
		fmt.Sprintf("status %d", r.Status()))

	// ---- Sermon audio upload ----
	// uploadSermon posts the sermon form as multipart with an audio file.
	uploadSermon := func(title, filename, content string) rweb.Response {
		tok, err := app.GenerateFormToken()
		must("csrf token", err)
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		for k, v := range map[string]string{"csrf": tok, "sermon_id": "0", "sermon_title": title,
			"sermon_date": "2026-09-06", "pastor-teacher": "Pastor", "categories": "", "scripture_refs": ""} {
			_ = mw.WriteField(k, v)
		}
		fw, err := mw.CreateFormFile("sermon_audio", filename)
		must("multipart file", err)
		_, _ = fw.Write([]byte(content))
		must("multipart close", mw.Close())
		hdrs := append([]rweb.Header{{Key: "Content-Type", Value: mw.FormDataContentType()}}, root...)
		r := s.Request("POST", "/admin/sermons", hdrs, &buf)
		lastBody = string(r.Body())
		return r
	}
	// leftovers lists staged upload files still in the year directory
	leftovers := func() []string {
		matches, _ := filepath.Glob(filepath.Join(sermonsDir, "2026", ".*.upload-*"))
		return matches
	}
	audioPath := filepath.Join(sermonsDir, "2026", "upload-check.mp3")

	r = uploadSermon("Upload Check", "upload-check.mp3", "first audio")
	got, _ := os.ReadFile(audioPath)
	check("sermon audio upload lands in place with no staged file left", r.Status() == 303 &&
		string(got) == "first audio" && len(leftovers()) == 0,
		fmt.Sprintf("status %d content %q leftovers %v", r.Status(), got, leftovers()))

	r = uploadSermon("Upload Check 2", "upload-check.mp3", "second audio")
	got, _ = os.ReadFile(audioPath)
	check("re-uploading the same name replaces the file whole", r.Status() == 303 &&
		string(got) == "second audio" && len(leftovers()) == 0,
		fmt.Sprintf("status %d content %q leftovers %v", r.Status(), got, leftovers()))

	r = uploadSermon("Upload Escape", "..%2F..%2Fescaped.mp3", "nope")
	escSaved := 0
	_ = dbH.QueryRow(`SELECT COUNT(*) FROM sermons WHERE title = $1`, "Upload Escape").Scan(&escSaved)
	_, escErr := os.Stat(filepath.Join(sermonsDir, "escaped.mp3"))
	_, escErr2 := os.Stat(filepath.Join(sermonsDir, "2026", "escaped.mp3"))
	check("an upload name with a path is refused and writes nothing", r.Status() == 303 && escSaved == 0 &&
		os.IsNotExist(escErr) && os.IsNotExist(escErr2) && len(leftovers()) == 0,
		fmt.Sprintf("status %d saved %d stat %v / %v", r.Status(), escSaved, escErr, escErr2))

	// ---- Duplicate titles ----
	// Slugs carry a time-and-hash suffix (stringops.SlugWithRandomString), so
	// a repeated title must save as a second item, not fail on the unique
	// slug index.
	for i := 0; i < 2; i++ {
		post(root, "/admin/articles", url.Values{"article_id": {"0"}, "article_title": {"Same Title"},
			"article_summary": {"s"}, "article_body": {"b"}, "categories": {"news"}})
		post(root, "/admin/sermons", url.Values{"sermon_id": {"0"}, "sermon_title": {"Same Sermon"},
			"sermon_date": {"2026-09-06"}, "pastor-teacher": {"Pastor"}, "categories": {""}, "scripture_refs": {""}})
	}
	distinctSlugs := func(table, title string) (int, error) {
		rows, err := dbH.Query(`SELECT slug FROM `+table+` WHERE title = $1`, title)
		if err != nil {
			return 0, err
		}
		defer rows.Close()
		seen := map[string]bool{}
		for rows.Next() {
			var slug string
			if err := rows.Scan(&slug); err != nil {
				return 0, err
			}
			seen[slug] = true
		}
		return len(seen), rows.Err()
	}
	nArt, errArt := distinctSlugs("articles", "Same Title")
	check("two articles with the same title both save, with distinct slugs", errArt == nil && nArt == 2,
		fmt.Sprintf("distinct slugs %d err %v", nArt, errArt))
	nSer, errSer := distinctSlugs("sermons", "Same Sermon")
	check("two sermons with the same title both save, with distinct slugs", errSer == nil && nSer == 2,
		fmt.Sprintf("distinct slugs %d err %v", nSer, errSer))

	// ---- Chat moderation by permission ----
	msg, err := chat.InsertMessage(dbH, chat.Message{Channel: "community", UserId: ids["ann"],
		Username: "ann", DisplayName: "Ann", Body: "keep me"})
	must("seed chat message", err)
	keepPath := "/chat/keep/" + strconv.FormatInt(msg.Id, 10)
	keep := func(who []rweb.Header) rweb.Response {
		hdrs := append([]rweb.Header{{Key: "Content-Type", Value: "application/x-www-form-urlencoded"}}, who...)
		r := s.Request("POST", keepPath, hdrs, strings.NewReader("keep=true"))
		lastBody = string(r.Body())
		return r
	}

	r = keep(mo)
	check("a member without chat.moderate can't pin a message", r.Status() == 403, fmt.Sprintf("status %d", r.Status()))

	r = post(root, "/admin/roles", url.Values{"role_id": {"0"}, "role_name": {"Chat Moderator"},
		"perm:chat.moderate": {"on"}})
	modRole := roleID("Chat Moderator")
	moForm := userForm("mo", 9)
	moForm.Set(roleField(modRole), "on")
	post(root, "/admin/users/update/"+id("mo"), moForm)

	r = keep(mo)
	stored, _, _ := chat.GetMessage(dbH, msg.Id)
	check("a member holding chat.moderate pins a message", r.Status() == 200 && stored.Keep,
		fmt.Sprintf("status %d keep %v", r.Status(), stored.Keep))
	_, body = get(mo, "/chat/messages?channel=community")
	check("the chat widget is told the moderator may moderate", strings.Contains(body, `"can_moderate":true`),
		"can_moderate not true")
	r, _ = get(mo, "/admin/home")
	check("chat.moderate alone doesn't open the admin area", r.Status() == 303 && r.Header("Location") == "/",
		fmt.Sprintf("status %d loc %q", r.Status(), r.Header("Location")))

	// ---- Role changes apply on the next request ----
	must("remove ed's roles", authz.SetUserRoles(dbH, ids["ed"], nil))
	r, _ = get(ed, "/admin/home")
	check("revoking every role ends admin access immediately", r.Status() == 303 && r.Header("Location") == "/",
		fmt.Sprintf("status %d loc %q", r.Status(), r.Header("Location")))
}
