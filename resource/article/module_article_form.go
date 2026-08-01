package article

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

const ModuleTypeArticleForm = "article_form"

type ModuleArticleForm struct {
	module.Presenter
	csrf string
}

func NewModuleArticleForm(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleArticleForm)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

func (m ModuleArticleForm) getData() (artPres Presenter, err error) {
	dbH, err := db.Db()
	if err != nil {
		return artPres, serr.Wrap(err, "Could not obtain DB handle")
	}
	art, err := findArticleById(dbH, m.Opts.ItemIds[0]) // len check safety on caller
	if err != nil {
		return artPres, serr.Wrap(err, "Unable to obtain article")
	}
	return presenterFromModel(art), nil
}

func (m *ModuleArticleForm) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	art := Presenter{}
	var err error

	operation := "Create"
	action := ""
	if len(m.Opts.ItemIds) > 0 {
		operation = "Update"
		art, err = m.getData()
		if err != nil {
			LogErr(err, "Error in module render", "module options", fmt.Sprintf("%#v", m.Opts))
			return ""
		}
		action = "/update/" + art.Id
	}
	b := element.NewBuilder()

	b.DivClass("af-wrap").R(
		b.H3("class", "af-page-title").T(operation+" "+m.Name.Singular),
		b.Form("method", "post", "action",
			"/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "name", "article_id", "value", art.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Article"),
				b.DivClass("af-field").R(
					b.Label("for", "article_title").R(
						b.T("Article Title "), b.SpanClass("af-req").T("*"),
					),
					b.Input("name", "article_title", "id", "article_title", "type", "text",
						"required", "required", "value", art.Title),
				),
				b.DivClass("af-editor bootstrap-wrapper", "style", "margin-top:1rem").R(
					b.Label("for", "article_summary").T("Summary / Intro"),
					b.Div("id", "summer1").T(art.Summary),
					b.TextArea("id", "article_summary", "name", "article_summary", "type", "text", "value", "",
						"style", "display:none").T(""), // this will hold the returned editor contents
				),
				b.DivClass("af-editor bootstrap-wrapper").R(
					b.Label("for", "article_body").T("Article Body"),
					b.Div("id", "summer2").T(art.Body),
					b.TextArea("id", "article_body", "name", "article_body", "type", "text", "value", "",
						"style", "display:none").T(""),
				),
				b.DivClass("af-field").R(
					b.Label("for", "categories").R(
						b.T("Categories "), b.SpanClass("af-opt").T("(comma separated)"),
					),
					b.Input("type", "text", "name", "categories", "id", "categories",
						"placeholder", "e.g. news, youth, missions",
						"value", strings.Join(art.Categories, ", ")),
				),
			),

			b.DivClass("af-footer").R(
				b.LabelClass("af-switch").R(
					b.Wrap(func() {
						if art.Published {
							b.Input("type", "checkbox", "name", "published", "checked", "checked")
						} else {
							b.Input("type", "checkbox", "name", "published")
						}
					}),
					b.SpanClass("af-slider").T(""),
					b.SpanClass("af-switch-text").T("Published"),
				),
				b.Input("type", "submit", "class", "af-submit", "value", operation),
			),
		),

		b.Script("type", "text/javascript").T(
			`$(document).ready(function(){$('#summer1').summernote(); $('#summer2').summernote();});
			function preSubmit() {
				var s1 = $('#summer1');
				var s2 = $('#summer2');
				var summary = document.getElementById("article_summary");
				var body = document.getElementById("article_body");
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
