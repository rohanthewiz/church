// Client side of the admin Page form (see page/module_page_form.go).
// Rewritten vanilla-JS: the old version string-built jQuery HTML and relied on
// jquery.serialize-object to post the whole form as JSON. Now the DOM is the
// single source of truth and submit serializes exactly the {"mods": [...]}
// shape page/form_objects.go unmarshals — booleans as real booleans, ids /
// limit / offset as strings (ModuleReceiver declares them as strings).
//
// Injected globals (set by the Go module just before this script):
//   modules     - the page's saved module option sets ([{opts: {...}}, ...])
//   moduleTypes - selectable module types for dynamic pages
//   contentBys  - moduleType -> content kind ("SingleId" | "MultiId" |
//                 "Pagination" | "Form" | ...) driving which option fields
//                 apply to a given type
var modules, moduleTypes, contentBys;

// PACKER START ModulePageForm_js
(function () {
	'use strict';

	var MAX_MODULES = 16;

	var list;       // container the module cards live in
	var addBtn;
	var countEl;    // "N modules" badge in the card title

	var newModuleOpts = {
		layout_column: 'center', published: true, main_module: false,
		title: '', module_type: 'article_single', custom_class: '',
		item_ids: [], show_unpublished: false, ascending: false,
		limit: 5, offset: 0
	};

	// content kind helpers -----------------------------------------------------
	function kindOf(modType) { return contentBys[modType] || ''; }
	function usesIds(modType) {
		var k = kindOf(modType);
		return k === 'SingleId' || k === 'MultiId';
	}
	function usesPagination(modType) { return kindOf(modType) === 'Pagination'; }

	// tiny DOM helpers ---------------------------------------------------------
	function el(tag, className, attrs) {
		var node = document.createElement(tag);
		if (className) { node.className = className; }
		if (attrs) {
			for (var k in attrs) {
				if (Object.prototype.hasOwnProperty.call(attrs, k)) {
					node.setAttribute(k, attrs[k]);
				}
			}
		}
		return node;
	}

	function field(labelText, input, extraClass) {
		var f = el('div', 'af-field' + (extraClass ? ' ' + extraClass : ''));
		var lb = el('label');
		lb.textContent = labelText;
		f.appendChild(lb);
		f.appendChild(input);
		return f;
	}

	function textInput(fieldName, value, placeholder) {
		var inp = el('input', '', { type: 'text', 'data-field': fieldName });
		if (placeholder) { inp.placeholder = placeholder; }
		inp.value = (value === undefined || value === null) ? '' : value;
		return inp;
	}

	function switchCtl(labelText, fieldName, checked, extraClass) {
		var lb = el('label', 'af-switch' + (extraClass ? ' ' + extraClass : ''));
		var inp = el('input', '', { type: 'checkbox', 'data-field': fieldName });
		inp.checked = !!checked;
		var slider = el('span', 'af-slider');
		var txt = el('span', 'af-switch-text');
		txt.textContent = labelText;
		lb.appendChild(inp); lb.appendChild(slider); lb.appendChild(txt);
		return lb;
	}

	// one module card ----------------------------------------------------------
	function buildCard(opts) {
		var card = el('div', 'pf-module');

		// Header: summary text + reorder/collapse/remove controls. The summary
		// mirrors type + title live so a collapsed card is still identifiable.
		var head = el('div', 'pf-module__head');
		var summary = el('span', 'pf-module__summary');
		var tools = el('div', 'pf-module__tools');
		var upBtn = btn('↑', 'Move up');
		var downBtn = btn('↓', 'Move down');
		var togBtn = btn('−', 'Collapse');
		var delBtn = btn('×', 'Remove module', 'af-btn--danger');
		tools.appendChild(upBtn); tools.appendChild(downBtn);
		tools.appendChild(togBtn); tools.appendChild(delBtn);
		head.appendChild(summary); head.appendChild(tools);
		card.appendChild(head);

		var body = el('div', 'pf-module__body');
		card.appendChild(body);

		// Row 1: type + title
		var typeSel = el('select', '', { 'data-field': 'module_type' });
		for (var i = 0; i < moduleTypes.length; i++) {
			var opt = el('option');
			opt.value = moduleTypes[i];
			opt.textContent = moduleTypes[i].replace(/_/g, ' ');
			if (moduleTypes[i] === opts.module_type) { opt.selected = true; }
			typeSel.appendChild(opt);
		}
		var titleInp = textInput('title', opts.title, 'shown as the module heading');
		var row1 = el('div', 'af-row');
		row1.appendChild(field('Module Type', typeSel));
		row1.appendChild(field('Module Title', titleInp));
		body.appendChild(row1);

		// Row 2: column + content-selection fields (which of these matter
		// depends on the module type; irrelevant ones are hidden, not disabled,
		// so the form never shows dead controls)
		var colSel = el('select', '', { 'data-field': 'layout_column' });
		['center', 'left', 'right'].forEach(function (c) {
			var o = el('option');
			o.value = c; o.textContent = c;
			if (c === (opts.layout_column || 'center')) { o.selected = true; }
			colSel.appendChild(o);
		});
		// A saved page may use a custom column name; keep it selectable
		if (opts.layout_column && ['center', 'left', 'right'].indexOf(opts.layout_column) === -1) {
			var custom = el('option');
			custom.value = opts.layout_column; custom.textContent = opts.layout_column;
			custom.selected = true;
			colSel.appendChild(custom);
		}
		var idsInp = textInput('item_ids',
			(opts.item_ids || []).join(','), 'e.g. 12 or 12,15');
		var limitInp = textInput('limit', String(opts.limit), 'how many');
		var offsetInp = textInput('offset', String(opts.offset), 'skip first N');
		var row2 = el('div', 'af-row af-row--3');
		row2.appendChild(field('Column Position', colSel));
		var idsField = field('Item Id(s)', idsInp, 'pf-ids');
		var limitField = field('Items to List', limitInp, 'pf-limit');
		var offsetField = field('Items to Skip', offsetInp, 'pf-offset');
		row2.appendChild(idsField);
		row2.appendChild(limitField);
		row2.appendChild(offsetField);
		body.appendChild(row2);

		// Row 3: custom style class
		var styleInp = textInput('custom_class', opts.custom_class,
			'CSS class(es), space separated');
		var row3 = el('div', 'af-row');
		row3.appendChild(field('Module Style Class (optional)', styleInp));
		body.appendChild(row3);

		// Row 4: switches
		var switches = el('div', 'pf-switches');
		switches.appendChild(switchCtl('Published', 'published', opts.published));
		switches.appendChild(switchCtl('Main Module', 'main_module',
			opts.main_module || opts.is_main_module, 'pf-main-switch'));
		switches.appendChild(switchCtl('Show Unpublished', 'show_unpublished',
			opts.show_unpublished, 'pf-list-switch'));
		switches.appendChild(switchCtl('Oldest First', 'ascending',
			opts.ascending, 'pf-list-switch'));
		body.appendChild(switches);

		function syncKind() {
			var t = typeSel.value;
			idsField.style.display = usesIds(t) ? '' : 'none';
			var pag = usesPagination(t);
			limitField.style.display = pag ? '' : 'none';
			offsetField.style.display = pag ? '' : 'none';
			var listSwitches = switches.querySelectorAll('.pf-list-switch');
			for (var i = 0; i < listSwitches.length; i++) {
				listSwitches[i].style.display = pag ? '' : 'none';
			}
		}
		function syncSummary() {
			var t = typeSel.value.replace(/_/g, ' ');
			summary.textContent = titleInp.value ? t + ' — ' + titleInp.value : t;
		}
		typeSel.addEventListener('change', function () { syncKind(); syncSummary(); });
		titleInp.addEventListener('input', syncSummary);

		// Only one module should be the page's main module; checking one
		// unchecks the rest rather than leaving the admin to hunt for it
		switches.querySelector('.pf-main-switch input')
			.addEventListener('change', function (e) {
				if (!e.target.checked) { return; }
				var others = list.querySelectorAll('.pf-main-switch input');
				for (var i = 0; i < others.length; i++) {
					if (others[i] !== e.target) { others[i].checked = false; }
				}
			});

		upBtn.addEventListener('click', function () {
			var prev = card.previousElementSibling;
			if (prev) { list.insertBefore(card, prev); }
		});
		downBtn.addEventListener('click', function () {
			var next = card.nextElementSibling;
			if (next) { list.insertBefore(next, card); }
		});
		togBtn.addEventListener('click', function () {
			var hidden = body.style.display === 'none';
			body.style.display = hidden ? '' : 'none';
			togBtn.textContent = hidden ? '−' : '+';
			togBtn.title = hidden ? 'Collapse' : 'Expand';
		});
		delBtn.addEventListener('click', function () {
			if (window.confirm('Remove this module from the page?')) {
				card.parentNode.removeChild(card);
				syncCount();
			}
		});

		syncKind();
		syncSummary();
		return card;
	}

	function btn(txt, title, extraClass) {
		var x = el('button', 'af-btn' + (extraClass ? ' ' + extraClass : ''),
			{ type: 'button', title: title });
		x.textContent = txt;
		return x;
	}

	function syncCount() {
		var n = list.querySelectorAll('.pf-module').length;
		countEl.textContent = n + (n === 1 ? ' module' : ' modules');
		addBtn.disabled = n >= MAX_MODULES;
		var empty = document.getElementById('pf_empty');
		if (empty) { empty.style.display = n === 0 ? '' : 'none'; }
	}

	// submit: DOM -> {"mods": [...]} in the hidden #modules field ---------------
	window.preSubmit = function () {
		var mods = [];
		var cards = list.querySelectorAll('.pf-module');
		for (var i = 0; i < cards.length; i++) {
			var read = function (name) {
				return cards[i].querySelector('[data-field="' + name + '"]');
			};
			mods.push({
				module_type: read('module_type').value,
				title: read('title').value,
				layout_column: read('layout_column').value,
				item_ids: read('item_ids').value,
				limit: read('limit').value,
				offset: read('offset').value,
				custom_class: read('custom_class').value,
				published: read('published').checked,
				main_module: read('main_module').checked,
				show_unpublished: read('show_unpublished').checked,
				ascending: read('ascending').checked
			});
		}
		if (mods.length === 0) {
			window.alert('A page needs at least one module. Use "+ Add Module".');
			return false;
		}
		document.getElementById('modules').value = JSON.stringify({ mods: mods });
		return true;
	};

	document.addEventListener('DOMContentLoaded', function () {
		list = document.getElementById('pf_modules');
		addBtn = document.getElementById('pf_add_module');
		countEl = document.getElementById('pf_count');

		if (modules) {
			for (var i = 0; i < modules.length; i++) {
				list.appendChild(buildCard(modules[i].opts || modules[i]));
			}
		}
		addBtn.addEventListener('click', function () {
			if (list.querySelectorAll('.pf-module').length >= MAX_MODULES) { return; }
			var card = buildCard(newModuleOpts);
			list.appendChild(card);
			syncCount();
			card.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
		});
		syncCount();

		// The page-level position toggles maintain the hidden
		// available_positions value ("left,center,right" subset; center fixed)
		var posBoxes = document.querySelectorAll('.pf-pos input[type="checkbox"]');
		var posHidden = document.getElementById('available_positions');
		function syncPositions() {
			var vals = ['center']; // center is the page's required main column
			for (var i = 0; i < posBoxes.length; i++) {
				if (posBoxes[i].checked) { vals.push(posBoxes[i].value); }
			}
			// keep canonical order for a stable, readable stored value
			vals.sort(function (a, b) {
				var order = { left: 0, center: 1, right: 2 };
				return order[a] - order[b];
			});
			posHidden.value = vals.join(',');
		}
		for (var p = 0; p < posBoxes.length; p++) {
			posBoxes[p].addEventListener('change', syncPositions);
		}
		if (posBoxes.length) { syncPositions(); }
	});
})();
// PACKER END
