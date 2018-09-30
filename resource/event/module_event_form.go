package event

import (
	"fmt"
	"strings"
	"github.com/rohanthewiz/serr"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/app"
	"github.com/rohanthewiz/element"
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
	//Slug is set only when the module model (db) is created. mod.Opts.Slug = string_util.SlugWithRandomString(title)

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
		return pres, serr.Wrap(err, "Unable to obtain event with id: " + fmt.Sprintf("%d", m.Opts.ItemIds[0]))
	}
	return presenterFromModel(evt), err
}

func (m *ModuleEventForm) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	evt := Presenter{}; var err error

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

	e := element.New

	published := e("input", "type", "checkbox", "name", "published")
	if evt.Published {
		published.AddAttributes("checked", "checked")
	}

	out := e("div", "class", "wrapper-material-form").R(
		e("h3", "class", "page-title").R(operation + " " + m.Name.Singular),
		e("form", "method", "post", "action", "/admin/" + m.Name.Plural + action, "onSubmit", "return preSubmit();").R(
			e("input", "type", "hidden", "name", "event_id", "value", evt.Id).R(),
			e("input", "type", "hidden", "name", "csrf", "value", m.csrf).R(),
			e("div", "class", "form-inline").R(
				e("div", "class", "form-group").R(
					e("input", "name", "event_title", "type", "text", "required", "required", "value", evt.Title).R(),
					e("label", "class", "control-label", "for", "event_title").R("Event Title"),
					e("i", "class", "bar").R(),
				),
				e("div", "class", "form-group").R(
					e("input", "name", "event_location", "type", "text", "required", "required", "value", evt.Location).R(),
					e("label", "class", "control-label", "for", "event_location").R("Location"),
					e("i", "class", "bar").R(),
				),
			),
			e("div", "class", "form-inline").R(
				e("div", "class", "form-group").R(
					e("input", "name", "event_date", "type", "date", "value", evt.EventDate).R(),
					e("label", "class", "control-label", "for", "event_date").R("Event Date"),
					//e("i", "class", "bar").R(),
				),
				e("div", "class", "form-group").R(
					e("input", "name", "event_time", "type", "time", "value", evt.EventTime).R(),
					e("label", "class", "control-label", "for", "event_time").R("Event Time"),
					//e("i", "class", "bar").R(),
				),
			),
			e("div", "class", "form-inline").R(
				e("div", "class", "form-group").R(
					e("input", "name", "contact_person", "type", "text", "placeholder", "(optional)", "value",
						evt.ContactPerson).R(),
					e("label", "class", "control-label", "for", "contact_person").R("Contact Person"),
					e("i", "class", "bar").R(),
				),
				e("div", "class", "form-group").R(
					e("input", "name", "categories", "type", "text", "value", strings.Join(evt.Categories, ", "),
						"placeholder", "(optional)").R(),
					e("label", "class", "control-label", "for", "categories").R("Tags (comma separated)"),
					e("i", "class", "bar").R(),
				),
			),
			e("div", "class", "form-inline").R(
				e("div", "class", "form-group").R(
					e("input", "name", "contact_email", "type", "text", "value", evt.ContactEmail).R(),
					e("label", "class", "control-label", "for", "contact_email").R("Contact Email"),
					e("i", "class", "bar").R(),
				),
				e("div", "class", "form-group").R(
					e("input", "name", "contact_url", "type", "text", "placeholder", "(optional)", "value",
						evt.ContactURL).R(),
					e("label", "class", "control-label", "for", "contact_url").R("Contact URL"),
					e("i", "class", "bar").R(),
				),
			),
			e("div", "class", "form-group bootstrap-wrapper").R(
				e("div", "id", "summer1").R(evt.Summary),
				e("textarea", "id", "event_summary", "name", "event_summary", "type", "text", "value", "",
					"style", "display:none").R(),
				e("label", "class", "control-label", "for", "event_summary").R("Summary"),
			),
			e("div", "class", "form-group bootstrap-wrapper").R(
				e("div", "id", "summer2").R(evt.Body),
				e("textarea", "id", "event_body", "name", "event_body", "type", "text", "value", "",
					"style", "display:none").R(),
				e("label", "class", "control-label", "for", "event_body").R("Event Body"),
			),

			e("div", "class", "checkbox").R(
				e("label").R(
					published.R(),
					e("i", "class", "helper").R(),
					"Published",
				),
				e("i", "class", "bar").R(),
			),

			e("div", "class", "form-group").R(
				e("input", "type", "submit", "class", "button", "value", operation).R(),
			),
		),

		//e("div", "id", "react-app").R(),
		e("script", "type", "text/javascript").R(
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

	return out
}
