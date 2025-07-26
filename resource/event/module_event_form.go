package event

import (
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

type ModuleEventForm struct {
	module.Presenter
	csrf string
}

const ModuleTypeEventForm = "event_form"

// Event Form deals with only a single item referenced in ItemIds[0] or a new one otherwise
func NewModuleEventForm(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleEventForm)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	// Slug is set only when the module model (db) is created. mod.Opts.Slug = string_util.SlugWithRandomString(title)

	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token.")
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

// Since this is only called from Render(), so safeties are in the caller (Render())
func (m ModuleEventForm) getData() (pres Presenter, err error) {
	evt, err := findEventById(m.Opts.ItemIds[0])
	if err != nil {
		return pres, serr.Wrap(err, "Unable to obtain event with id: "+fmt.Sprintf("%d", m.Opts.ItemIds[0]))
	}
	return presenterFromModel(evt), err
}

func (m *ModuleEventForm) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	evt := Presenter{}
	var err error

	operation := "Create"
	action := ""

	if len(m.Opts.ItemIds) > 0 {
		operation = "Update"
		evt, err = m.getData()
		if err != nil {
			logger.LogErr(err, "Error in module render", "module_type", ModuleTypeEventForm)
			return ""
		}
		action = "/update/" + evt.Id
	}

	b := element.NewBuilder()

	b.DivClass("wrapper-material-form").R(
		b.H3("class", "page-title").T(operation+" "+m.Name.Singular),
		b.Form("method", "post", "action", "/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "name", "event_id", "value", evt.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),
			b.DivClass("form-inline").R(
				b.DivClass("form-group").R(
					b.Input("name", "event_title", "type", "text", "required", "required", "value", evt.Title),
					b.Label("class", "control-label", "for", "event_title").T("Event Title"),
					b.IClass("bar").T(""),
				),
				b.DivClass("form-group").R(
					b.Input("name", "event_location", "type", "text", "required", "required", "value", evt.Location),
					b.Label("class", "control-label", "for", "event_location").T("Location"),
					b.IClass("bar").T(""),
				),
			),
			b.DivClass("form-inline").R(
				b.DivClass("form-group").R(
					b.Input("name", "event_date", "type", "date", "value", evt.EventDate),
					b.Label("class", "control-label", "for", "event_date").T("Event Date"),
					// b.IClass("bar"),
				),
				b.DivClass("form-group").R(
					b.Input("name", "event_time", "type", "time", "value", evt.EventTime),
					b.Label("class", "control-label", "for", "event_time").T("Event Time"),
					// b.IClass("bar"),
				),
			),
			b.DivClass("form-inline").R(
				b.DivClass("form-group").R(
					b.Input("name", "contact_person", "type", "text", "placeholder", "(optional)", "value",
						evt.ContactPerson),
					b.Label("class", "control-label", "for", "contact_person").T("Contact Person"),
					b.IClass("bar").T(""),
				),
				b.DivClass("form-group").R(
					b.Input("name", "categories", "type", "text", "value", strings.Join(evt.Categories, ", "),
						"placeholder", "(optional)"),
					b.Label("class", "control-label", "for", "categories").T("Tags (comma separated)"),
					b.IClass("bar").T(""),
				),
			),
			b.DivClass("form-inline").R(
				b.DivClass("form-group").R(
					b.Input("name", "contact_email", "type", "text", "value", evt.ContactEmail),
					b.Label("class", "control-label", "for", "contact_email").T("Contact Email"),
					b.IClass("bar").T(""),
				),
				b.DivClass("form-group").R(
					b.Input("name", "contact_phone", "type", "text", "placeholder", "(optional)", "value",
						evt.ContactPhone),
					b.Label("class", "control-label", "for", "contact_phone").T("Contact Phone"),
					b.IClass("bar").T(""),
				),
				// b.DivClass("form-group").R(
				// 	b.Input("name", "contact_url", "type", "text", "placeholder", "(optional)", "value",
				// 		evt.ContactURL),
				// 	b.Label("class", "control-label", "for", "contact_url").T("Contact URL"),
				// 	b.IClass("bar"),
				// ),
			),
			b.DivClass("form-group bootstrap-wrapper").R(
				b.Div("id", "summer1").T(evt.Summary),
				b.TextArea("id", "event_summary", "name", "event_summary", "type", "text", "value", "",
					"style", "display:none").T(""),
				b.Label("class", "control-label", "for", "event_summary").T("Summary"),
			),
			b.DivClass("form-group bootstrap-wrapper").R(
				b.Div("id", "summer2").T(evt.Body),
				b.TextArea("id", "event_body", "name", "event_body", "type", "text", "value", "",
					"style", "display:none").T(""),
				b.Label("class", "control-label", "for", "event_body").T("Event Body"),
			),

			b.DivClass("checkbox").R(
				b.Label().R(
					b.Wrap(func() {
						if evt.Published {
							b.Input("type", "checkbox", "name", "published", "checked", "checked")
						} else {
							b.Input("type", "checkbox", "name", "published")
						}
					}),
					b.IClass("helper").T(""),
					b.T("Published"),
				),
				b.IClass("bar").T(""),
			),

			b.DivClass("form-group").R(
				b.Input("type", "submit", "class", "button", "value", operation),
			),
		),

		// b.Div("id", "react-app"),
		b.Script("type", "text/javascript").T(
			`$(document).ready(function(){$('#summer1').summernote(); $('#summer2').summernote();});
			function preSubmit() {  // todo validate fields here
				var s1 = $("#summer1");
				var s2 = $("#summer2");
				var summary = document.getElementById("event_summary");
				var body = document.getElementById("event_body");
				if (s1 && summary) {
					summary.innerHTML = s1.summernote('code');
				}
				if (s2 && body) {
					body.innerHTML = s2.summernote('code');
				}
				return true;
			}
		`),
	)

	return b.String()
}
