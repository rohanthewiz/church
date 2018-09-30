package article

import (
	"fmt"
	"strings"
	"github.com/rohanthewiz/serr"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/element"
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
		return nil, serr.Wrap(err, "Could not generate form token", "location", FunctionLoc())
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

func (m ModuleArticleForm) getData() (artPres Presenter, err error) {
	art, err := findArticleById(m.Opts.ItemIds[0])  // len check safety on caller
	if err != nil {
		return artPres, serr.Wrap(err, "Unable to obtain article")
	}
	return presenterFromModel(art), nil
}

func (m *ModuleArticleForm) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to us
		m.SetId(opts)
	}
	art := Presenter{}; var err error

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
	e := element.New
	elEnabled := e("input", "type", "checkbox", "class", "enabled", "name", "published")
	if art.Published {
		elEnabled.AddAttributes("checked", "checked")
	}
	out := e("div", "class", "wrapper-material-form").R(
		e("h3", "class", "page-title").R(operation + " " + m.Name.Singular),
		e("form", "method", "post", "action",
			"/admin/" + m.Name.Plural + action, "onSubmit", "return preSubmit();").R(
			e("input", "type", "hidden", "name", "article_id", "value", art.Id).R(),
			e("input", "type", "hidden", "name", "csrf", "value", m.csrf).R(),

			e("div", "class", "form-group").R(
				e("input", "name", "article_title", "type", "text",
					"required", "required", "value", art.Title).R(),  // we are using 'required' here to drive `input:valid` selector
				e("label", "class", "control-label", "for", "article_title").R("Article Title"),
				e("i", "class", "bar").R(),
			),
			e("div", "class", "form-group bootstrap-wrapper").R(
				e("div", "id", "summer1").R(art.Summary),
				e("textarea", "id", "article_summary", "name", "article_summary", "type", "text", "value", "",
					"style", "display:none").R(), // this will hold the returned editor contents
				e("label", "class", "control-label", "for", "article_summary").R("Summary / Intro"),
				// no bar if content editable //e("i", "class", "bar").R(),
			),
			e("div", "class", "form-group bootstrap-wrapper").R(
				e("div", "id", "summer2").R(art.Body),
				e("textarea", "id", "article_body", "name", "article_body", "type", "text", "value", "",
					"style", "display:none").R(),
				e("label", "class", "control-label", "for", "article_body").R("Article Body"),
			),
			e("div", "class", "form-group").R(
				e("input", "type", "text", "name", "categories",
					"value", strings.Join(art.Categories, ", ")).R(),
				e("label", "class", "control-label", "for", "categories").R("Categories"),
				e("i", "class", "bar").R(),
			),
			e("div", "class", "checkbox").R(
				e("label").R(
					elEnabled.R(),
					e("i", "class", "helper").R(),
					"Published",
				),
				e("i", "class", "bar").R(),
			),

			e("div", "class", "form-group").R(
				e("input", "type", "submit", "class", "button", "value", operation).R(),
			),
		),

		e("script", "type", "text/javascript").R(
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
			}`,
		),
	)
	return out
}
