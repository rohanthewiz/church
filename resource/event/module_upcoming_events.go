package event

import (
	"strconv"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/chweb/module"
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
	return QueryEvents(m.Opts.Condition, "event_date " + m.Order(), m.Opts.Limit, 0)
}

func (m *ModuleUpcomingEvents) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to us
		if id, ok := opts["limit"]; ok {  // I only ever see us changing the limit
			limit, err := strconv.ParseInt(id, 10, 64)
			if err == nil {
				m.Opts.Limit = limit
			}
		}
	}

	evts, err := m.GetData()

	if err != nil {
		Log("Error", "Error obtaining data in ModuleUpComingEvents", "error", err.Error())
		return ""
	}
	out := `<div class="ch-module-wrapper ch-` + m.Opts.ModuleType + `"><div class="ch-module-heading ch-clickable-heading" onclick="window.location = '/events'">` + m.Opts.Title +
		`</div><div class="ch-module-body"><table>`
	if len(evts) < 1 {
		out += `<tr><td colspan="2">No upcoming events</td></tr>`
	} else {
		for _, evt := range evts {
			out += "<tr><td>" + evt.EventDate + `</td><td><a href="/events/` + evt.Id + `">` + evt.Title + "</a></td></tr>"
		}
	}

	out += "</table></div></div>"
	return out
}
