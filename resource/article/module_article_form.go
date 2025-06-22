package article

import (
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
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
	art, err := findArticleById(m.Opts.ItemIds[0]) // len check safety on caller
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

	b.DivClass("wrapper-material-form").R(
		b.H3("class", "page-title").T(operation+" "+m.Name.Singular),
		b.Form("method", "post", "action",
			"/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "name", "article_id", "value", art.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),

			b.DivClass("form-group").R(
				b.Input("name", "article_title", "type", "text",
					"required", "required", "value", art.Title), // we are using 'required' here to drive `input:valid` selector
				b.Label("class", "control-label", "for", "article_title").T("Article Title"),
				b.IClass("bar").T(""),
			),
			b.DivClass("form-group bootstrap-wrapper").R(
				b.Div("id", "summer1").T(art.Summary),
				b.TextArea("id", "article_summary", "name", "article_summary", "type", "text", "value", "",
					"style", "display:none").T(""), // this will hold the returned editor contents
				b.Label("class", "control-label", "for", "article_summary").T("Summary / Intro"),
				// no bar if content editable //b.IClass("bar"),
			),
			b.DivClass("form-group bootstrap-wrapper").R(
				b.Div("id", "summer2").T(art.Body),
				b.TextArea("id", "article_body", "name", "article_body", "type", "text", "value", "",
					"style", "display:none").T(""),
				b.Label("class", "control-label", "for", "article_body").T("Article Body"),
			),
			b.DivClass("form-group").R(
				b.Input("type", "text", "name", "categories",
					"value", strings.Join(art.Categories, ", ")),
				b.Label("class", "control-label", "for", "categories").T("Categories"),
				b.IClass("bar").T(""),
			),
			b.DivClass("checkbox").R(
				b.Label().R(
					b.Wrap(func() {
						if art.Published {
							b.Input("type", "checkbox", "class", "enabled", "name", "published", "checked", "checked")
						} else {
							b.Input("type", "checkbox", "class", "enabled", "name", "published")
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
