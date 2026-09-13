package page

import (
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/util/stringops"
)

// RoleForm is the hardwired Role Management form page (new and edit).
func RoleForm() (*Page, error) {
	title := "Role Form"
	pgdef := Presenter{
		Title: title, Slug: stringops.Slugify(title),
		IsAdmin:            true,
		AvailablePositions: []string{"center"},
	}
	pgdef.Modules = []module.Presenter{{
		Opts: module.Opts{
			Title:        "Role",
			ModuleType:   authz.ModuleTypeRoleForm,
			IsAdmin:      true,
			Published:    true,
			IsMainModule: true,
		},
	}}
	return pageFromPresenter(pgdef), nil
}

// RolesList is the hardwired Role Management list page.
func RolesList() (*Page, error) {
	title := "Roles List"
	pgdef := Presenter{Title: title, Slug: stringops.Slugify(title), IsAdmin: true}
	pgdef.Modules = []module.Presenter{{
		Opts: module.Opts{
			Title:        "Role Management",
			ModuleType:   authz.ModuleTypeRolesList,
			IsAdmin:      true,
			Published:    true,
			IsMainModule: true,
		},
	}}
	return pageFromPresenter(pgdef), nil
}
