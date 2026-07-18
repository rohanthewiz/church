package event

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	. "github.com/rohanthewiz/logger"
)

const ModuleTypeSingleEvent = "event_single"

type ModuleSingleEvent struct {
	module.Presenter
}

func NewModuleSingleEvent(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleSingleEvent)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

func (m ModuleSingleEvent) getData() (pres Presenter, err error) {
	if len(m.Opts.ItemIds) < 1 {
		return
	}
	dbH, err := db.Db()
	if err != nil {
		LogErr(err, "Could not obtain DB handle")
		return pres, err
	}
	evt, err := findEventById(dbH, m.Opts.ItemIds[0])
	if err != nil {
		LogErr(err, "Unable to obtain event", "event_id", fmt.Sprintf("%d", m.Opts.ItemIds[0]))
		return pres, err
	}
	return presenterFromModel(evt,
		PresenterParams{TimeNormalFormat: "3:04 PM", DateLongFormat: "1/2/2006", DateTimeFormat: "1/2/2006 3:04 PM TZ"}), err
}

// contactHref normalizes a user-entered contact URL into something safe to
// drop into an href. Editors paste anything from "ccswm.org/contact" to full
// URLs to internal paths; a bare host without a scheme would otherwise be
// treated as a relative path and 404.
func contactHref(rawURL string) string {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return ""
	}
	if strings.HasPrefix(trimmedURL, "/") || strings.Contains(trimmedURL, "://") {
		return trimmedURL // internal path or already has a scheme
	}
	return "https://" + trimmedURL
}

// Render emits the single-event page as a semantic, fully-classed structure.
// The framework deliberately ships no styling for it — every hook below is a
// contract with the per-site stylesheets (cema, ccswm, ...), which own the look.
// Sections with no data are omitted entirely so themes never have to hide
// empty shells.
//
// Structure contract (all classes prefixed ch- per platform convention):
//
//	div.ch-module-wrapper.ch-event_single [.CustomClass]
//	└─ div.ch-module-heading            module title (theme may hide)
//	└─ div.ch-module-body
//	   └─ article.ch-event [data-event-id]
//	      ├─ header.ch-event-header
//	      │  ├─ div.ch-event-date-badge      stacked Jul / 17 / Friday
//	      │  │    span.ch-event-badge-month
//	      │  │    span.ch-event-badge-day
//	      │  │    span.ch-event-badge-weekday
//	      │  └─ div.ch-event-headline
//	      │       h2.ch-event-title
//	      │       div.ch-event-meta
//	      │         span.ch-event-when > time.ch-event-datetime + span.ch-event-time
//	      │         span.ch-event-where
//	      ├─ div.ch-event-summary            plain-text lede
//	      ├─ div.ch-event-description        rich HTML from the editor
//	      ├─ section.ch-event-contact
//	      │    h3.ch-event-section-heading
//	      │    ul.ch-event-contact-list > li.ch-event-contact-item.ch-contact-{person,phone,email,url}
//	      │         span.ch-contact-label + span.ch-contact-value
//	      └─ footer.ch-event-footer
//	           ul.ch-event-categories > li.ch-event-category
//	           span.ch-event-updated
func (m *ModuleSingleEvent) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	// Safety - todo add to all modules
	if len(m.Opts.ItemIds) == 0 {
		LogErr(errors.New("No id provided for module"), "Error rendering Single Module event",
			"module_options", fmt.Sprintf("%#v", m.Opts))
		return ""
	}

	evt, err := m.getData()
	if err != nil {
		LogErr(err, "Error in module render")
		return ""
	}

	// CustomClass lets a page instance opt into a site-specific variant
	// (e.g. "featured") without the framework knowing about it.
	wrapperClass := "ch-module-wrapper ch-" + m.Opts.ModuleType
	if m.Opts.CustomClass != "" {
		wrapperClass += " " + m.Opts.CustomClass
	}

	// A contact "row" only renders when it has a value; precompute so we can
	// skip the whole contact section when the event has no contact info.
	hasContact := evt.ContactPerson != "" || evt.ContactPhone != "" ||
		evt.ContactEmail != "" || evt.ContactURL != ""

	b := element.NewBuilder()

	b.DivClass(wrapperClass).R(
		b.DivClass("ch-module-heading").T(m.Opts.Title),
		b.DivClass("ch-module-body").R(
			// data-event-id gives site JS (share buttons, calendar export)
			// a stable hook without scraping the URL
			b.ArticleClass("ch-event", "data-event-id", evt.Id).R(

				b.HeaderClass("ch-event-header").R(
					// The badge duplicates info in .ch-event-when, so hide it
					// from screen readers — it is a purely visual affordance
					b.Wrap(func() {
						if evt.EventDayOfMonth != "" {
							b.DivClass("ch-event-date-badge", "aria-hidden", "true").R(
								b.SpanClass("ch-event-badge-month").T(evt.EventMonthShort),
								b.SpanClass("ch-event-badge-day").T(evt.EventDayOfMonth),
								b.SpanClass("ch-event-badge-weekday").T(evt.EventWeekday),
							)
						}
					}),
					b.DivClass("ch-event-headline").R(
						b.H2Class("ch-event-title").T(evt.Title),
						b.DivClass("ch-event-meta").R(
							b.Wrap(func() {
								if evt.EventDateDisplayLong != "" {
									b.SpanClass("ch-event-when").R(
										b.TimeClass("ch-event-datetime", "datetime", evt.EventDateISO).T(
											evt.EventWeekday+", "+evt.EventDateDisplayLong),
										b.Wrap(func() {
											if evt.EventTime != "" {
												b.SpanClass("ch-event-time").T(evt.EventTime)
											}
										}),
									)
								}
								if evt.Location != "" {
									b.SpanClass("ch-event-where").T(evt.Location)
								}
							}),
						),
					),
				),

				b.Wrap(func() {
					if evt.Summary != "" {
						b.DivClass("ch-event-summary").T(evt.Summary)
					}
					if evt.Body != "" {
						// Body is trusted rich HTML authored in the admin editor
						b.DivClass("ch-event-description").T(evt.Body)
					}

					if hasContact {
						b.SectionClass("ch-event-contact").R(
							b.H3Class("ch-event-section-heading").T("Contact"),
							b.UlClass("ch-event-contact-list").R(
								b.Wrap(func() {
									// Label spans carry the field name so themes can
									// show them, hide them, or replace them with icons
									if evt.ContactPerson != "" {
										b.LiClass("ch-event-contact-item ch-contact-person").R(
											b.SpanClass("ch-contact-label").T("Contact"),
											b.SpanClass("ch-contact-value").T(evt.ContactPerson),
										)
									}
									if evt.ContactPhone != "" {
										b.LiClass("ch-event-contact-item ch-contact-phone").R(
											b.SpanClass("ch-contact-label").T("Phone"),
											b.SpanClass("ch-contact-value").R(
												b.A("href", "tel:"+strings.ReplaceAll(evt.ContactPhone, " ", "")).T(evt.ContactPhone),
											),
										)
									}
									if evt.ContactEmail != "" {
										b.LiClass("ch-event-contact-item ch-contact-email").R(
											b.SpanClass("ch-contact-label").T("Email"),
											b.SpanClass("ch-contact-value").R(
												b.A("href", "mailto:"+evt.ContactEmail).T(evt.ContactEmail),
											),
										)
									}
									if evt.ContactURL != "" {
										b.LiClass("ch-event-contact-item ch-contact-url").R(
											b.SpanClass("ch-contact-label").T("Website"),
											b.SpanClass("ch-contact-value").R(
												b.A("href", contactHref(evt.ContactURL), "target", "_blank", "rel", "noopener").T(evt.ContactURL),
											),
										)
									}
								}),
							),
						)
					}

					if len(evt.Categories) > 0 || evt.UpdatedAt != "" {
						b.FooterClass("ch-event-footer").R(
							b.Wrap(func() {
								if len(evt.Categories) > 0 {
									b.UlClass("ch-event-categories").R(
										element.ForEach(evt.Categories, func(category string) {
											b.LiClass("ch-event-category").T(category)
										}),
									)
								}
								if evt.UpdatedAt != "" {
									b.SpanClass("ch-event-updated").T("Updated " + evt.UpdatedAt)
								}
							}),
						)
					}

					if loggedIn && len(m.Opts.ItemIds) > 0 {
						b.AClass("edit-link", "href", m.GetEditURL()+
							strconv.FormatInt(m.Opts.ItemIds[0], 10)).R(
							b.ImgClass("edit-icon", "title", "Edit Event", "src", "/assets/images/edit_article.svg").R(),
						)
					}
				}),
			),
		),
	)

	return b.String()
}
