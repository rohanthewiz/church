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
