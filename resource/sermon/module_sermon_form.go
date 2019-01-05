package sermon

import (
	"fmt"
	"strings"
	"github.com/rohanthewiz/serr"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/element"
)

const ModuleTypeSermonForm = "sermon_form"

type ModuleSermonForm struct {
	module.Presenter
	csrf string
}

// Sermon Form deals with only a single item referenced in ItemIds[0] or a new one otherwise
func NewModuleSermonForm(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleSermonForm)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token.")
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

func (m ModuleSermonForm) getData() (pres Presenter, err error) {
	ser, err := findSermonById(m.Opts.ItemIds[0])
	if err != nil {
		return pres, serr.Wrap(err, "Unable to obtain sermon", "id", fmt.Sprintf("%d", m.Opts.ItemIds[0]))
	}
	return presenterFromModel(ser), nil
}

func (m *ModuleSermonForm) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to us
		m.SetId(opts)
	}
	ser := Presenter{}; var err error

	operation := "Create"
	action := ""
	if len(m.Opts.ItemIds) > 0 {
		operation = "Update"
		ser, err = m.getData()
		if err != nil {
			LogErr(err, "Error in module render")
			return ""
		}
		action = "/update/" + ser.Id
	}

	e := element.New

	published := e("input", "type", "checkbox", "name", "published")
	if ser.Published {
		published.AddAttributes("checked", "checked")
	}
	audioLinkOvrd := e("input", "type", "checkbox", "name", "audio-link-ovrd")

	out := e("div", "class", "wrapper-material-form").R(
		e("h3", "class", "page-title").R(operation + " " + m.Name.Singular),
		e("form", "method", "post", "enctype", "multipart/form-data", "action",
			"/admin/" + m.Name.Plural + action, "onSubmit", "return preSubmit();").R(
			e("input", "type", "hidden", "name", "sermon_id", "value", ser.Id).R(),
			e("input", "type", "hidden", "name", "csrf", "value", m.csrf).R(),
			e("div", "class", "form-inline").R(
				e("div", "class", "form-group").R(
					e("input", "name", "sermon_title", "type", "text",
						"required", "required", "value", ser.Title).R(),  // we are using 'required' here to drive `input:valid` selector
					e("label", "class", "control-label", "for", "sermon_title").R("Sermon Title"),
					e("i", "class", "bar").R(),
				),
				e("div", "class", "form-group").R(
					e("input", "name", "sermon_date", "type", "date", "value", ser.DateTaught).R(), // todo - maual validation
					e("label", "class", "control-label", "for", "sermon_date").R("Sermon Date"),
					//e("i", "class", "bar").R(),
				),
			),
			e("div", "class", "form-group bootstrap-wrapper").R(
				e("div", "id", "summer1").R(ser.Summary),
				e("textarea", "id", "sermon_summary", "name", "sermon_summary", "type", "text", "value", "",
					"style", "display:none").R(),
				e("label", "class", "control-label", "for", "sermon_summary").R("Summary"),
			),
			e("div", "class", "form-group bootstrap-wrapper").R(
				e("div", "id", "summer2").R(ser.Body),
				e("textarea", "id", "sermon_body", "name", "sermon_body", "type", "text", "value", "",
					"style", "display:none").R(),
				e("label", "class", "control-label", "for", "sermon_body").R("Sermon Body"),
			),
			e("div", "class", "form-inline").R(
				e("div", "class", "form-group").R(
					e("input", "name", "pastor-teacher", "type", "text",
						"required", "required", "value", ser.Teacher).R(),
					e("label", "class", "control-label", "for", "pastor-teacher").R("Pastor / Teacher"),
					e("i", "class", "bar").R(),
				),
				e("div", "class", "form-group").R(
					e("input", "name", "sermon_place", "type", "text", "placeholder", "(optional)", "value",
							ser.PlaceTaught).R(),
					e("label", "class", "control-label", "for", "sermon_place").R("Place Taught"),
					e("i", "class", "bar").R(),
				),
			),
			e("div", "class", "form-inline").R(
				e("div", "class", "form-group").R(
					e("input", "name", "sermon_audio", "type", "file", "value", "").R(),
					e("label", "class", "control-label", "for", "sermon_audio").R("Upload Audio File"),
					e("i", "class", "bar").R(),
				),
				e("div", "class", "form-group").R( // todo - autogenerate this link
					e("input", "name", "audio_link", "type", "text", "placeholder", "(automatically generated)",
							"value", ser.AudioLink).R(),
					e("label", "class", "control-label", "for", "audio_link",
							).R("Link to Sermon"),
					e("i", "class", "bar").R(),
				),
			),
			e("div", "class", "form-inline").R(
				e("div", "class", "form-group").R(
					e("input", "name", "categories", "type", "text", "value", strings.Join(ser.Categories, ", "),
						"placeholder", "(optional)").R(),
					e("label", "class", "control-label", "for", "categories").R("Tags (comma separated)"),
					e("i", "class", "bar").R(),
				),
				e("div", "class", "form-group").R(
					e("input", "name", "scripture_refs", "type", "text", "value", strings.Join(ser.ScriptureRefs, ", "),
						"placeholder", "(optional)").R(),
					e("label", "class", "control-label", "for", "scripture_refs").
							R("Scripture references (comma separated)"),
					e("i", "class", "bar").R(),
				),
			),
			e("div", "class", "form-inline").R(
    			e("div", "class", "checkbox").R(
    				e("label").R(
    					published.R(),
    					e("i", "class", "helper").R(),
    					"Published",
    				),
    				e("i", "class", "bar").R(),
    			),
    			e("div", "class", "checkbox").R(
    				e("label").R(
    					audioLinkOvrd.R(),
    					e("i", "class", "helper").R(),
    					"Audio Link Override (webmaster only)",
    				),
    				e("i", "class", "bar").R(),
    			),
			),

			e("div", "class", "form-group").R(
				e("input", "type", "submit", "class", "button", "value", operation).R(),
			),
		),

		//e("div", "id", "react-app").R(),
		e("script", "type", "text/javascript").R(
			`$(document).ready(function(){$('#summer1').summernote(); $('#summer2').summernote();});
			function preSubmit() {
				var s1 = $('#summer1');
				var s2 = $('#summer2');
				var summary = document.getElementById("sermon_summary");
				var body = document.getElementById("sermon_body");
				if (s1 && summary) {
					summary.innerHTML = s1.summernote('code');
				}
				if (s2 && body) {
					body.innerHTML = s2.summernote('code');
				}
				return true;
			}`,
		),
	)
	return out
}