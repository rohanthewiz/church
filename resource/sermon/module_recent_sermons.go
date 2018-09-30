package sermon

import (
	"strconv"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/chweb/module"
)

const ModuleTypeRecentSermons = "sermons_recent"

type ModuleRecentSermons struct {
	module.Presenter
}

func NewModuleRecentSermons(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleRecentSermons)
	mod.Name = pres.Name
	mod.Opts = pres.Opts

	// Work out local condition
	cond := "1 = 1"
	if !mod.Opts.IsAdmin && !mod.Opts.ShowUnpublished {
		cond = "published = true"
	}
	// merge with any incoming condition
	if mod.Opts.Condition != "" {
		cond = mod.Opts.Condition + " AND " + cond
	}
	mod.Opts.Condition = cond

	return module.Module(mod), nil
}

func (m ModuleRecentSermons) GetData() ([]Presenter, error) {
	return QuerySermons(m.Opts.Condition, "date_taught " + m.Order(), m.Opts.Limit, 0)
}

func (m *ModuleRecentSermons) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		if id, ok := opts["limit"]; ok {  // I only ever see us changing the limit
			limit, err := strconv.ParseInt(id, 10, 64)
			if err == nil {
				m.Opts.Limit = limit
			}
		}
	}

	sermons, err := m.GetData()

	if err != nil {
		LogErr(err, "Error obtaining data in ModuleRecentSermons")
		return ""
	}
	out := `<div class="ch-module-wrapper ch-` + m.Opts.ModuleType + `"><div class="ch-module-heading ch-clickable-heading" onclick="window.location = '/sermons'">` + m.Opts.Title +
		`</div><div class="ch-module-body"><table>`
	if len(sermons) < 1 {
		out += `<tr><td colspan="2">No recent sermons</td></tr>`
	} else {
		for _, ser := range sermons {
			out += "<tr><td>" + ser.DateTaughtShort + `</td><td><a href="/sermons/` + ser.Id + `">` + ser.Title + "</a></td></tr>"
		}
	}
	out += "</table></div></div>"
	return out
}
