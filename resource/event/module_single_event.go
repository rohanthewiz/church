package event

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	. "github.com/rohanthewiz/logger"
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
	if len(m.Opts.ItemIds) < 1 {
		return
	}
	evt, err := findEventById(m.Opts.ItemIds[0])
	if err != nil {
		LogErr(err, "Unable to obtain event", "event_id", fmt.Sprintf("%d", m.Opts.ItemIds[0]))
		return pres, err
	}
	return presenterFromModel(evt,
		PresenterParams{TimeNormalFormat: "3:04 PM", DateLongFormat: "1/2/2006", DateTimeFormat: "1/2/2006 3:04 PM TZ"}), err
}

func (m *ModuleSingleEvent) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
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

	b := element.NewBuilder()

	b.H3().T("Event (" + evt.Title + ")")
	b.Table().R(
		b.Tr().R(b.Td().T("Name"), b.Td().T(evt.Title)),
		b.Tr().R(b.Td().T("Event Date"), b.Td().T(evt.EventDateDisplayLong)),
		b.Tr().R(b.Td().T("Event Time"), b.Td().T(evt.EventTime)),
		b.Tr().R(b.Td().T("Summary"), b.Td().T(evt.Summary)),
		b.Tr().R(b.Td().T("Description"), b.Td().T(evt.Body)),
		b.Tr().R(b.Td().T("Location"), b.Td().T(evt.Location)),
		b.Tr().R(b.Td().T("Contact Person"), b.Td().T(evt.ContactPerson)),
		b.Tr().R(b.Td().T("Contact Phone"), b.Td().T(evt.ContactPhone)),
		b.Tr().R(b.Td().T("Contact Email"), b.Td().T(evt.ContactEmail)),
		b.Tr().R(b.Td().T("Contact URL"), b.Td().T(evt.ContactURL)),
		b.Tr().R(b.Td().T("Categories"), b.Td().T(strings.Join(evt.Categories, ", "))),
		b.Tr().R(b.Td().T("Updated At"), b.Td().T(evt.UpdatedAt)),
	)
	b.Wrap(func() {
		if loggedIn && len(m.Opts.ItemIds) > 0 {
			b.AClass("edit-link", "href", m.GetEditURL()+
				strconv.FormatInt(m.Opts.ItemIds[0], 10)).R(
				b.ImgClass("edit-icon", "title", "Edit Event", "src", "/assets/images/edit_article.svg").R(),
			)
		}
	})
	return b.String()
}
