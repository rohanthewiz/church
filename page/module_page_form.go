package page

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/pack/packed"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

type ModulePageForm struct {
	module.Presenter
	csrf string
}

const ModuleTypePageForm = "page_form"

func NewModulePageForm(pres module.Presenter) (module.Module, error) {
	mod := new(ModulePageForm)
	mod.Name = pres.Name
	mod.Opts = pres.Opts

	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token.")
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

func (m ModulePageForm) getData() (Presenter, error) {
	pg, err := findPageById(m.Opts.ItemIds[0])
	if err != nil {
		return Presenter{}, serr.Wrap(err, "Unable to obtain page with id: "+fmt.Sprintf("%d", m.Opts.ItemIds[0]))
	}
	return presenterFromModel(pg)
}

func (m *ModulePageForm) Render(params map[string]map[string]string, loggedIn bool) string {
	// fmt.Printf("*|* Params: %#v\n", params)
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	pg := Presenter{}
	var err error

	operation := "Create"
	action := ""
	if len(m.Opts.ItemIds) != 0 {
		operation = "Update"
		pg, err = m.getData()
		if err != nil {
			logger.LogErr(err, "Error in module render", "module_options", fmt.Sprintf("%#v", m.Opts))
			return "" // todo - error presentation to user
		}
		logger.LogAsync("Debug", "Existing page object for module form", "page object", fmt.Sprintf("%#v", pg))
		println("|* Page has:", len(pg.Modules), "modules")
		action = "/update/" + pg.Id
	}

	b := element.NewBuilder()

	moduleByts, err := json.Marshal(pg.Modules)
	if err != nil {
		logger.LogErr(err, "Error marshalling modules for page form", "modules", fmt.Sprintf("%#v", pg.Modules))
		return "page error - try again or contact the site administrator"
	}
	avModTypes := availableModuleTypes()
	moduleTypesByts, err := json.Marshal(avModTypes)
	if err != nil {
		logger.LogErr(err, "Error marshalling available module types", "available_module_types", strings.Join(avModTypes, ","))
		return "page error - try again or contact the site administrator"
	}
	moduleContentBys, err := json.Marshal(moduleContentBy)
	if err != nil {
		logger.LogErr(err, "Error marshalling module content bys")
		return "page error - try again or contact the site administrator"
	}

	b.DivClass("wrapper-material-form").R(
		b.H3("class", "page-title").T(operation+" "+m.Name.Singular),
		b.Form("id", "page_form", "method", "post", "action", "/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "id", "modules", "name", "modules", "value", ""),
			b.Input("type", "hidden", "name", "page_id", "value", pg.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),

			b.DivClass("form-inner").R(
				b.DivClass("form-inline").R(
					b.DivClass("form-group").R(
						b.Input("name", "page_title", "type", "text", "value", pg.Title),
						b.Label("class", "control-label", "for", "page_title").T("Page Title"),
						b.IClass("bar"),
					),
					b.DivClass("form-group").R(
						b.Input("class", "form-group__slug", "name", "page_slug", "type", "text",
							"placeholder", "will be automatically filled in", "value", pg.Slug),
						b.Label("class", "control-label form-group__label--disabled", "for", "page_slug").T("Page Slug (identifier)"),
						b.IClass("bar"),
					),
				),
				b.DivClass("form-group").R(
					b.Input("name", "available_positions", "type", "text", "placeholder", "combo of left,right,center - must include center",
						"value", strings.Join(pg.AvailablePositions, ",")),
					b.Label("class", "control-label", "for", "available_positions").T("Available Column Positions"),
					b.IClass("bar"),
				),
				b.DivClass("form-inline").R(
					b.DivClass("checkbox").R(
						b.Label().R(
							b.Wrap(func() {
								if pg.Published {
									b.Input("type", "checkbox", "name", "published", "checked", "checked")
								} else {
									b.Input("type", "checkbox", "name", "published")
								}
							}),
							b.IClass("helper"),
							b.Text("Publish Page"),
						),
						b.IClass("bar"),
					),
				),
				b.DivClass("form-inline").R(
					b.DivClass("form-group").R(
						b.H3().T("Modules (page components)"),
					),
					b.Button("class", "btn-add-module", "title", "Add Module").T("+"),
				),
			), // end form-inner

			b.DivClass("form-group").R(
				b.Input("type", "submit", "class", "button", "value", operation),
			),
		),
		b.Script("type", "text/javascript").T(
			"var modules = JSON.parse(`"+string(moduleByts)+"`);"+
				"var moduleTypes = JSON.parse(`"+string(moduleTypesByts)+"`);"+
				"var contentBys = JSON.parse(`"+string(moduleContentBys)+"`);"+
				packed.ModulePageForm_js),
	)

	return b.String()
}
