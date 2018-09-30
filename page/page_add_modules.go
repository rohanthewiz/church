package page
// A page is essentially an arrangement of modules
import (
	"errors"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/errormodule"
	"fmt"
	"github.com/rohanthewiz/church/chweb/util/stringops"
)

// Future think of a way to automatically register a module
func (p *Page) AddModules(modules []module.Presenter) {
	p.modules = map[string][]module.Module{}  // instantiate

	for i, mod := range modules{  // todo sort by layout order
		if fun, ok := modulesRegistry[mod.Opts.ModuleType]; ok {

			// Fixup before add
			if mod.Opts.Slug == "" {
				mod.Opts.Slug = stringops.Slugify(p.Title + "." +
					mod.Opts.ModuleType + fmt.Sprintf("%d", i))
			}
			if name, ok := moduleTypeToName[mod.Opts.ModuleType]; ok {
				mod.Name = name
				mod.Opts.ItemsURLPath = name.Plural  // todo - deprecate this - use name.Plural instead
			}
			if mod.Opts.LayoutColumn == "" { mod.Opts.LayoutColumn = "center" }

			// Instantiate the module
			moduleInstance, err := fun(mod)
			if err != nil {
				logger.LogErr(err, "Error building module", "module_type", mod.Opts.ModuleType)
				emod := errormodule.NewModuleError(module.Opts{
					Title: "Hmm, something isn't quite right",
					ModuleType: errormodule.ModuleTypeError})
				p.AddModule(emod, mod.Opts.LayoutColumn)  // add an error module instead
				continue
			}
			//fmt.Printf("*|* module instance before add - %#v\n", moduleInstance )
			p.AddModule(moduleInstance, mod.Opts.LayoutColumn)
		} else {
			logger.LogErr(errors.New("module type unknown"), "module_type", mod.Opts.ModuleType)
		}
	}
}
