package event

import (
	"strconv"

	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
)

const ModuleTypeUpcomingEvents = "events_upcoming"

type ModuleUpcomingEvents struct {
	module.Presenter
}

func NewModuleUpcomingEvents(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleUpcomingEvents)
	mod.Name = pres.Name
	mod.Opts = pres.Opts

	// Work out local condition
	cond := "1 = 1"
	if !mod.Opts.IsAdmin && !mod.Opts.ShowUnpublished {
		cond = "published = true AND event_date::date >= now()::date"
	}
	// merge with any incoming condition
	if mod.Opts.Condition != "" {
		cond = mod.Opts.Condition + " AND " + cond
	}
	mod.Opts.Condition = cond

	return module.Module(mod), nil
}

func (m ModuleUpcomingEvents) GetData() ([]Presenter, error) {
	return QueryEvents(m.Opts.Condition, "event_date "+m.Order(), m.Opts.Limit, 0)
}

func (m *ModuleUpcomingEvents) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		if id, ok := opts["limit"]; ok { // I only ever see us changing the limit
			limit, err := strconv.ParseInt(id, 10, 64)
			if err == nil {
				m.Opts.Limit = limit
			}
		}
	}

	evts, err := m.GetData()

	if err != nil {
		logger.LogErr(err, "Error obtaining data in ModuleUpComingEvents", "error")
		return ""
	}

	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading ch-clickable-heading", "onclick", "window.location = '/events'").T(m.Opts.Title),
		b.DivClass("ch-module-body").R(
			b.Table().R(
				b.Wrap(func() {
					if len(evts) < 1 {
						b.Tr().R(
							b.Td("colspan", "2").T("No upcoming events"),
						)
					} else {
						for _, evt := range evts {
							b.Tr().R(
								b.Td().T(evt.EventDate),
								b.Td().R(
									b.A("href", "/events/"+evt.Id).T(evt.Title),
								),
							)
						}
					}
				}),
			),
		),
	)

	return b.String()
}
