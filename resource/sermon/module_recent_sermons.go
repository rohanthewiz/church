package sermon

import (
	"strconv"

	"github.com/rohanthewiz/church/core/html"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	. "github.com/rohanthewiz/logger"
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
	return QuerySermons(m.Opts.Condition, "date_taught "+m.Order(), m.Opts.Limit, 0)
}

func (m *ModuleRecentSermons) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		if id, ok := opts["limit"]; ok { // I only ever see us changing the limit
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

	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading ch-clickable-heading", "onclick", "window.location = '/sermons'").T(m.Opts.Title),
		b.DivClass("ch-module-body").R(
			b.Table().R(
				b.Wrap(func() {
					if len(sermons) < 1 {
						b.Tr().R(b.Td("colspan", "3").T("No recent sermons"))
					} else {
						for _, ser := range sermons {
							b.Tr().R(
								b.Td().T(ser.DateTaughtShort),
								b.Td().R(b.A("href", "/sermons/"+ser.Id).T(ser.Title)),
								b.Td().R(
									b.Wrap(func() {
										if ser.AudioLink != "" {
											b.AClass("sermon-play-icon", "style", "font-size:0.9em", "href", ser.AudioLink).T(html.TriangleRightSmall)
											// b.T("&nbsp;")
											// b.A("href", ser.AudioLink, "title", "download", "style",
											// 	"text-decoration: none; font-size: 12px;").T("📥")
										} else {
											b.T("")
										}
									}),
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
