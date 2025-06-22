package sermon

import (
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
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
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	ser := Presenter{}
	var err error

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

	b := element.NewBuilder()

	b.DivClass("wrapper-material-form").R(
		b.H3Class("page-title").T(operation+" "+m.Name.Singular),
		b.Form("method", "post", "enctype", "multipart/form-data", "action",
			"/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "name", "sermon_id", "value", ser.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),
			b.DivClass("form-inline").R(
				b.DivClass("form-group").R(
					b.Input("name", "sermon_title", "type", "text",
						"required", "required", "value", ser.Title), // we are using 'required' here to drive `input:valid` selector
					b.LabelClass("control-label", "for", "sermon_title").T("Sermon Title"),
					b.IClass("bar").R(),
				),
				b.DivClass("form-group").R(
					b.Input("name", "sermon_date", "type", "date", "value", ser.DateTaught), // todo - maual validation
					b.LabelClass("control-label", "for", "sermon_date").T("Sermon Date"),
					// b.I("class", "bar"),
				),
			),
			b.DivClass("form-group bootstrap-wrapper").R(
				b.Div("id", "summer1").T(ser.Summary),
				b.TextArea("id", "sermon_summary", "name", "sermon_summary", "type", "text", "value", "",
					"style", "display:none").R(),
				b.LabelClass("control-label", "for", "sermon_summary").T("Summary"),
			),
			b.DivClass("form-group bootstrap-wrapper").R(
				b.Div("id", "summer2").T(ser.Body),
				b.TextArea("id", "sermon_body", "name", "sermon_body", "type", "text", "value", "",
					"style", "display:none").R(),
				b.LabelClass("control-label", "for", "sermon_body").T("Sermon Body"),
			),
			b.DivClass("form-inline").R(
				b.DivClass("form-group").R(
					b.Input("name", "pastor-teacher", "type", "text",
						"required", "required", "value", ser.Teacher),
					b.LabelClass("control-label", "for", "pastor-teacher").T("Pastor / Teacher"),
					b.IClass("bar").R(),
				),
				b.DivClass("form-group").R(
					b.Input("name", "sermon_place", "type", "text", "placeholder", "(optional)", "value",
						ser.PlaceTaught),
					b.LabelClass("control-label", "for", "sermon_place").T("Place Taught"),
					b.IClass("bar").R(),
				),
			),
			b.DivClass("form-inline").R(
				b.DivClass("form-group").R(
					b.Input("name", "sermon_audio", "type", "file", "value", ""),
					b.LabelClass("control-label", "for", "sermon_audio").T("Upload Audio File"),
					b.IClass("bar").R(),
				),
				b.DivClass("form-group").R( // todo - autogenerate this link
					b.Input("name", "audio_link", "type", "text", "placeholder", "(automatically generated)",
						"value", ser.AudioLink),
					b.LabelClass("control-label", "for", "audio_link").T("Link to Sermon"),
					b.IClass("bar").R(),
				),
			),
			b.DivClass("form-inline").R(
				b.DivClass("form-group").R(
					b.Input("name", "categories", "type", "text", "value", strings.Join(ser.Categories, ", "),
						"placeholder", "(optional)"),
					b.LabelClass("control-label", "for", "categories").T("Tags (comma separated)"),
					b.IClass("bar").R(),
				),
				b.DivClass("form-group").R(
					b.Input("name", "scripture_refs", "type", "text", "value", strings.Join(ser.ScriptureRefs, ", "),
						"placeholder", "(optional)"),
					b.LabelClass("control-label", "for", "scripture_refs").T("Scripture references (comma separated)"),
					b.IClass("bar").R(),
				),
			),
			b.DivClass("form-inline").R(
				b.DivClass("checkbox").R(
					b.Label().R(
						b.Wrap(func() {
							if ser.Published || operation == "Create" {
								b.Input("type", "checkbox", "name", "published", "checked", "checked")
							} else {
								b.Input("type", "checkbox", "name", "published")
							}
						}),
						b.IClass("helper").R(),
						b.T("Published"),
					),
					b.IClass("bar").R(),
				),
				b.DivClass("checkbox").R(
					b.Label().R(
						b.Input("type", "checkbox", "name", "audio-link-ovrd"),
						b.IClass("helper").R(),
						b.T("Audio Link Override (webmaster only)"),
					),
					b.IClass("bar").R(),
				),
			),

			b.DivClass("form-group").R(
				b.InputClass("button", "type", "submit", "value", operation),
			),
		),

		// b.Div("id", "react-app"),
		b.Script("type", "text/javascript").T(
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
			}`),
	)

	return b.String()
}
