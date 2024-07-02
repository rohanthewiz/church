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
	e := b.E

	// Prep some vars
	published := e("input", "type", "checkbox", "name", "published")
	if pg.Published {
		published.AddAttributes("checked", "checked")
	}
	isAdmin := e("input", "type", "checkbox", "name", "is_admin")
	if pg.IsAdmin {
		isAdmin.AddAttributes("checked", "checked")
	}

	moduleByts, err := json.Marshal(pg.Modules)
	if err != nil {
		logger.LogErrAsync(err, "Error marshalling modules for page form", "modules", fmt.Sprintf("%#v", pg.Modules))
		return "page error - try again or contact the site administrator"
	}
	avModTypes := availableModuleTypes()
	moduleTypesByts, err := json.Marshal(avModTypes)
	if err != nil {
		logger.LogErrAsync(err, "Error marshalling available module types", "available_module_types", strings.Join(avModTypes, ","))
		return "page error - try again or contact the site administrator"
	}
	moduleContentBys, err := json.Marshal(moduleContentBy)
	if err != nil {
		logger.LogErrAsync(err, "Error marshalling module content bys")
		return "page error - try again or contact the site administrator"
	}

	e("div", "class", "wrapper-material-form").R(
		e("h3", "class", "page-title").R(operation+" "+m.Name.Singular),
		e("form", "id", "page_form", "method", "post", "action", "/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			e("input", "type", "hidden", "id", "modules", "name", "modules", "value", "").R(),
			e("input", "type", "hidden", "name", "page_id", "value", pg.Id).R(),
			e("input", "type", "hidden", "name", "csrf", "value", m.csrf).R(),

			e("div", "class", "form-inner").R(
				e("div", "class", "form-inline").R(
					e("div", "class", "form-group").R(
						e("input", "name", "page_title", "type", "text", "value", pg.Title).R(),
						e("label", "class", "control-label", "for", "page_title").R("Page Title"),
						e("i", "class", "bar").R(),
					),
					e("div", "class", "form-group").R(
						e("input", "class", "form-group__slug", "name", "page_slug", "type", "text",
							"placeholder", "will be automatically filled in", "value", pg.Slug).R(),
						e("label", "class", "control-label form-group__label--disabled", "for", "page_slug").R("Page Slug (identifier)"),
						e("i", "class", "bar").R(),
					),
				),
				e("div", "class", "form-group").R(
					e("input", "name", "available_positions", "type", "text", "placeholder", "combo of left,right,center - must include center",
						"value", strings.Join(pg.AvailablePositions, ",")).R(),
					e("label", "class", "control-label", "for", "available_positions").R("Available Column Positions"),
					e("i", "class", "bar").R(),
				),
				e("div", "class", "form-inline").R(
					e("div", "class", "checkbox").R(
						e("label").R(
							published.R(),
							e("i", "class", "helper").R(),
							"Publish Page",
						),
						e("i", "class", "bar").R(),
					),
				),
				e("div", "class", "form-inline").R(
					e("div", "class", "form-group").R(
						e("h3").R("Modules (page components)"),
					),
					e("button", "class", "btn-add-module", "title", "Add Module").R("+"),
				),
			), // end form-inner

			e("div", "class", "form-group").R(
				e("input", "type", "submit", "class", "button", "value", operation).R(),
			),
		),
		e("script", "type", "text/javascript").R(
			"var modules = JSON.parse(`"+string(moduleByts)+"`);",
			"var moduleTypes = JSON.parse(`"+string(moduleTypesByts)+"`);",
			"var contentBys = JSON.parse(`"+string(moduleContentBys)+"`);",
			packed.ModulePageForm_js,
		),
	)

	return b.S()
}
