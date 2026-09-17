package menu

import (
	"strings"
	"testing"

	"github.com/rohanthewiz/church/resource/authz"
)

func TestLinkPermitted(t *testing.T) {
	editor := authz.NewActorForTest(2, "ed", 7, authz.ArticlesRead, authz.ArticlesCreate, authz.SermonsUpdate, authz.SermonsRead)
	giver := authz.NewActorForTest(3, "gv", 9, authz.ChargesRead)
	moderator := authz.NewActorForTest(4, "mod", 9, authz.ChatModerate) // site-only: no admin access
	super := authz.NewActorForTest(1, "root", authz.SuperAdminRole)

	cases := []struct {
		name  string
		actor *authz.Actor
		url   string
		want  bool
	}{
		{"public link, anonymous", nil, "/pages/about", true},
		{"external link, anonymous", nil, "https://example.com/admin/users", true},
		{"empty url", nil, "", true},
		{"admin link, anonymous", nil, "/admin/articles", false},
		{"debug, anonymous", nil, "/debug/show", false},

		{"list with read", editor, "/admin/articles", true},
		{"trailing slash and query", editor, "/admin/articles/?offset=10", true},
		{"new with create", editor, "/admin/articles/new", true},
		{"edit without update", editor, "/admin/articles/edit/5", false},
		{"list without read", editor, "/admin/users", false},
		{"roles without read", editor, "/admin/roles", false},
		{"giving without charges.read", editor, "/admin/giving", false},
		{"sermon cleanup rides update", editor, "/admin/sermons/cleanup", true},
		{"sermon import rides create", editor, "/admin/sermons/import", false},
		{"dashboard for any admin", editor, "/admin/home", true},
		{"bare prefix for any admin", editor, "/admin", true},
		{"logout for any admin", editor, "/admin/logout", true},
		{"unknown admin route for any admin", editor, "/admin/newsletter", true},
		{"prefix lookalike is public", editor, "/administration", true},
		{"debug needs super", editor, "/debug/show", false},

		{"giving maps to charges", giver, "/admin/giving", true},
		{"giving csv is a read", giver, "/admin/giving/csv/summary", true},

		{"moderator has no dashboard", moderator, "/admin/home", false},
		{"moderator has no unknown admin route", moderator, "/admin/newsletter", false},

		{"super sees everything", super, "/admin/roles/new", true},
		{"super sees debug", super, "/debug/show", true},
	}
	for _, c := range cases {
		if got := linkPermitted(c.actor, c.url); got != c.want {
			t.Errorf("%s: linkPermitted(%q) = %v, want %v", c.name, c.url, got, c.want)
		}
	}
}

// renderMain renders the hardwired main menu (nil executor) for a viewer whose
// permissions arrive the way admin pages pass them.
func renderMain(t *testing.T, loggedIn bool, perms string) string {
	t.Helper()
	nr := &navRender{loggedIn: loggedIn, glob: map[string]string{authz.ParamKey: perms}}
	html, _, _ := nr.buildMenu("main-menu")
	return html
}

func TestBuildMenuFiltersAdminSubmenu(t *testing.T) {
	// Anonymous: the IsAdmin submenu is skipped outright, as before.
	anon := renderMain(t, false, "")
	if strings.Contains(anon, "/admin") || strings.Contains(anon, ">Admin<") {
		t.Errorf("anonymous nav shows admin links:\n%s", anon)
	}
	if !strings.Contains(anon, `href="/pages/articles"`) {
		t.Errorf("anonymous nav lost public links:\n%s", anon)
	}

	// Editor with articles only: the dropdown keeps Dashboard, Articles and
	// Logout, and drops the areas they can't open.
	ed := renderMain(t, true, string(authz.ArticlesRead)+","+string(authz.ArticlesCreate))
	for _, want := range []string{`href="/admin/home"`, `href="/admin/articles"`, `href="/admin/logout"`, ">Admin<"} {
		if !strings.Contains(ed, want) {
			t.Errorf("editor nav missing %s:\n%s", want, ed)
		}
	}
	for _, unwanted := range []string{"/admin/users", "/admin/roles", "/admin/giving", "/admin/pages", "/admin/menus"} {
		if strings.Contains(ed, unwanted) {
			t.Errorf("editor nav shows %s:\n%s", unwanted, ed)
		}
	}

	// Super: every hardwired admin link.
	su := renderMain(t, true, "*")
	for _, want := range []string{"/admin/users", "/admin/roles", "/admin/giving", "/admin/pages"} {
		if !strings.Contains(su, want) {
			t.Errorf("super nav missing %s:\n%s", want, su)
		}
	}

	// Signed in with no resolvable permissions (e.g. a chat member on a public
	// page with no DB): every admin link is filtered, so the whole dropdown
	// goes rather than an empty "Admin" label.
	member := renderMain(t, true, "")
	if strings.Contains(member, ">Admin<") || strings.Contains(member, "/admin") {
		t.Errorf("member nav shows the Admin dropdown:\n%s", member)
	}
	if !strings.Contains(member, `href="/calendar"`) {
		t.Errorf("member nav lost public links:\n%s", member)
	}

	// Inactive items keep a bare <li>, not class="".
	if strings.Contains(su, `class=""`) {
		t.Errorf("nav renders an empty class attribute:\n%s", su)
	}
}
