package menu

import (
	"encoding/json"
	"fmt"

	"github.com/rohanthewiz/church/app"
	theDB "github.com/rohanthewiz/church/db"
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
	dbH, err := theDB.Db()
	if err != nil {
		return mdef, serr.Wrap(err, "Could not obtain DB handle")
	}
	mnu, err := findModelById(dbH, m.Opts.ItemIds[0])
	if err != nil {
		return mdef, serr.Wrap(err, "Unable to obtain menu with id: "+fmt.Sprintf("%d", m.Opts.ItemIds[0]))
	}
	return menuDefFromModel(mnu)
}

// Menu-form-only structural styling; colors resolve through the shared --af-*
// custom properties (template/admin_css.go) so site themes re-skin this form.
// Each menu item is one responsive grid row: three fields plus a tool cluster
// that drops under the fields on narrow screens.
const menuFormCSS = `
.mf-item { display: grid; grid-template-columns: 1fr 1.4fr 1fr auto;
	gap: 0.6rem 0.9rem; align-items: end; border: 1px solid var(--af-inset-border);
	border-radius: 6px; background: var(--af-inset-bg);
	padding: 0.6rem 0.7rem 0.7rem; margin-bottom: 0.6rem; min-width: 0; }
.mf-item__tools { display: flex; gap: 0.3rem; padding-bottom: 0.15rem; }
.mf-item__tools .af-btn { padding: 0.15rem 0.55rem; font-size: 0.95rem; }
.mf-switches { display: flex; flex-wrap: wrap; gap: 0.5rem 1.4rem; margin-top: 0.8rem; }
@media (max-width: 760px) {
	.mf-item { grid-template-columns: 1fr; align-items: stretch; }
	.mf-item__tools { justify-content: flex-end; padding-bottom: 0; }
}
`

// Client side: vanilla JS, DOM as source of truth. Submit serializes exactly
// the {"items": [...]} shape the controller unmarshals (FormMenuObject) —
// the old version posted the whole jQuery-serialized form instead.
const menuFormJS = `
(function () {
	'use strict';
	var MAX_ITEMS = 20;
	var list, addBtn, countEl;

	function el(tag, className, attrs) {
		var n = document.createElement(tag);
		if (className) { n.className = className; }
		if (attrs) { for (var k in attrs) { n.setAttribute(k, attrs[k]); } }
		return n;
	}

	function field(labelText, fieldName, value, placeholder) {
		var f = el('div', 'af-field');
		var lb = el('label');
		lb.textContent = labelText;
		var inp = el('input', '', { type: 'text', 'data-field': fieldName });
		if (placeholder) { inp.placeholder = placeholder; }
		inp.value = value || '';
		f.appendChild(lb); f.appendChild(inp);
		return f;
	}

	function btn(txt, title, extraClass) {
		var x = el('button', 'af-btn' + (extraClass ? ' ' + extraClass : ''),
			{ type: 'button', title: title });
		x.textContent = txt;
		return x;
	}

	function buildItem(item) {
		var row = el('div', 'mf-item');
		row.appendChild(field('Label', 'label', item.label, 'e.g. About Us'));
		row.appendChild(field('URL', 'url', item.url, '/pages/page-slug, or # for submenu parent'));
		row.appendChild(field('Submenu Slug (optional)', 'sub_menu_slug', item.sub_menu_slug,
			'slug of another menu'));

		var tools = el('div', 'mf-item__tools');
		var up = btn('↑', 'Move up');
		var down = btn('↓', 'Move down');
		var del = btn('×', 'Remove menu item', 'af-btn--danger');
		tools.appendChild(up); tools.appendChild(down); tools.appendChild(del);
		row.appendChild(tools);

		up.addEventListener('click', function () {
			var prev = row.previousElementSibling;
			if (prev) { list.insertBefore(row, prev); }
		});
		down.addEventListener('click', function () {
			var next = row.nextElementSibling;
			if (next) { list.insertBefore(next, row); }
		});
		del.addEventListener('click', function () {
			if (window.confirm('Remove this menu item?')) {
				row.parentNode.removeChild(row);
				syncCount();
			}
		});
		return row;
	}

	function syncCount() {
		var n = list.querySelectorAll('.mf-item').length;
		countEl.textContent = n + (n === 1 ? ' item' : ' items');
		addBtn.disabled = n >= MAX_ITEMS;
		var empty = document.getElementById('mf_empty');
		if (empty) { empty.style.display = n === 0 ? '' : 'none'; }
	}

	window.preSubmit = function () {
		var out = [];
		var rows = list.querySelectorAll('.mf-item');
		for (var i = 0; i < rows.length; i++) {
			var read = function (name) {
				return rows[i].querySelector('[data-field="' + name + '"]').value;
			};
			out.push({ label: read('label'), url: read('url'),
				sub_menu_slug: read('sub_menu_slug') });
		}
		document.getElementById('items').value = JSON.stringify({ items: out });
		return true;
	};

	document.addEventListener('DOMContentLoaded', function () {
		list = document.getElementById('mf_items');
		addBtn = document.getElementById('mf_add_item');
		countEl = document.getElementById('mf_count');

		if (window.menuItems) {
			for (var i = 0; i < window.menuItems.length; i++) {
				list.appendChild(buildItem(window.menuItems[i]));
			}
		}
		addBtn.addEventListener('click', function () {
			if (list.querySelectorAll('.mf-item').length >= MAX_ITEMS) { return; }
			var row = buildItem({ label: '', url: '', sub_menu_slug: '' });
			list.appendChild(row);
			syncCount();
			row.querySelector('input').focus();
		});
		syncCount();
	});
})();
`

func (m *ModuleMenuForm) Render(params map[string]map[string]string, loggedIn bool) string {
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
		action = "/update/" + mnu.Id
	}

	b := element.NewBuilder()

	byts, err := json.Marshal(mnu.Items)
	if err != nil {
		logger.LogErr(err, "Error marshalling menu items for menu form", "menu_presenter", fmt.Sprintf("%#v", mnu))
		return "menu error"
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
		b.Style().T(menuFormCSS),
		b.H3("class", "af-page-title").T(operation+" "+m.Name.Singular),
		b.Form("id", "menu_form", "method", "post", "action", "/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "id", "items", "name", "items", "value", ""),
			b.Input("type", "hidden", "name", "menu_id", "value", mnu.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Menu Details"),
				b.DivClass("af-row").R(
					b.DivClass("af-field").R(
						b.Label("for", "menu_title").R(
							b.T("Menu Title "), b.SpanClass("af-req").T("*"),
						),
						b.Input("name", "menu_title", "id", "menu_title", "type", "text",
							"required", "required", "value", mnu.Title),
					),
					b.DivClass("af-field").R(
						b.Label("for", "menu_slug").R(
							b.T("Menu Slug "), b.SpanClass("af-opt").T("(auto-generated on save)"),
						),
						// The server derives the slug from the title and ignores this
						// field on post, so show it read-only rather than implying
						// it is editable (the old form accepted edits it discarded)
						b.Input("name", "menu_slug", "id", "menu_slug", "type", "text",
							"readonly", "readonly", "placeholder", "generated when saved",
							"value", mnu.Slug),
					),
				),
				b.DivClass("mf-switches").R(
					b.Wrap(func() {
						namedSwitch("Published", "published", mnu.Published)
						namedSwitch("For Admin Only", "is_admin", mnu.IsAdmin)
					}),
				),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").R(
					b.Span().T("Menu Items"),
					b.Span().R(
						b.Span("id", "mf_count", "style", "font-weight:400;margin-right:0.8rem").T(""),
						b.Button("id", "mf_add_item", "type", "button", "class", "af-btn af-btn--primary",
							"title", "Add a menu item").T("+ Add Item"),
					),
				),
				b.PClass("af-help").T(`Each item links to a URL like "/pages/page-slug". `+
					`To make an item open a submenu, set its URL to "#" and put the other menu's slug in Submenu Slug.`),
				b.PClass("af-help", "id", "mf_empty").T("No items yet — use \"+ Add Item\" to build this menu."),
				b.Div("id", "mf_items").R(),
			),

			b.DivClass("af-footer").R(
				b.AClass("af-btn", "href", "/admin/"+m.Name.Plural).T("Cancel"),
				b.Input("type", "submit", "class", "af-submit", "value", operation),
			),
		),
		b.Script("type", "text/javascript").T(
			"window.menuItems = JSON.parse(`"+string(byts)+"`);"+menuFormJS),
	)

	return b.String()
}
