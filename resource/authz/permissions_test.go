package authz

import (
	"testing"
)

// Every named constant must be in the catalog, or a handler would check a
// permission no role form can ever grant.
func TestConstantsAreInCatalog(t *testing.T) {
	for _, p := range []Permission{
		MenusCreate, MenusRead, MenusUpdate, MenusDelete, MenusEnable,
		PagesCreate, PagesRead, PagesUpdate, PagesDelete, PagesPublish,
		ArticlesCreate, ArticlesRead, ArticlesUpdate, ArticlesDelete, ArticlesPublish,
		SermonsCreate, SermonsRead, SermonsUpdate, SermonsDelete, SermonsPublish,
		EventsCreate, EventsRead, EventsUpdate, EventsDelete, EventsPublish,
		UsersCreate, UsersRead, UsersUpdate, UsersDelete, UsersEnable,
		RolesCreate, RolesRead, RolesUpdate, RolesDelete,
		ChargesRead,
		ChatModerate,
	} {
		if !Valid(p) {
			t.Errorf("%s is not in the catalog", p)
		}
	}
	// Giving is read-only by design.
	for _, p := range []Permission{"charges.create", "charges.update", "charges.delete"} {
		if Valid(p) {
			t.Errorf("%s must not be grantable", p)
		}
	}
}

func TestNormalizeAddsReadAndDropsUnknown(t *testing.T) {
	got := Normalize(NewSet(ArticlesPublish, "bogus.thing", "articles.fly"))
	want := NewSet(ArticlesPublish, ArticlesRead)
	if len(got) != len(want) || !got.SubsetOf(want) {
		t.Errorf("Normalize = %v, want %v", got.Sorted(), want.Sorted())
	}
}

func TestActorChecks(t *testing.T) {
	var anon *Actor
	if anon.Can(ArticlesRead) || anon.HasAdminAccess() || anon.CanGrant(Set{}) {
		t.Error("a nil actor must be able to do nothing")
	}

	member := NewActorForTest(1, "joe", 9)
	if member.HasAdminAccess() {
		t.Error("an actor with no permissions must not enter the admin area")
	}

	editor := NewActorForTest(2, "ed", 7, ArticlesRead, ArticlesUpdate)
	if !editor.Can(ArticlesUpdate) || editor.Can(ArticlesPublish) {
		t.Error("editor permissions resolved wrongly")
	}
	if !editor.CanGrant(NewSet(ArticlesRead)) {
		t.Error("an actor may grant a subset of their own permissions")
	}
	if editor.CanGrant(NewSet(ArticlesRead, UsersUpdate)) {
		t.Error("an actor must not grant a permission they lack")
	}

	super := NewActorForTest(3, "root", SuperAdminRole)
	if !super.Can(ChargesRead) || !super.CanGrant(AllPermissions()) {
		t.Error("SuperAdmin must bypass every check")
	}

	// A site-only permission moderates the site but doesn't open the admin.
	moderator := NewActorForTest(4, "mo", 9, ChatModerate)
	if !moderator.Can(ChatModerate) || moderator.HasAdminAccess() {
		t.Error("chat.moderate alone must not grant admin access")
	}
	if !NewActorForTest(5, "mix", 9, ChatModerate, EventsRead).HasAdminAccess() {
		t.Error("an admin-area permission alongside chat.moderate must grant admin access")
	}
}

func TestLegacyModerator(t *testing.T) {
	cases := map[int]bool{
		99: true,  // SuperAdmin (the ordering exception)
		1:  true,  // Admin
		5:  true,  // Publisher
		7:  true,  // Author/Editor
		9:  false, // RegisteredUser
		0:  false, // no role loaded
	}
	for role, want := range cases {
		if got := LegacyModerator(role); got != want {
			t.Errorf("LegacyModerator(%d) = %v, want %v", role, got, want)
		}
	}
}

// countHolders is the arithmetic behind LocksOutRoleManagers; the queries
// are exercised against bytdb in queries_bytdb_test.go.
func TestCountHolders(t *testing.T) {
	perms := map[int64]Set{1: NewSet(RolesUpdate, RolesRead), 2: NewSet(ArticlesRead)}
	assign := map[int64][]int64{10: {2, 1}, 11: {2}, 12: {1}}
	eligible := map[int64]bool{10: true, 11: true} // 12 disabled or SuperAdmin
	if n := countHolders(RolesUpdate, perms, assign, eligible); n != 1 {
		t.Errorf("countHolders = %d, want 1 (only user 10)", n)
	}
	delete(perms, 1) // role deleted
	if n := countHolders(RolesUpdate, perms, assign, eligible); n != 0 {
		t.Errorf("countHolders after role delete = %d, want 0", n)
	}
}

// The render-params encoding must reproduce exactly what the actor may do.
func TestParamsRoundTrip(t *testing.T) {
	editor := NewActorForTest(2, "ed", 7, ArticlesRead, ArticlesUpdate)
	params := map[string]map[string]string{"_global": {"username": "ed", ParamKey: encode(editor)}}
	got := FromParams(params)
	if !got.Can(ArticlesUpdate) || got.Can(ArticlesPublish) || got.IsSuper() {
		t.Errorf("round trip lost permissions: %v", got.Permissions().Sorted())
	}

	super := NewActorForTest(3, "root", SuperAdminRole)
	params["_global"][ParamKey] = encode(super)
	if !FromParams(params).IsSuper() {
		t.Error("round trip lost the SuperAdmin bypass")
	}

	if FromParams(map[string]map[string]string{}).HasAdminAccess() {
		t.Error("missing params must decode to no access")
	}
}
