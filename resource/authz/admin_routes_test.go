package authz

import "testing"

func TestAdminRoutesTableIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range AdminRoutes {
		key := r.Method + " " + r.Path
		if seen[key] {
			t.Errorf("duplicate row %s", key)
		}
		seen[key] = true
		if r.Method != "GET" && r.Method != "POST" {
			t.Errorf("%s: method must be GET or POST", key)
		}
		if r.Perm != AdminAccess && !Valid(r.Perm) {
			t.Errorf("%s: %q is not in the permission catalog", key, r.Perm)
		}
	}
}

func TestAdminRoutePerm(t *testing.T) {
	if p, ok := AdminRoutePerm("POST", "/articles/delete/:id"); !ok || p != ArticlesDelete {
		t.Errorf("POST /articles/delete/:id = %q, %v", p, ok)
	}
	// The router's lookup is exact: a concrete id is not a registered pattern.
	if _, ok := AdminRoutePerm("POST", "/articles/delete/5"); ok {
		t.Error("a concrete path matched a pattern in the router's exact lookup")
	}
	if _, ok := AdminRoutePerm("GET", "/articles/delete/:id"); ok {
		t.Error("method was ignored")
	}
}

func TestAdminURLPerm(t *testing.T) {
	cases := []struct {
		path string
		want Permission
		ok   bool
	}{
		{"/articles", ArticlesRead, true},
		{"/articles/new", ArticlesCreate, true},
		{"/articles/edit/12", ArticlesUpdate, true},
		{"/pages/new", PagesCreate, true}, // literal beats the /pages/:id preview
		{"/pages/12", PagesRead, true},    // the preview
		{"/giving/csv/summary", ChargesRead, true},
		{"/sermons/import", SermonsCreate, true},
		{"/sermons/cleanup", SermonsUpdate, true},
		{"/home", AdminAccess, true},
		{"/logout", AdminAccess, true},
		{"/articles/delete/12", "", false}, // POST only; not a link target
		{"/newsletter", "", false},         // a route a site added
		{"", "", false},                    // the bare prefix
		{"/articles/edit", "", false},      // too short for the pattern
	}
	for _, c := range cases {
		got, ok := AdminURLPerm(c.path)
		if got != c.want || ok != c.ok {
			t.Errorf("AdminURLPerm(%q) = %q, %v; want %q, %v", c.path, got, ok, c.want, c.ok)
		}
	}
}
