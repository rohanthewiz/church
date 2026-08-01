package event

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/db"
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
	dbH, err := db.Db()
	if err != nil {
		return pres, serr.Wrap(err, "Could not obtain DB handle")
	}
	evt, err := findEventById(dbH, m.Opts.ItemIds[0])
	if err != nil {
		return pres, serr.Wrap(err, "Unable to obtain event with id: "+fmt.Sprintf("%d", m.Opts.ItemIds[0]))
	}
	pres = presenterFromModel(evt)
	// Recurrence lives in its own table; load it only here (single-event edit)
	// rather than in presenterFromModel, which list views call per row
	if err = pres.LoadRecurrence(dbH, evt.ID); err != nil {
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

// Shared chrome (cards/fields/segmented control/switch/footer) now comes from
// the framework-wide admin stylesheet (template/admin_css.go, af-* classes,
// themeable via --af-* custom properties). Only genuinely event-specific
// styling remains here: the recurrence reveal panel and the plain-English
// recurrence summary strip.
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
.ef-recur-panel { display: none; }
.ef-recur-panel.ef-show { display: block; }
.ef-recur-summary { margin: 0.7rem 0 0; font-size: 0.9rem; color: #33691e;
	background: #f1f8e9; border-left: 3px solid var(--af-ok); padding: 0.45rem 0.7rem;
	border-radius: 0 4px 4px 0; }
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
		panel.className = f ? 'af-inset ef-recur-panel ef-show' : 'af-inset ef-recur-panel';
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

	b.DivClass("af-wrap").R(
		b.Style().T(eventFormCSS),
		b.H3("class", "af-page-title").T(operation+" "+m.Name.Singular),
		b.Form("method", "post", "action", "/admin/"+m.Name.Plural+action, "onSubmit", "return preSubmit();").R(
			b.Input("type", "hidden", "name", "event_id", "value", evt.Id),
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Event Details"),
				b.DivClass("af-row").R(
					b.DivClass("af-field").R(
						b.Label("for", "event_title").R(
							b.T("Event Title "), b.SpanClass("af-req").T("*"),
						),
						b.Input("name", "event_title", "id", "event_title", "type", "text",
							"required", "required", "value", evt.Title),
					),
					b.DivClass("af-field").R(
						b.Label("for", "event_location").R(
							b.T("Location "), b.SpanClass("af-req").T("*"),
						),
						b.Input("name", "event_location", "id", "event_location", "type", "text",
							"required", "required", "value", evt.Location),
					),
				),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Date & Recurrence"),
				b.DivClass("af-row").R(
					b.DivClass("af-field").R(
						b.Label("for", "event_date").T("Event Date"),
						b.Input("name", "event_date", "id", "event_date", "type", "date",
							"value", evt.EventDate),
					),
					b.DivClass("af-field").R(
						b.Label("for", "event_time").T("Event Time"),
						b.Input("name", "event_time", "id", "event_time", "type", "time",
							"value", evt.EventTime),
					),
				),
				b.DivClass("af-seg", "role", "radiogroup", "aria-label", "Repeats").R(
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
				b.Div("id", "recur_panel", "class", "af-inset ef-recur-panel").R(
					b.DivClass("af-row af-row--3").R(
						b.Div("id", "recur_week_field", "class", "af-field").R(
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
						b.DivClass("af-field").R(
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
						b.DivClass("af-field").R(
							b.Label("for", "recur_until").R(
								b.T("Repeat Until "), b.SpanClass("af-opt").T("(optional)"),
							),
							b.Input("name", "recur_until", "id", "recur_until", "type", "date",
								"value", evt.RecurUntil),
						),
					),
				),
				b.P("id", "recur_summary", "class", "ef-recur-summary").T(""),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Contact (optional)"),
				b.DivClass("af-row af-row--3").R(
					b.DivClass("af-field").R(
						b.Label("for", "contact_person").T("Contact Person"),
						b.Input("name", "contact_person", "id", "contact_person", "type", "text",
							"value", evt.ContactPerson),
					),
					b.DivClass("af-field").R(
						b.Label("for", "contact_email").T("Contact Email"),
						b.Input("name", "contact_email", "id", "contact_email", "type", "text",
							"value", evt.ContactEmail),
					),
					b.DivClass("af-field").R(
						b.Label("for", "contact_phone").T("Contact Phone"),
						b.Input("name", "contact_phone", "id", "contact_phone", "type", "text",
							"value", evt.ContactPhone),
					),
				),
				// b.DivClass("af-field").R(
				// 	b.Label("for", "contact_url").T("Contact URL"),
				// 	b.Input("name", "contact_url", "id", "contact_url", "type", "text",
				// 		"value", evt.ContactURL),
				// ),
			),

			b.DivClass("af-card").R(
				b.DivClass("af-card__title").T("Content"),
				b.DivClass("af-field").R(
					b.Label("for", "categories").T("Tags (comma separated)"),
					b.Input("name", "categories", "id", "categories", "type", "text",
						"value", strings.Join(evt.Categories, ", ")),
				),
				b.DivClass("af-editor bootstrap-wrapper", "style", "margin-top:1rem").R(
					b.Label("for", "event_summary").T("Summary"),
					b.Div("id", "summer1").T(evt.Summary),
					b.TextArea("id", "event_summary", "name", "event_summary", "type", "text", "value", "",
						"style", "display:none").T(""),
				),
				b.DivClass("af-editor bootstrap-wrapper").R(
					b.Label("for", "event_body").T("Event Body"),
					b.Div("id", "summer2").T(evt.Body),
					b.TextArea("id", "event_body", "name", "event_body", "type", "text", "value", "",
						"style", "display:none").T(""),
				),
			),

			b.DivClass("af-footer").R(
				b.Label("class", "af-switch").R(
					b.Wrap(func() {
						if evt.Published {
							b.Input("type", "checkbox", "name", "published", "checked", "checked")
						} else {
							b.Input("type", "checkbox", "name", "published")
						}
					}),
					b.SpanClass("af-slider").T(""),
					b.SpanClass("af-switch-text").T("Published"),
				),
				b.Input("type", "submit", "class", "af-submit", "value", operation),
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
