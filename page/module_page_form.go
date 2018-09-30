package page

import (
	"fmt"
	"github.com/rohanthewiz/serr"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/app"
	"encoding/json"
	"strings"
	"github.com/rohanthewiz/element"
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
		return Presenter{}, serr.Wrap(err, "Unable to obtain page with id: " + fmt.Sprintf("%d", m.Opts.ItemIds[0]))
	}
	return presenterFromModel(pg)
}

func (m *ModulePageForm) Render(params map[string]map[string]string, loggedIn bool) string {
	//fmt.Printf("*|* Params: %#v\n", params)
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	pg := Presenter{}; var err error

	operation := "Create"
	action := ""
	if len(m.Opts.ItemIds) != 0 {
		operation = "Update"
		pg, err = m.getData()
		if err != nil {
			logger.LogErr(err, "Error in module render", "module_options", fmt.Sprintf("%#v", m.Opts))
			return ""  // todo - error presentation to user
		}
		logger.LogAsync("Debug", "Existing page object for module form", "page object", fmt.Sprintf("%#v", pg))
		println("|* Page has:", len(pg.Modules), "modules")
		action = "/update/" + pg.Id
	}

	e := element.New
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

	out := e("div", "class", "wrapper-material-form").R(
		e("h3", "class", "page-title").R(operation + " " + m.Name.Singular),
		e("form", "id", "page_form", "method", "post", "action", "/admin/" + m.Name.Plural + action, "onSubmit", "return preSubmit();").R(
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
		"var modules = JSON.parse(`" + string(moduleByts) + "`);" +
		"var moduleTypes = JSON.parse(`" + string(moduleTypesByts) + "`);" +
		"var contentBys = JSON.parse(`" + string(moduleContentBys) + "`);" +

		`var newModule = {
			opts: {
				layout_column: "center", published: true, main_module: false,
				title: "", slug: "", module_type: "article_single",
				items_url_path: "", item_ids: [], is_admin: false,
				show_unpublished: false, ascending: false, condition: "", limit: 5, offset: 0
			}
		};

		function preSubmit() {
			$('#modules').val($('#page_form').serializeJSON());
			//console.log($('#modules').get(0).value);
			return true;
		}

		$(document).ready(function() {
			//var wrapper = $("#page_form"); //Fields wrapper
			var inner = $(".form-inner");
			var add_button = $("#page_form .btn-add-module"); //Add button ID
			var count = 0;
			var max_components = 16; //maximum components allowed

			// Initial Components
			if(modules) {
				//console.log(modules);  // ***debug***
				for (var i = 0; i < modules.length; i++) {
					$(inner).append(buildComponent(i, modules[i])); //add input box
				}
			}
			// Can Add
			$(add_button).click(function (e) { //on add input button click
				e.preventDefault();
				if (count < max_components) {
					$(inner).append(buildComponent($('.module').length, newModule)); //add input box
				}
			});
			// Serialize
			//$(wrapper).on("click","#btn_serialize", function(e){
			//    e.preventDefault();
			//    console.log($(wrapper).serializeJSON()); // ..izeObject();
			//});

			// Remove
			$(inner).on("click",".remove_field", function(e){
				e.preventDefault();
				$(this).closest('.module').remove();
				count--;
				reorderItems();
			});
			// Move Up
			$(inner).on("click",".move_up", function(e){
				e.preventDefault();
				var parent = $(this).closest('.module');
				parent.insertBefore(parent.prev('.module'));
				reorderItems();
			});
			// Move Down
			$(inner).on("click",".move_down", function(e){
				e.preventDefault();
				var parent = $(this).closest('.module');
				parent.insertAfter(parent.next('.module'));
				reorderItems();
			});
			// ModuleType Selection Change - Todo at document ready also
			$(inner).on("change", ".module_type", function(e){
				e.preventDefault();
				var contentBy = contentBys[this.value];
				//console.log(this.value);
				// remove any disabled attrs // not necessary on startup
				$(this).closest('.module').find('.can-disable').each(function(i){
					$(this).prop("disabled", false);
				});
				// add disabled attrs depending on contentBy MultiId or SingleId
				if (contentBy == "SingleId" || contentBy == "MultiId") {
					$(this).closest('.module').find('.by-list').each(function(i){
						$(this).prop("disabled", true);
					});
				} else {
					$(this).closest('.module').find('.by-id').first().prop("disabled", true);
				}
			});
		});

		function buildComponent(x, mod) {
			var ids = "";
			if (mod.opts.item_ids) {
				ids = mod.opts.item_ids.join(',');
			}
			return $('<div class="module">' +
				'<div class="form-pack">' +
				'<div class="form-group"><label>Choose Module Type</label><select class="module_type" name="mods[' + x + '][module_type]" value="' + mod.opts.module_type + '">' + buildModuleTypeOptions(mod.opts.module_type) + '</select><i class="bar"></i></div>' +
				'<div class="form-group"><label>Module Title</label>&nbsp;<input type="text" placeholder="title" name="mods[' + x + '][title]" value="' + mod.opts.title + '" /><i class="bar"></i></div>' +
				'<div class="form-group btn-group"><button class="btn move_up" title="Move row up">Up</button><button class="btn move_down" title="Move row down">Down</button>' + '<button class="remove_field">Delete</button></div>' +
				'</div><div class="form-pack">' +
				'<div class="form-group"><label>Column Position (e.g. center)</label>&nbsp;<input type="text" placeholder="layout_column" name="mods[' + x + '][layout_column]" value="' + mod.opts.layout_column + '"><i class="bar"></i></div>' +
				'<div class="form-group"><label>Item Ids</label>&nbsp;<input class="by-id can-disable" type="text"' + ((contentBys[mod.opts.module_type] != "SingleId" && contentBys[mod.opts.module_type] != "MultiId") ? ' disabled="disabled"' : '')  + '  placeholder="Item id(s)" name="mods[' + x + '][item_ids]" value="' + ids + '"><i class="bar"></i></div>' +
				'</div><div class="form-pack">' +
				'<div class="form-group"><label>Number of Items to List</label>&nbsp;<input class="by-list can-disable" type="text"' + ((contentBys[mod.opts.module_type] == "SingleId" || contentBys[mod.opts.module_type] == "MultiId") ? ' disabled="disabled"' : '')  + ' placeholder="limit" name="mods[' + x + '][limit]" value="' + mod.opts.limit + '"><i class="bar"></i></div>' +
				'<div class="form-group"><label>Number of Items to Skip</label>&nbsp;<input class="by-list can-disable" type="text"' + ((contentBys[mod.opts.module_type] == "SingleId" || contentBys[mod.opts.module_type] == "MultiId") ? ' disabled="disabled"' : '')  + ' placeholder="offset" name="mods[' + x + '][offset]" value="' + mod.opts.offset + '"><i class="bar"></i></div>' +
				'</div><div class="form-pack">' +
				//checkbox('Admin', 'is_admin', x, mod.opts.is_admin) +
				checkbox('Published', 'published', x, mod.opts.published) +
				checkbox('Main Module (Only one module should be the Main)', 'main_module', x, mod.opts.is_main_module) +
				checkbox('Show Unpublished', 'show_unpublished', x, mod.opts.show_unpublished) +
				checkbox('Oldest First', 'ascending', x, mod.opts.ascending) +
				'</div></div>');
		}

		function buildModuleTypeOptions(modType) {
			var out = ""
			for(var i = 0; i < moduleTypes.length; i++) {
				out += '<option value="' + moduleTypes[i] + '"'
				if (moduleTypes[i] === modType) { out += ' selected="selected"' }
				out += '>' + moduleTypes[i]
				out += '</option>'
			}
			return out
		}

		function reorderItems() {
			// Remember we need to reorder all types of inputs including selects
			$('.module').each(function(i){
				$(this).find('input[name^=mods]').each(function(){
					m = $(this).attr("name").replace(/mods\[\d+\]/, 'mods[' + i + ']');
					$(this).attr("name", m);
				})
				$(this).find('select[name^=mods]').each(function(){
					m = $(this).attr("name").replace(/mods\[\d+\]/, 'mods[' + i + ']');
					$(this).attr("name", m);
				})
			})
		}

		function checkbox(displayName, fieldName, index, isChecked) {
			var str = hspace() + '<input type="checkbox" name="mods[' + index + '][' + fieldName + ']"';
			if (isChecked) {
				str += ' checked="checked"';
			}
			str += ' /> ' + displayName + hspace();
			return str;
		}

		function hspace() {
			return '&nbsp;&nbsp;'
		}
		`),
	)

	return out
}
