package authz

// Runs the real query functions against an embedded bytdb over the loopback
// Postgres wire, the same way a bytdb site binary boots (db.InitDB). The
// queries were written to the subset of SQL both backends share; this is the
// proof for bytdb. Postgres accepts a strict superset of that subset.

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/util/inputerr"
)

func TestRolesAgainstBytDB(t *testing.T) {
	if err := db.InitDB(db.DBOpts{DBType: db.DBTypes.BytDB, File: filepath.Join(t.TempDir(), "authz.db")}); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(db.CloseDB)
	exec, err := db.Db()
	if err != nil {
		t.Fatalf("Db: %v", err)
	}

	// Legacy users: admin(1), publisher(5), editor(7), member(9), a disabled
	// admin, and a superadmin (99).
	type seedUser struct {
		name    string
		role    int
		enabled bool
	}
	ids := map[string]int64{}
	for _, u := range []seedUser{
		{"ann", 1, true}, {"pat", 5, true}, {"ed", 7, true}, {"mem", 9, true},
		{"gone", 1, false}, {"root", SuperAdminRole, true},
	} {
		var id int64
		err := exec.QueryRow(`INSERT INTO users (updated_by, enabled, role, username, email_address, first_name)
			VALUES ('test', $1, $2, $3, $4, $3) RETURNING id`, u.enabled, u.role, u.name, u.name+"@x.test").Scan(&id)
		if err != nil {
			t.Fatalf("seed user %s: %v", u.name, err)
		}
		ids[u.name] = id
	}

	// ---- Default roles + backfill ----
	if err := EnsureDefaultRoles(exec); err != nil {
		t.Fatalf("EnsureDefaultRoles: %v", err)
	}
	roles, err := ListRoles(exec)
	if err != nil || len(roles) != 3 {
		t.Fatalf("ListRoles = %d roles, err %v; want 3", len(roles), err)
	}
	// Second run is a no-op (the one-shot marker is a non-empty roles table).
	if err := EnsureDefaultRoles(exec); err != nil {
		t.Fatalf("EnsureDefaultRoles rerun: %v", err)
	}
	if again, _ := ListRoles(exec); len(again) != 3 {
		t.Fatalf("rerun created roles: %d", len(again))
	}

	actor := func(name string) (*Actor, bool) {
		t.Helper()
		a, found, err := LoadActor(exec, name)
		if err != nil {
			t.Fatalf("LoadActor %s: %v", name, err)
		}
		return a, found
	}

	if a, _ := actor("ann"); !a.Can(RolesUpdate) || !a.Can(ChargesRead) {
		t.Error("legacy admin should hold Administrator (everything)")
	}
	if a, _ := actor("pat"); !a.Can(ArticlesPublish) || !a.Can(MenusEnable) || a.Can(UsersRead) {
		t.Errorf("legacy publisher resolved wrongly: %v", a.Permissions().Sorted())
	}
	if a, _ := actor("ed"); !a.Can(ArticlesUpdate) || a.Can(ArticlesPublish) || a.Can(ArticlesDelete) {
		t.Errorf("legacy editor resolved wrongly: %v", a.Permissions().Sorted())
	}
	if a, _ := actor("mem"); a.HasAdminAccess() {
		t.Error("legacy registered user must get no admin access")
	}
	if _, found := actor("gone"); found {
		t.Error("a disabled user must not resolve")
	}
	if a, _ := actor("root"); !a.IsSuper() {
		t.Error("superadmin must resolve with the bypass")
	}

	// ---- Role CRUD ----
	_, err = SaveRole(exec, Role{Name: "editor"}, "test") // case-insensitive clash with "Editor"
	if _, isInput := inputerr.UserMessage(err); !isInput {
		t.Errorf("duplicate role name should be an input error, got %v", err)
	}

	greeterID, err := SaveRole(exec, Role{Name: "Greeter", Perms: NewSet(EventsPublish, "nope.nope")}, "test")
	if err != nil {
		t.Fatalf("SaveRole create: %v", err)
	}
	g, found, err := GetRole(exec, greeterID)
	if err != nil || !found {
		t.Fatalf("GetRole: found=%v err=%v", found, err)
	}
	if !g.Perms.Has(EventsPublish) || !g.Perms.Has(EventsRead) || g.Perms.Has("nope.nope") {
		t.Errorf("saved perms not normalized: %v", g.Perms.Sorted())
	}

	// Update swaps a permission (exercises delete-then-insert).
	g.Perms = NewSet(ChargesRead) // held by no default role but Administrator
	if _, err := SaveRole(exec, g, "test"); err != nil {
		t.Fatalf("SaveRole update: %v", err)
	}
	if g2, _, _ := GetRole(exec, greeterID); len(g2.Perms) != 1 || !g2.Perms.Has(ChargesRead) {
		t.Errorf("update left perms %v, want [charges.read]", g2.Perms.Sorted())
	}

	// ---- Multiple roles per user ----
	edRoles, _ := RoleIDsForUser(exec, ids["ed"])
	if err := SetUserRoles(exec, ids["ed"], append(edRoles, greeterID)); err != nil {
		t.Fatalf("SetUserRoles: %v", err)
	}
	if a, _ := actor("ed"); !a.Can(ChargesRead) || !a.Can(ArticlesUpdate) {
		t.Errorf("union of two roles resolved wrongly: %v", a.Permissions().Sorted())
	}
	names, err := RoleNamesByUser(exec)
	if err != nil || len(names[ids["ed"]]) != 2 || names[ids["ed"]][0] != "Editor" || names[ids["ed"]][1] != "Greeter" {
		t.Errorf("RoleNamesByUser[ed] = %v (err %v), want [Editor Greeter]", names[ids["ed"]], err)
	}

	// ---- Delete cascades assignments ----
	if err := DeleteRole(exec, greeterID); err != nil {
		t.Fatalf("DeleteRole: %v", err)
	}
	if a, _ := actor("ed"); a.Can(ChargesRead) {
		t.Error("deleting a role must revoke what it granted")
	}

	// ---- Publish flag resolution ----
	var artID int64
	err = exec.QueryRow(`INSERT INTO articles (updated_by, title, slug, summary, published, categories)
		VALUES ('test', 'Live', 'live', 's', true, '{}') RETURNING id`).Scan(&artID)
	if err != nil {
		t.Fatalf("seed article: %v", err)
	}
	ed, _ := actor("ed")
	idStr := strconv.FormatInt(artID, 10)
	if v, err := ResolveFlag(exec, ed, ArticlesPublish, FlagArticlePublished, idStr, false); err != nil || !v {
		t.Errorf("editor unticking a live article must keep it published: got %v err %v", v, err)
	}
	if v, _ := ResolveFlag(exec, ed, ArticlesPublish, FlagArticlePublished, "", true); v {
		t.Error("editor creating an article must not publish it")
	}
	pat, _ := actor("pat")
	if v, _ := ResolveFlag(exec, pat, ArticlesPublish, FlagArticlePublished, idStr, false); v {
		t.Error("publisher's submitted value must stand")
	}
}
