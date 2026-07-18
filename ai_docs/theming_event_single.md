# Theming the Single-Event Page (`ch-event_single`)

The church framework renders the event detail page (e.g. `/events/37`) as a
semantic, fully-classed structure and ships **no styling** for it. Each site
(cema, ccswm, ...) hangs its own themed CSS on the hooks below. Sections with
no data (contact, categories, summary) are omitted server-side, so themes never
need `:empty` tricks.

Rendered by `resource/event/module_single_event.go`.

## Structure contract

```
div.ch-module-wrapper.ch-event_single [.CustomClass]
├─ div.ch-module-heading                  module title (hide it if the theme uses the h2 instead)
└─ div.ch-module-body
   └─ article.ch-event [data-event-id]    JS hook for share/calendar-export buttons
      ├─ header.ch-event-header
      │  ├─ div.ch-event-date-badge       stacked calendar badge (aria-hidden; purely visual)
      │  │    span.ch-event-badge-month   "Jul"
      │  │    span.ch-event-badge-day     "17"
      │  │    span.ch-event-badge-weekday "Friday"
      │  └─ div.ch-event-headline
      │       h2.ch-event-title
      │       div.ch-event-meta
      │         span.ch-event-when
      │           time.ch-event-datetime[datetime=RFC3339]   "Friday, 7/17/2026"
      │           span.ch-event-time                          "6:30 PM"
      │         span.ch-event-where                           location
      ├─ div.ch-event-summary             plain-text lede
      ├─ div.ch-event-description         rich HTML from the admin editor — style descendants defensively
      ├─ section.ch-event-contact         only when at least one contact field is set
      │    h3.ch-event-section-heading
      │    ul.ch-event-contact-list
      │      li.ch-event-contact-item.ch-contact-person   span.ch-contact-label + span.ch-contact-value
      │      li.ch-event-contact-item.ch-contact-phone    value wraps a tel: link
      │      li.ch-event-contact-item.ch-contact-email    value wraps a mailto: link
      │      li.ch-event-contact-item.ch-contact-url      value wraps an external link
      └─ footer.ch-event-footer           only when categories or updated-at exist
           ul.ch-event-categories > li.ch-event-category   render as tag pills
           span.ch-event-updated
```

Notes for theme authors:

- `.ch-contact-label` spans carry the field name ("Phone", "Email"...). Show
  them, hide them, or replace them with icons (`.ch-contact-phone .ch-contact-label { ... }`).
- `CustomClass` on the module instance lands on the wrapper — use it for
  per-page variants (e.g. `.featured`).
- The `<time datetime>` attribute is RFC3339; safe to read from JS for
  "Add to Calendar" features.

## Starter skeleton (copy into the site stylesheet and theme it)

Layout scaffolding only — spacing, alignment, and the badge stack. Colors,
fonts, borders, and radii are deliberately left to the site.

```css
/* ---- Single Event (ch-event_single) ---------------------------------- */
.ch-event_single .ch-event-header {
  display: flex;
  gap: 1.25rem;
  align-items: flex-start;
}
.ch-event_single .ch-event-date-badge {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 4.5rem;
  padding: 0.5rem;
  text-align: center;
  /* theme: background, border, radius */
}
.ch-event_single .ch-event-badge-day {
  font-size: 2rem;
  font-weight: 700;
  line-height: 1;
}
.ch-event_single .ch-event-badge-month { text-transform: uppercase; letter-spacing: 0.08em; }
.ch-event_single .ch-event-badge-weekday { font-size: 0.75rem; }

.ch-event_single .ch-event-title { margin: 0 0 0.25rem; }
.ch-event_single .ch-event-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 1.25rem;
  /* theme: muted color */
}
.ch-event_single .ch-event-time::before { content: "\2022"; margin: 0 0.4em; } /* dot separator */

.ch-event_single .ch-event-summary { font-size: 1.1rem; /* theme: lede treatment */ }
.ch-event_single .ch-event-description { /* style rich-text descendants: p, ul, img { max-width: 100%; } */ }

.ch-event_single .ch-event-contact-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.5rem;
}
.ch-event_single .ch-contact-label { font-weight: 600; margin-right: 0.5em; }

.ch-event_single .ch-event-categories {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.ch-event_single .ch-event-category { padding: 0.15rem 0.6rem; /* theme: pill bg + radius */ }
.ch-event_single .ch-event-updated { font-size: 0.8rem; /* theme: muted color */ }

@media (max-width: 40rem) {
  .ch-event_single .ch-event-header { flex-direction: column; }
}
```
