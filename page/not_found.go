package page

import (
	"github.com/rohanthewiz/church/errormodule"
	"github.com/rohanthewiz/church/module"
)

// NotFound is the hardwired page served (with a 404 status) when a dynamic
// page slug has no row. It renders inside the normal site layout, so the
// visitor keeps the header, nav and footer and can navigate on, rather than
// getting a bare error body.
//
// Built by hand instead of through pageFromPresenter/AddModules because the
// error module is not in modulesRegistry; it is only ever constructed
// directly. Published must be set, or Page.Render skips the module.
func NotFound() *Page {
	pg := pageFromPresenter(Presenter{
		Title:              "Page Not Found",
		AvailablePositions: []string{"center"},
	})
	pg.AddModule(errormodule.NewModuleError(module.Opts{
		Title:        "Sorry, we couldn't find that page.",
		ModuleType:   errormodule.ModuleTypeError,
		LayoutColumn: "center",
		Published:    true,
	}), "center")
	return pg
}
