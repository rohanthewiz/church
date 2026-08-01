package page

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/db"
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
	dbH, err := db.Db()
	if err != nil {
		return Presenter{}, serr.Wrap(err, "Could not obtain DB handle")
	}
	pg, err := findPageById(dbH, m.Opts.ItemIds[0])
	if err != nil {
		return Presenter{}, serr.Wrap(err, "Unable to obtain page with id: "+fmt.Sprintf("%d", m.Opts.ItemIds[0]))
	}
	return presenterFromModel(pg)
}

// Page-form-only styling on top of the shared admin stylesheet
// (template/admin_css.go). Everything colorful resolves through the --af-*
// custom properties so a site theme that overrides those re-skins this form
// too. Only structural rules live here: the module-card repeater chrome and
// the switch strip.
const pageFormCSS = `
.pf-module { border: 1px solid var(--af-inset-border); border-radius: 6px;
	background: var(--af-inset-bg); margin-bottom: 0.8rem; min-width: 0; }
.pf-module__head { display: flex; align-items: center; gap: 0.6rem;
	justify-content: space-between; padding: 0.45rem 0.7rem;
	border-bottom: 1px solid var(--af-inset-border); }
.pf-module__summary { font-weight: 600; font-size: 0.9rem;
	color: var(--af-text-soft); text-transform: capitalize; min-width: 0;
	overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pf-module__tools { display: flex; gap: 0.3rem; flex: none; }
.pf-module__tools .af-btn { padding: 0.15rem 0.55rem; font-size: 0.95rem; }
.pf-module__body { padding: 0.7rem 0.7rem 0.8rem; }
.pf-module__body > .af-row { margin-bottom: 0.7rem; }
.pf-switches { display: flex; flex-wrap: wrap; gap: 0.5rem 1.4rem; }
.pf-pos { display: flex; flex-wrap: wrap; gap: 0.5rem 1.4rem; margin-top: 0.3rem; }
`

func (m *ModulePageForm) Render(params map[string]map[string]string, loggedIn bool) string {
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
		logger.DebugF("Page %q has: %d module(s)", pg.Title, len(pg.Modules))
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

	// The stored value is a comma list like "left,center,right"; the form
	// offers left/right as switches (center is always present — it is the main
	// column) and JS maintains the hidden canonical value.
	hasLeft, hasRight := false, false
	for _, p := range pg.AvailablePositions {
		switch strings.TrimSpace(p) {
		case "left":
			hasLeft = true
		case "right":
			hasRight = true
		}
	}

	posSwitch := func(label, value string, on bool) {
		b.LabelClass("af-switch").R(
			b.Wrap(func() {
				if on {
					b.Input("type", "checkbox", "value", value, "checked", "checked")
				} else {
					b.Input("type", "checkbox", "value", value)
				}
			}),
			b.SpanClass("af-slider").T(""),
			b.SpanClass("af-switch-text").T(label),
		)
	}

	namedSwitch := func(label, name string, on bool) {
		b.LabelClass("af-switch").R(
			b.Wrap(func() {
				if on {
					b.Input("type", "checkbox", "name", name, "checked", "checked")
				} else {
					b.Input("type", "checkbox", "name", name)
				}
			}),
			b.SpanClass("af-slider").T(""),
			b.SpanClass("af-switch-text").T(label),
		)
	}

	b.DivClass("af-wrap").R(
		b.Style().T(pageFormCSS),
		b.H3("class", "af-page-title").T(operation+" "+m.Name.Singular),
		b.Form("id", "page_form", "method", "post", "action", "/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "id", "modules", "name", "modules", "value", ""),
			b.Input("type", "hidden", "name", "page_id", "value", pg.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),
			b.Input("type", "hidden", "id", "available_positions", "name", "available_positions",
				"value", strings.Join(pg.AvailablePositions, ",")),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Page Details"),
				b.DivClass("af-row").R(
					b.DivClass("af-field").R(
						b.Label("for", "page_title").R(
							b.T("Page Title "), b.SpanClass("af-req").T("*"),
						),
						b.Input("name", "page_title", "id", "page_title", "type", "text",
							"required", "required", "value", pg.Title),
					),
					b.DivClass("af-field").R(
						b.Label("for", "page_slug").R(
							b.T("Page Slug "), b.SpanClass("af-opt").T("(auto-generated)"),
						),
						b.Input("name", "page_slug", "id", "page_slug", "type", "text",
							"placeholder", "will be automatically filled in", "value", pg.Slug),
					),
				),
				b.DivClass("af-field", "style", "margin-top:0.8rem").R(
					b.Label().T("Side Columns"),
					b.DivClass("pf-pos").R(
						b.Wrap(func() {
							posSwitch("Left column", "left", hasLeft)
							posSwitch("Right column", "right", hasRight)
						}),
					),
					b.PClass("af-help").T("The center (main) column is always present. Toggle side columns to give modules more positions."),
				),
				b.DivClass("pf-switches", "style", "margin-top:0.8rem").R(
					b.Wrap(func() {
						namedSwitch("Publish Page", "published", pg.Published)
						namedSwitch("Make this the Home Page", "is_home", pg.IsHome)
					}),
				),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").R(
					b.Span().T("Modules (page components)"),
					b.Span().R(
						b.Span("id", "pf_count", "style", "font-weight:400;margin-right:0.8rem").T(""),
						b.Button("id", "pf_add_module", "type", "button", "class", "af-btn af-btn--primary",
							"title", "Add a module to this page").T("+ Add Module"),
					),
				),
				b.PClass("af-help", "id", "pf_empty").T("No modules yet — a page renders its modules, so add at least one."),
				b.Div("id", "pf_modules").R(),
			),

			b.DivClass("af-footer").R(
				b.AClass("af-btn", "href", "/admin/"+m.Name.Plural).T("Cancel"),
				b.Input("type", "submit", "class", "af-submit", "value", operation),
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
