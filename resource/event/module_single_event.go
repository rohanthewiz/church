package event

import (
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/chweb/module"
	"fmt"
	"strings"
	"errors"
	"strconv"
	"github.com/rohanthewiz/element"
)

const ModuleTypeSingleEvent = "event_single"

type ModuleSingleEvent struct {
	module.Presenter
}

func NewModuleSingleEvent(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleSingleEvent)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

func (m ModuleSingleEvent) getData() (pres Presenter, err error) {
	if len(m.Opts.ItemIds) < 1 { return }
	evt, err := findEventById(m.Opts.ItemIds[0])
	if err != nil {
		LogErr(err, "Unable to obtain event", "event_id",  fmt.Sprintf("%d", m.Opts.ItemIds[0]))
		return pres, err
	}
	return presenterFromModel(evt), err
}

func (m *ModuleSingleEvent) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to us
		m.SetId(opts)
	}
	// Safety - todo add to all modules
	if len(m.Opts.ItemIds) == 0 {
		LogErr(errors.New("No id provided for module"), "Error rendering Single Module event",
			"module_options", fmt.Sprintf("%#v", m.Opts))
		return ""
	}

	evt, err := m.getData()
	if err != nil {
		LogErr(err, "Error in module render")
		return ""
	}
	e := element.New
	out := "<h3>Event (" + evt.Title + `)</h3><table>
		<tr><td>Name</td><td>` + evt.Title + `</td></tr>
		<tr><td>Event Date</td><td>` + evt.EventDate + `</td></tr>
		<tr><td>Event Time</td><td>` + evt.EventTime + `</td></tr>
		<tr><td>Summary</td><td>` + evt.Summary + `</td></tr>
		<tr><td>Description</td><td>` + evt.Body + `</td></tr>
		<tr><td>Location</td><td>` + evt.Location + `</td></tr>
		<tr><td>Contact Person</td><td>` + evt.ContactPerson + `</td></tr>
		<tr><td>Contact Phone</td><td>` + evt.ContactPhone + `</td></tr>
		<tr><td>Contact Email</td><td>` + evt.ContactEmail + `</td></tr>
		<tr><td>Contact URL</td><td>` + evt.ContactURL + `</td></tr>
		<tr><td>Categories</td><td>` + strings.Join(evt.Categories, ", ") + `</td></tr>
		<tr><td>Updated At</td><td>` + evt.UpdatedAt + `</td></tr>
		</table>`
	if loggedIn && len(m.Opts.ItemIds) > 0 {
		out += e("a", "class", "edit-link", "href", m.GetEditURL() +
			strconv.FormatInt(m.Opts.ItemIds[0], 10)).R(
			e("img", "class", "edit-icon", "title", "Edit Event", "src", "/assets/images/edit_article.svg").R(),
		)
	}
	return out
}
