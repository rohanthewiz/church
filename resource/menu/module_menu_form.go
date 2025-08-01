package menu

import (
	"encoding/json"
	"fmt"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

type ModuleMenuForm struct {
	module.Presenter
	csrf string
}

const ModuleTypeMenuForm = "menu_form"

// Menu Form deals with only a single item referenced in ItemIds[0] or a new one otherwise
func NewModuleMenuForm(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleMenuForm)
	mod.Name = pres.Name
	mod.Opts = pres.Opts

	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token.")
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

// Since this is only called from Render(), so safeties are in Render()
func (m ModuleMenuForm) getData() (mdef MenuDef, err error) {
	mnu, err := findModelById(m.Opts.ItemIds[0])
	if err != nil {
		return mdef, serr.Wrap(err, "Unable to obtain menu with id: "+fmt.Sprintf("%d", m.Opts.ItemIds[0]))
	}
	return menuDefFromModel(mnu)
}

func (m *ModuleMenuForm) Render(params map[string]map[string]string, loggedIn bool) string {
	// fmt.Printf("*|* Params: %#v\n", params)
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	var err error
	mnu := MenuDef{}

	operation := "Create"
	action := ""
	if len(m.Opts.ItemIds) > 0 {
		operation = "Update"
		mnu, err = m.getData()
		if err != nil {
			logger.LogErr(err, "Error in menu render", "module", fmt.Sprintf("%#v", m.Opts))
			return "error generating menu"
		}
		fmt.Printf("Menu object: %#v\n", mnu)
		action = "/update/" + mnu.Id
	}

	b := element.NewBuilder()

	byts, err := json.Marshal(mnu.Items)
	if err != nil {
		logger.LogErr(err, "Error marshalling menu items for menu form", "menu_presenter", fmt.Sprintf("%#v", mnu))
		return "menu error"
	}

	b.DivClass("wrapper-material-form").R(
		b.H3("class", "page-title").T(operation+" "+m.Name.Singular),
		b.Form("id", "menu_form", "method", "post", "action", "/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "id", "items", "name", "items", "value", ""),
			b.Input("type", "hidden", "name", "menu_id", "value", mnu.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),
			b.DivClass("form-inner").R(

				b.DivClass("form-inline").R(
					b.DivClass("form-group").R(
						b.Input("name", "menu_title", "type", "text", "value", mnu.Title),
						b.Label("class", "control-label", "for", "menu_title").T("Menu Title"),
						b.IClass("bar").R(),
					),
					b.DivClass("form-group").R(
						b.Input("class", "form-group__slug", "name", "menu_slug", "type", "text",
							"placeholder", "slug is automatically generated on save", "value", mnu.Slug),
						b.Label("class", "control-label form-group__label--disabled", "for", "menu_slug").T("Menu Slug"),
						b.IClass("bar").R(),
					),
				),
				b.DivClass("form-inline").R(
					b.DivClass("checkbox").R(
						b.Label().R(
							b.Wrap(func() {
								if mnu.Published {
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
							b.Wrap(func() {
								if mnu.IsAdmin {
									b.Input("type", "checkbox", "name", "is_admin", "checked", "checked")
								} else {
									b.Input("type", "checkbox", "name", "is_admin")
								}
							}),
							b.IClass("helper").R(),
							b.T("For Admin Only"),
						),
						b.IClass("bar").R(),
					),
				),
				b.DivClass("form-inline").R(
					b.DivClass("form-group").R(
						b.H3().T("Menu Items"),
					),
					b.Button("class", "btn-add-menu-item", "title", "Add Menu Item").T("+"),
				),
			), // end form-inner
			b.DivClass("form-group").R(
				b.Input("type", "submit", "class", "button", "value", operation),
			),
		),
		b.Script("type", "text/javascript").T(
			"var items = JSON.parse(`"+string(byts)+"`);"+
				`var newItem = {
			label: "", url: "", sub_menu_slug: ""
		};

		function preSubmit() {
			$('#items').val($('#menu_form').serializeJSON());
			//console.log($('#items').get(0).value);
			return true;
		}

		$(document).ready(function() {
			var inner = $(".form-inner");
			var add_button = $("#menu_form .btn-add-menu-item"); //Add button ID
			var count = 0;
			var max_components = 20; //maximum components allowed

			// Initial Components
			if(items) {
				//console.log(items);  // ***debug***
				for (var i = 0; i < items.length; i++) {
					$(inner).append(buildComponent(i, items[i])); //add input box
				}
			}
			// Can Add
			$(add_button).click(function (e) { //on add input button click
				e.preventDefault();
				if (count < max_components) {
					$(inner).append(buildComponent($('.item').length, newItem)); //add input box
				}
			});

			// Remove
			$(inner).on("click",".remove_field", function(e){
				e.preventDefault();
				$(this).closest('.item').remove();
				count--;
				reorderItems();
			});
			// Move Up
			$(inner).on("click",".move_up", function(e){
				e.preventDefault();
				var parent = $(this).closest('.item');
				parent.insertBefore(parent.prev('.item'));
				reorderItems();
			});
			// Move Down
			$(inner).on("click",".move_down", function(e){
				e.preventDefault();
				var parent = $(this).closest('.item');
				parent.insertAfter(parent.next('.item'));
				reorderItems();
			});
		});

function buildComponent(x, item) {
	return $('<div class="item form-pack">' +
		'<div class="form-group"><label>Label</label>&nbsp;<input type="text" placeholder="label" name="items[' + x + '][label]" value="' + item.label + '"><i class="bar"></i></div>' +
		'<div class="form-group"><label>URL (e.g. /pages/page-slug...)</label><input type="text" placeholder="/pages/page-slug or #" name="items[' + x + '][url]" value="' + item.url + '" /><i class="bar"></i></div>' +
		'<div class="form-group"><label>Submenu slug</label><input type="text" name="items[' + x + '][sub_menu_slug]" value="' + item.sub_menu_slug + '"><i class="bar"></i></div>' +
		'<div class="form-group btn-group"><button class="btn move_up" title="Move row up">Up</button><button class="btn move_down" title="Move row down">Down</button>' +
			'<button class="remove_field">Delete</button></div>');
}
function checkbox(displayName, fieldName, index, isChecked) {
	var str = hspace() + '<input type="checkbox" name="items[' + index + '][' + fieldName + ']"';
	if (isChecked) {
		str += ' checked="checked"';
	}
	str += ' /> ' + displayName + hspace();
	return str;
}
function reorderItems() {
	$('.item').each(function(i){
		$(this).find('input[name^=items]').each(function(){
			m = $(this).attr("name").replace(/items\[\d+\]/, 'items[' + i + ']');
			$(this).attr("name", m);
		})
	})
}
function hspace() {
	return '&nbsp;&nbsp;'
}`),
	)

	return b.String()
}
