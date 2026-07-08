package event

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

type ModuleEventForm struct {
	module.Presenter
	csrf string
}

const ModuleTypeEventForm = "event_form"

// Event Form deals with only a single item referenced in ItemIds[0] or a new one otherwise
func NewModuleEventForm(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleEventForm)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	// Slug is set only when the module model (db) is created. mod.Opts.Slug = string_util.SlugWithRandomString(title)

	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token.")
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

// Since this is only called from Render(), so safeties are in the caller (Render())
func (m ModuleEventForm) getData() (pres Presenter, err error) {
	evt, err := findEventById(m.Opts.ItemIds[0])
	if err != nil {
		return pres, serr.Wrap(err, "Unable to obtain event with id: "+fmt.Sprintf("%d", m.Opts.ItemIds[0]))
	}
	pres = presenterFromModel(evt)
	// Recurrence lives in its own table; load it only here (single-event edit)
	// rather than in presenterFromModel, which list views call per row
	if err = pres.LoadRecurrence(evt.ID); err != nil {
		logger.LogErr(err, "Unable to load recurrence rule for event form", "event_id", pres.Id)
	}
	return pres, nil
}

// selectOptions renders a select control's options, marking the current value
func selectOptions(b *element.Builder, opts [][2]string, current string) {
	for _, opt := range opts {
		params := []string{"value", opt[0]}
		if opt[0] == current {
			params = append(params, "selected", "selected")
		}
		b.Option(params...).T(opt[1])
	}
}

// The form is fully self-styled: every rule is scoped under .ef-wrap so it
// cannot leak into other admin modules, and the page needs no site CSS
// rebuild (each church site compiles its own app.css, so shipping styles
// with the module is the only zero-deploy-coordination option).
// Field layout:
//
//	┌ Event Details ─────────────────────────┐
//	│ Title            | Location            │
//	└─────────────────────────────────────────┘
//	┌ Date & Recurrence ─────────────────────┐
//	│ Date             | Time                │
//	│ [One-time][Weekly][Monthly]  (segmented)│
//	│ (panel: week-of-month, weekday, until)  │
//	│ "Repeats: Second Saturday of each month"│
//	└─────────────────────────────────────────┘
//	... Contact / Content / footer (publish + submit)
const eventFormCSS = `
.ef-wrap { max-width: 56rem; margin: 0.5rem auto 2rem; color: #2d3436;
	font-size: 1rem; }
.ef-wrap .ef-page-title { text-align: center; text-transform: capitalize; margin: 0.6rem 0 1rem; }
.ef-card { background: #fff; border: 1px solid #dfe6e0; border-radius: 8px;
	padding: 1rem 1.2rem 1.2rem; margin-bottom: 1.1rem;
	box-shadow: 0 1px 3px rgba(0,0,0,0.06); }
.ef-card__title { font-size: 0.8rem; font-weight: 600; letter-spacing: 0.08em;
	text-transform: uppercase; color: #6b7c74; margin-bottom: 0.9rem;
	border-bottom: 1px solid #eef2ee; padding-bottom: 0.45rem; }
.ef-row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.9rem 1.4rem; }
.ef-row--3 { grid-template-columns: 1fr 1fr 1fr; }
@media (max-width: 640px) { .ef-row, .ef-row--3 { grid-template-columns: 1fr; } }
.ef-field { display: flex; flex-direction: column; }
.ef-field label { font-size: 0.82rem; font-weight: 600; color: #57606a;
	margin-bottom: 0.28rem; }
.ef-field .ef-opt { font-weight: 400; color: #98a1a8; }
.ef-req { color: #d9534f; }
.ef-field input, .ef-field select {
	font-size: 0.95rem; color: #2d3436; background: #fbfdfb;
	border: 1px solid #c9d3cc; border-radius: 5px; padding: 0.42rem 0.55rem;
	line-height: 1.4; width: 100%; box-shadow: none;
	transition: border-color 0.2s ease, box-shadow 0.2s ease; }
.ef-field input:focus, .ef-field select:focus { outline: none;
	border-color: #337ab7; box-shadow: 0 0 0 3px rgba(51,122,183,0.15); }
/* Segmented control: the radios stay in the form (so recur_freq posts
   unchanged) but are visually replaced by their labels */
.ef-seg { display: inline-flex; border: 1px solid #c9d3cc; border-radius: 6px;
	overflow: hidden; margin: 0.2rem 0 0.4rem; }
.ef-seg input[type="radio"] { position: absolute; opacity: 0; width: 0; height: 0; }
.ef-seg label { padding: 0.4rem 1.15rem; font-size: 0.9rem; cursor: pointer;
	background: #fbfdfb; color: #57606a; border-left: 1px solid #c9d3cc;
	margin: 0; transition: background 0.15s ease, color 0.15s ease; }
.ef-seg label:first-of-type { border-left: none; }
.ef-seg input:checked + label { background: #337ab7; color: #fff; }
.ef-seg input:focus-visible + label { box-shadow: inset 0 0 0 2px rgba(51,122,183,0.5); }
.ef-recur-panel { display: none; background: #f4f8f4; border: 1px solid #e0e9e0;
	border-radius: 6px; padding: 0.8rem 0.9rem; margin-top: 0.6rem; }
.ef-recur-panel.ef-show { display: block; }
.ef-recur-summary { margin: 0.7rem 0 0; font-size: 0.9rem; color: #33691e;
	background: #f1f8e9; border-left: 3px solid #7cb342; padding: 0.45rem 0.7rem;
	border-radius: 0 4px 4px 0; }
.ef-help { font-size: 0.8rem; color: #98a1a8; margin: 0.45rem 0 0; }
/* Publish toggle: a plain checkbox (posts "on" as before) drawn as a switch */
.ef-switch { display: inline-flex; align-items: center; cursor: pointer; gap: 0.6rem; }
.ef-switch input { position: absolute; opacity: 0; width: 0; height: 0; }
.ef-switch .ef-slider { width: 2.4rem; height: 1.3rem; background: #c9d3cc;
	border-radius: 1rem; position: relative; transition: background 0.2s ease;
	flex: none; }
.ef-switch .ef-slider::before { content: ''; position: absolute; top: 0.15rem;
	left: 0.15rem; width: 1rem; height: 1rem; background: #fff; border-radius: 50%;
	transition: transform 0.2s ease; box-shadow: 0 1px 2px rgba(0,0,0,0.25); }
.ef-switch input:checked + .ef-slider { background: #7cb342; }
.ef-switch input:checked + .ef-slider::before { transform: translateX(1.1rem); }
.ef-switch .ef-switch-text { font-size: 0.95rem; font-weight: 600; color: #57606a; }
.ef-footer { display: flex; align-items: center; justify-content: space-between;
	padding: 0.4rem 0.2rem; }
.ef-submit { background: #337ab7; color: #fff; border: 1px solid #2e6da4;
	border-radius: 6px; font-size: 1rem; padding: 0.5rem 2.6rem; cursor: pointer;
	transition: background 0.2s ease, box-shadow 0.2s ease;
	box-shadow: 0 2px 4px rgba(0,0,0,0.15); }
.ef-submit:hover { background: #286090; box-shadow: 0 3px 8px rgba(0,0,0,0.2); }
/* Summernote editors keep their own (scoped bootstrap) look; just space the labels */
.ef-editor label { display: block; font-size: 0.82rem; font-weight: 600;
	color: #57606a; margin-bottom: 0.28rem; }
.ef-editor { margin-bottom: 1rem; }
`

// Client-side recurrence behavior. Kept in vanilla JS (jQuery is only needed
// for Summernote). Three responsibilities:
//  1. Show/hide the recurrence panel and the monthly-only "week of month"
//     field to match the selected frequency.
//  2. Mirror the server's Recurrence.Describe() wording in a live summary so
//     the admin can read the rule back in plain English before saving.
//  3. Convenience defaults: until the admin touches weekday/week themselves,
//     keep them in sync with the chosen event date (picking July 12 which is
//     a Sunday pre-selects "Sunday" / "Second"). Dates are parsed by splitting
//     the yyyy-mm-dd string — new Date("yyyy-mm-dd") is parsed as UTC and can
//     land on the previous local day, shifting the weekday.
const eventFormJS = `
(function () {
	var freqRadios = document.querySelectorAll('input[name="recur_freq"]');
	var panel = document.getElementById('recur_panel');
	var weekField = document.getElementById('recur_week_field');
	var weekSel = document.getElementById('recur_week');
	var weekdaySel = document.getElementById('recur_weekday');
	var untilInput = document.getElementById('recur_until');
	var dateInput = document.querySelector('input[name="event_date"]');
	var summaryEl = document.getElementById('recur_summary');
	// An existing rule's weekday/week are deliberate choices - never auto-overwrite
	// them. Seeded per field: a weekly rule has no saved week-of-month, so that
	// select may still sync from the date if the admin switches to Monthly.
	var weekdayTouched = EF_HAS_RULE;
	var weekTouched = EF_WEEK_SET;

	var dayNames = ['Sunday','Monday','Tuesday','Wednesday','Thursday','Friday','Saturday'];
	var ordinals = { '1': 'First', '2': 'Second', '3': 'Third', '4': 'Fourth', '-1': 'Last' };

	function freq() {
		for (var i = 0; i < freqRadios.length; i++) {
			if (freqRadios[i].checked) { return freqRadios[i].value; }
		}
		return '';
	}

	function dateParts() {
		if (!dateInput || !dateInput.value) { return null; }
		var p = dateInput.value.split('-');
		if (p.length !== 3) { return null; }
		return { y: +p[0], m: +p[1], d: +p[2] };
	}

	function syncFromDate() {
		var dp = dateParts();
		if (!dp) { return; }
		var weekday = new Date(Date.UTC(dp.y, dp.m - 1, dp.d)).getUTCDay();
		if (!weekdayTouched) { weekdaySel.value = String(weekday); }
		// Day 1-7 is the first such weekday of the month, 8-14 the second, ...
		// capped at fourth since a fifth occurrence is better expressed as "last"
		if (!weekTouched) { weekSel.value = String(Math.min(4, Math.ceil(dp.d / 7))); }
	}

	function updateSummary() {
		var f = freq();
		if (!f) {
			summaryEl.textContent = 'This event occurs once, on the date above.';
			return;
		}
		var day = dayNames[+weekdaySel.value] || '';
		var txt = (f === 'weekly')
			? 'Every ' + day
			: (ordinals[weekSel.value] || '') + ' ' + day + ' of each month';
		if (untilInput.value) { txt += ', until ' + untilInput.value; }
		summaryEl.textContent = 'Repeats: ' + txt +
			'. The event date above is the first occurrence.';
	}

	function syncUI() {
		var f = freq();
		panel.className = f ? 'ef-recur-panel ef-show' : 'ef-recur-panel';
		weekField.style.display = (f === 'monthly') ? '' : 'none';
		updateSummary();
	}

	for (var i = 0; i < freqRadios.length; i++) {
		freqRadios[i].addEventListener('change', function () { syncFromDate(); syncUI(); });
	}
	weekdaySel.addEventListener('change', function () { weekdayTouched = true; updateSummary(); });
	weekSel.addEventListener('change', function () { weekTouched = true; updateSummary(); });
	untilInput.addEventListener('change', updateSummary);
	if (dateInput) {
		dateInput.addEventListener('change', function () { syncFromDate(); updateSummary(); });
	}

	if (!EF_HAS_RULE) { syncFromDate(); }
	syncUI();
})();
`

func (m *ModuleEventForm) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	evt := Presenter{}
	var err error

	operation := "Create"
	action := ""

	if len(m.Opts.ItemIds) > 0 {
		operation = "Update"
		evt, err = m.getData()
		if err != nil {
			logger.LogErr(err, "Error in module render", "module_type", ModuleTypeEventForm)
			return ""
		}
		action = "/update/" + evt.Id
	}

	// hasRule seeds the JS "touched" state: an existing rule's weekday/week
	// must not be silently rewritten by the date-sync convenience
	hasRule := evt.RecurFreq != RecurNone

	// The segmented frequency control is real radio inputs so recur_freq posts
	// exactly as the old select did — the controller and UpsertEvent are untouched
	freqChoices := [][2]string{
		{"", "One-time"},
		{RecurWeekly, "Weekly"},
		{RecurMonthly, "Monthly"},
	}

	b := element.NewBuilder()

	b.DivClass("ef-wrap").R(
		b.Style().T(eventFormCSS),
		b.H3("class", "ef-page-title").T(operation+" "+m.Name.Singular),
		b.Form("method", "post", "action", "/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "name", "event_id", "value", evt.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),

			b.DivClass("ef-card").R(
				b.DivClass("ef-card__title").T("Event Details"),
				b.DivClass("ef-row").R(
					b.DivClass("ef-field").R(
						b.Label("for", "event_title").R(
							b.T("Event Title "), b.SpanClass("ef-req").T("*"),
						),
						b.Input("name", "event_title", "id", "event_title", "type", "text",
							"required", "required", "value", evt.Title),
					),
					b.DivClass("ef-field").R(
						b.Label("for", "event_location").R(
							b.T("Location "), b.SpanClass("ef-req").T("*"),
						),
						b.Input("name", "event_location", "id", "event_location", "type", "text",
							"required", "required", "value", evt.Location),
					),
				),
			),

			b.DivClass("ef-card").R(
				b.DivClass("ef-card__title").T("Date & Recurrence"),
				b.DivClass("ef-row").R(
					b.DivClass("ef-field").R(
						b.Label("for", "event_date").T("Event Date"),
						b.Input("name", "event_date", "id", "event_date", "type", "date",
							"value", evt.EventDate),
					),
					b.DivClass("ef-field").R(
						b.Label("for", "event_time").T("Event Time"),
						b.Input("name", "event_time", "id", "event_time", "type", "time",
							"value", evt.EventTime),
					),
				),
				b.DivClass("ef-seg", "role", "radiogroup", "aria-label", "Repeats").R(
					b.Wrap(func() {
						for i, choice := range freqChoices {
							radioID := "recur_freq_" + strconv.Itoa(i)
							radioParams := []string{"type", "radio", "name", "recur_freq",
								"id", radioID, "value", choice[0]}
							if choice[0] == evt.RecurFreq {
								radioParams = append(radioParams, "checked", "checked")
							}
							b.Input(radioParams...)
							b.Label("for", radioID).T(choice[1])
						}
					}),
				),
				// Weekday/week/until only make sense for a repeating event; JS
				// reveals this panel for weekly/monthly. Hidden fields still post,
				// which is fine: the server ignores them when recur_freq is empty
				b.Div("id", "recur_panel", "class", "ef-recur-panel").R(
					b.DivClass("ef-row ef-row--3").R(
						b.Div("id", "recur_week_field", "class", "ef-field").R(
							b.Label("for", "recur_week").T("Week of Month"),
							b.Select("name", "recur_week", "id", "recur_week").R(
								b.Wrap(func() {
									selectOptions(b, [][2]string{
										{"1", "First"}, {"2", "Second"}, {"3", "Third"},
										{"4", "Fourth"}, {"-1", "Last"},
									}, evt.RecurWeek)
								}),
							),
						),
						b.DivClass("ef-field").R(
							b.Label("for", "recur_weekday").T("Day of Week"),
							b.Select("name", "recur_weekday", "id", "recur_weekday").R(
								b.Wrap(func() {
									selectOptions(b, [][2]string{
										{"0", "Sunday"}, {"1", "Monday"}, {"2", "Tuesday"},
										{"3", "Wednesday"}, {"4", "Thursday"}, {"5", "Friday"},
										{"6", "Saturday"},
									}, evt.RecurWeekday)
								}),
							),
						),
						b.DivClass("ef-field").R(
							b.Label("for", "recur_until").R(
								b.T("Repeat Until "), b.SpanClass("ef-opt").T("(optional)"),
							),
							b.Input("name", "recur_until", "id", "recur_until", "type", "date",
								"value", evt.RecurUntil),
						),
					),
				),
				b.P("id", "recur_summary", "class", "ef-recur-summary").T(""),
			),

			b.DivClass("ef-card").R(
				b.DivClass("ef-card__title").T("Contact (optional)"),
				b.DivClass("ef-row ef-row--3").R(
					b.DivClass("ef-field").R(
						b.Label("for", "contact_person").T("Contact Person"),
						b.Input("name", "contact_person", "id", "contact_person", "type", "text",
							"value", evt.ContactPerson),
					),
					b.DivClass("ef-field").R(
						b.Label("for", "contact_email").T("Contact Email"),
						b.Input("name", "contact_email", "id", "contact_email", "type", "text",
							"value", evt.ContactEmail),
					),
					b.DivClass("ef-field").R(
						b.Label("for", "contact_phone").T("Contact Phone"),
						b.Input("name", "contact_phone", "id", "contact_phone", "type", "text",
							"value", evt.ContactPhone),
					),
				),
				// b.DivClass("ef-field").R(
				// 	b.Label("for", "contact_url").T("Contact URL"),
				// 	b.Input("name", "contact_url", "id", "contact_url", "type", "text",
				// 		"value", evt.ContactURL),
				// ),
			),

			b.DivClass("ef-card").R(
				b.DivClass("ef-card__title").T("Content"),
				b.DivClass("ef-field").R(
					b.Label("for", "categories").T("Tags (comma separated)"),
					b.Input("name", "categories", "id", "categories", "type", "text",
						"value", strings.Join(evt.Categories, ", ")),
				),
				b.DivClass("ef-editor bootstrap-wrapper", "style", "margin-top:1rem").R(
					b.Label("for", "event_summary").T("Summary"),
					b.Div("id", "summer1").T(evt.Summary),
					b.TextArea("id", "event_summary", "name", "event_summary", "type", "text", "value", "",
						"style", "display:none").T(""),
				),
				b.DivClass("ef-editor bootstrap-wrapper").R(
					b.Label("for", "event_body").T("Event Body"),
					b.Div("id", "summer2").T(evt.Body),
					b.TextArea("id", "event_body", "name", "event_body", "type", "text", "value", "",
						"style", "display:none").T(""),
				),
			),

			b.DivClass("ef-footer").R(
				b.Label("class", "ef-switch").R(
					b.Wrap(func() {
						if evt.Published {
							b.Input("type", "checkbox", "name", "published", "checked", "checked")
						} else {
							b.Input("type", "checkbox", "name", "published")
						}
					}),
					b.SpanClass("ef-slider").T(""),
					b.SpanClass("ef-switch-text").T("Published"),
				),
				b.Input("type", "submit", "class", "ef-submit", "value", operation),
			),
		),

		b.Script("type", "text/javascript").T(
			`$(document).ready(function(){$('#summer1').summernote(); $('#summer2').summernote();});
			function preSubmit() {  // todo validate fields here
				var s1 = $("#summer1");
				var s2 = $("#summer2");
				var summary = document.getElementById("event_summary");
				var body = document.getElementById("event_body");
				if (s1 && summary) {
					summary.innerHTML = s1.summernote('code');
				}
				if (s2 && body) {
					body.innerHTML = s2.summernote('code');
				}
				return true;
			}
			var EF_HAS_RULE = `+strconv.FormatBool(hasRule)+`;
			var EF_WEEK_SET = `+strconv.FormatBool(evt.RecurWeek != "")+`;
			`+eventFormJS),
	)

	return b.String()
}
