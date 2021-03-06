var modules,
	moduleTypes,
	contentBys; // module option types (by Id(s) or by limit/offset)

// PACKER START ModulePageForm_js
// The format is PACKER START <varname_to_hold_contents>
var newModule = {
		opts: {
			layout_column: "center", published: true, main_module: false,
			title: "", slug: "", module_type: "article_single", custom_class: "",
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

// Todo - init (contentBys[mod.opts.module_type] != "SingleId" && contentBys[mod.opts.module_type] != "MultiId") to a var isList
function buildComponent(x, mod) {
	var ids = "";
	if (mod.opts.item_ids) {
		ids = mod.opts.item_ids.join(',');
	}
	return $('<div class="module">' +
		'<div class="form-pack">' +
			'<div class="form-group"><label>Choose Module Type</label><select class="module_type" name="mods[' + x + '][module_type]" value="' +
					mod.opts.module_type + '">' + buildModuleTypeOptions(mod.opts.module_type) + '</select><i class="bar"></i>' +
			'</div>' +
			'<div class="form-group"><label>Module Title</label>&nbsp;<input type="text" placeholder="title" name="mods[' + x + '][title]" ' +
					'value="' + mod.opts.title + '" /><i class="bar"></i>' +
			'</div>' +
			'<div class="form-group btn-group"><button class="btn move_up" title="Move row up">Up</button>' +
					'<button class="btn move_down" title="Move row down">Down</button>' +
					'<button class="remove_field">Delete</button>' +
			'</div>' +
		'</div>' +

		'<div class="form-pack">' +
			'<div class="form-group"><label>Column Position (e.g. center)</label>&nbsp;<input type="text"' +
					'placeholder="layout_column" name="mods[' + x + '][layout_column]" value="' + mod.opts.layout_column +
					'"><i class="bar"></i>' +
			'</div>' +
			'<div class="form-group"><label>Item Ids</label>&nbsp;<input class="by-id can-disable" type="text"' +
					((contentBys[mod.opts.module_type] != "SingleId" && contentBys[mod.opts.module_type] != "MultiId") ? ' disabled="disabled"' : '') +
					'placeholder="Item id(s)" name="mods[' + x + '][item_ids]" value="' + ids + '"><i class="bar"></i>' +
			'</div>' +
		'</div>' +
		'<div class="form-pack">' + // Todo - Apply a class to show only on isList (bool var)
			'<div class="form-group"><label>Number of Items to List</label>&nbsp;<input class="by-list can-disable" type="text"' +
					((contentBys[mod.opts.module_type] == "SingleId" || contentBys[mod.opts.module_type] == "MultiId") ? ' disabled="disabled"' : '')  +
					'placeholder="limit" name="mods[' + x + '][limit]" value="' + mod.opts.limit + '"><i class="bar"></i>' +
			'</div>' +
			'<div class="form-group"><label>Number of Items to Skip</label>&nbsp;<input class="by-list can-disable" type="text"' +
					((contentBys[mod.opts.module_type] == "SingleId" || contentBys[mod.opts.module_type] == "MultiId") ? ' disabled="disabled"' : '')  +
					'placeholder="offset" name="mods[' + x + '][offset]" value="' + mod.opts.offset + '"><i class="bar"></i>' +
			'</div>' +
		'</div>' +
		'<div class="form-pack">' +
			'<div class="form-group"><label>Module style</label>&nbsp;<input placeholder="module style(s) - separate multiples with spaces" name="mods[' + x + '][custom_class]" value="' + mod.opts.custom_class + '"><i class="bar"></i>' +
			'</div>' +
		'</div>' +
		'<div class="form-pack">' +
			//checkbox('Admin', 'is_admin', x, mod.opts.is_admin) +
			checkbox('Published', 'published', x, mod.opts.published) +
			checkbox('Main Module (Only one module should be the Main)', 'main_module', x, mod.opts.is_main_module) +
			checkbox('Show Unpublished', 'show_unpublished', x, mod.opts.show_unpublished) +
			checkbox('Oldest First', 'ascending', x, mod.opts.ascending) +
		'</div>' +
	'</div>');
}

function buildModuleTypeOptions(modType) {
	var out = "";
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
// PACKER END