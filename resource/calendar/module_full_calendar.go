package calendar

import (
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/element"
)

const ModuleTypeFullCalendar = "calendar"

type ModuleFullCalendar struct {
	module.Presenter
}

func NewModuleFullCalendar(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleFullCalendar)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

// FullCalendar will ask for its own data
// func (m ModuleFullCalendar) getData() (pres Presenter, err error) {
//	// If the module instance has an item slug defined, it takes highest precedence
//	if m.Opts.ItemSlug != "" {
//		pres, err = presenterFromSlug(m.Opts.ItemSlug)
//	} else {
//		if len(m.Opts.ItemIds) < 1 {
//			return pres, serr.Wrap(errors.New("No item ids found"),
//				"module_options", fmt.Sprintf("%#v", m.Opts))
//		}
//		pres, err = presenterFromId(m.Opts.ItemIds[0])  // Todo presenterFromId for other resources
//	}
//	return
// }

func (m *ModuleFullCalendar) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	b := element.NewBuilder()
	e := b.Ele
	t := b.Text

	calClass := "cal" + auth.RandomKey() // unique class for this instance
	e("div", "class", "ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		// e("h3", "class", "calendar-title").R("calendar"),
		e("div", "class", calClass).R(),
		e("script", "type", "text/javascript").R(t(`
			$(document).ready(function() {
				$('.`+calClass+`').fullCalendar({
					events: '/calendar',
					eventClick: function(calEvt, jsEvt, view) {
						window.location = calEvt.url;
					}
				});
			});
		`)),
	)
	return b.String()
}
