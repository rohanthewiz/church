package sermon

import (
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/db"
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
	dbH, err := db.Db()
	if err != nil {
		return pres, serr.Wrap(err, "Could not obtain DB handle")
	}
	ser, err := findSermonById(dbH, m.Opts.ItemIds[0])
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

	b.DivClass("af-wrap").R(
		b.H3Class("af-page-title").T(operation+" "+m.Name.Singular),
		b.Form("method", "post", "enctype", "multipart/form-data", "action",
			"/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "name", "sermon_id", "value", ser.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Sermon Details"),
				b.DivClass("af-row").R(
					b.DivClass("af-field").R(
						b.Label("for", "sermon_title").R(
							b.T("Sermon Title "), b.SpanClass("af-req").T("*"),
						),
						b.Input("name", "sermon_title", "id", "sermon_title", "type", "text",
							"required", "required", "value", ser.Title),
					),
					b.DivClass("af-field").R(
						b.Label("for", "sermon_date").R(
							b.T("Sermon Date "), b.SpanClass("af-req").T("*"),
						),
						// required is real validation here: an empty date used to
						// reach time.Parse server-side and blow away the whole form
						b.Input("name", "sermon_date", "id", "sermon_date", "type", "date",
							"required", "required", "value", ser.DateTaught),
					),
				),
				b.DivClass("af-row", "style", "margin-top:0.9rem").R(
					b.DivClass("af-field").R(
						b.Label("for", "pastor-teacher").R(
							b.T("Pastor / Teacher "), b.SpanClass("af-req").T("*"),
						),
						b.Input("name", "pastor-teacher", "id", "pastor-teacher", "type", "text",
							"required", "required", "value", ser.Teacher),
					),
					b.DivClass("af-field").R(
						b.Label("for", "sermon_place").R(
							b.T("Place Taught "), b.SpanClass("af-opt").T("(optional)"),
						),
						b.Input("name", "sermon_place", "id", "sermon_place", "type", "text",
							"value", ser.PlaceTaught),
					),
				),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Audio"),
				b.DivClass("af-row").R(
					b.DivClass("af-field").R(
						b.Label("for", "sermon_audio").T("Upload Audio File"),
						b.Input("name", "sermon_audio", "id", "sermon_audio", "type", "file",
							"accept", "audio/*", "value", ""),
						b.PClass("af-help").T("After saving, the recording uploads to cloud storage in the background — it can take a minute to become playable."),
					),
					b.DivClass("af-field").R(
						b.Label("for", "audio_link").T("Link to Sermon"),
						b.Input("name", "audio_link", "id", "audio_link", "type", "text",
							"placeholder", "(automatically generated)", "value", ser.AudioLink),
						b.PClass("af-help").T("Generated from the upload. Edits here are ignored unless \"Use custom audio link\" below is on."),
					),
				),
				b.LabelClass("af-switch", "style", "margin-top:0.8rem").R(
					b.Input("type", "checkbox", "name", "audio-link-ovrd"),
					b.SpanClass("af-slider").T(""),
					b.SpanClass("af-switch-text").T("Use custom audio link (webmaster only)"),
				),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Content"),
				b.DivClass("af-editor bootstrap-wrapper").R(
					b.Label("for", "sermon_summary").T("Summary"),
					b.Div("id", "summer1").T(ser.Summary),
					b.TextArea("id", "sermon_summary", "name", "sermon_summary", "type", "text", "value", "",
						"style", "display:none").R(),
				),
				b.DivClass("af-editor bootstrap-wrapper").R(
					b.Label("for", "sermon_body").T("Sermon Body"),
					b.Div("id", "summer2").T(ser.Body),
					b.TextArea("id", "sermon_body", "name", "sermon_body", "type", "text", "value", "",
						"style", "display:none").R(),
				),
				b.DivClass("af-row").R(
					b.DivClass("af-field").R(
						b.Label("for", "categories").R(
							b.T("Tags "), b.SpanClass("af-opt").T("(comma separated, optional)"),
						),
						b.Input("name", "categories", "id", "categories", "type", "text",
							"value", strings.Join(ser.Categories, ", "),
							"placeholder", "e.g. faith, prayer"),
					),
					b.DivClass("af-field").R(
						b.Label("for", "scripture_refs").R(
							b.T("Scripture References "), b.SpanClass("af-opt").T("(comma separated, optional)"),
						),
						b.Input("name", "scripture_refs", "id", "scripture_refs", "type", "text",
							"value", strings.Join(ser.ScriptureRefs, ", "),
							"placeholder", "e.g. John 3:16, Rom 8:28-30"),
					),
				),
			),

			b.DivClass("af-footer").R(
				b.LabelClass("af-switch").R(
					b.Wrap(func() {
						if ser.Published || operation == "Create" {
							b.Input("type", "checkbox", "name", "published", "checked", "checked")
						} else {
							b.Input("type", "checkbox", "name", "published")
						}
					}),
					b.SpanClass("af-slider").T(""),
					b.SpanClass("af-switch-text").T("Published"),
				),
				b.InputClass("af-submit", "type", "submit", "value", operation),
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
