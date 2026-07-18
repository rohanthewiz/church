# Session: Event Page Structure Facelift + CCSWM Theme Example

- Session ID: `8bdf6aa7-7354-4364-8a02-6583e318984f`
- Date: 2026-07-17 23:15
- Repos touched: `~/projs/go/church/church` (framework), `~/projs/go/church/ccswm` (site theme)

## Goal

The public event detail page (e.g. `https://ccswm.org/events/37`) needed a facelift.
Since styling belongs to each site using the church framework, the framework's job
was to emit a rich, semantic, fully-classed **structure** (with zero styling) that
site themes can hang CSS on. Then, as an example, ccswm got a themed implementation.

## Framework changes (`church/church`)

### `resource/event/get_presenter.go`
Added pre-split date-part fields to `Presenter`, populated in `presenterFromModel`
from the event's local time (fixed formats — structural tokens, unlike the
site-configurable display formats):

- `EventDateISO` — RFC3339, feeds `<time datetime="...">` (SEO / calendar JS)
- `EventMonthShort` — "Jul"
- `EventDayOfMonth` — "17"
- `EventWeekday` — "Friday"

Rationale: the presenter is the one place that already knows the event's local
time; views should arrange strings, never parse them.

### `resource/event/module_single_event.go`
Rewrote `Render` from a bare class-less label/value `<table>` to a semantic
structure under the platform's `ch-module-wrapper ch-<module_type>` convention
(matching article/list modules). Key points:

- Honors `Opts.CustomClass` on the wrapper (per-page variants)
- `article.ch-event[data-event-id]` — stable JS hook
- Empty sections are omitted server-side (no `:empty` tricks needed in themes)
- Contact links: `tel:` (spaces stripped), `mailto:`, and `contactHref()` which
  normalizes editor-pasted URLs (bare host → `https://`, internal `/path` and
  scheme'd URLs pass through)
- Date badge is `aria-hidden` (duplicates `.ch-event-when` for screen readers)
- Admin edit link preserved

Structure contract (full tree also in a comment on `Render` and in
`ai_docs/theming_event_single.md`):

```
div.ch-module-wrapper.ch-event_single [.CustomClass]
├─ div.ch-module-heading
└─ div.ch-module-body
   └─ article.ch-event [data-event-id]
      ├─ header.ch-event-header
      │  ├─ div.ch-event-date-badge (span month/day/weekday)
      │  └─ div.ch-event-headline > h2.ch-event-title + div.ch-event-meta
      │       (span.ch-event-when > time.ch-event-datetime + span.ch-event-time,
      │        span.ch-event-where)
      ├─ div.ch-event-summary
      ├─ div.ch-event-description   (rich HTML from editor)
      ├─ section.ch-event-contact > ul.ch-event-contact-list
      │    li.ch-event-contact-item.ch-contact-{person,phone,email,url}
      │       (span.ch-contact-label + span.ch-contact-value)
      └─ footer.ch-event-footer
           ul.ch-event-categories > li.ch-event-category
           span.ch-event-updated
```

### `ai_docs/theming_event_single.md` (new)
Theming guide for site authors: the structure contract, notes (label spans can
be swapped for icons; `<time datetime>` readable from JS), and a copy-paste
starter CSS skeleton (layout scaffolding only; colors/fonts left to sites).

Verified: `go build ./...`, `go vet`, `go test ./resource/event/` all pass.
Element v0.5.6 (pinned) has all needed semantic helpers (Article/Header/Footer/
Section/Time + *Class variants).

## CCSWM theme example (`ccswm` repo)

Site styles are Stylus: partials in `styles/styl/_styl/`, module mixins in
`_styl/modules/*` invoked inside `.ch-module-wrapper` in `_module.styl`, scoped
by `&.ch-<module_type>`.

- **New** `styles/styl/_styl/modules/event_single.styl` — `event_single()` mixin
  using existing Cobalt theme vars: `color-cema-blue` date badge with
  `theme-common-box-shadow-right-bottom`; title in
  `theme-cobalt-article-heading-color`; muted meta line (dot before time, "@"
  before location); italic lede summary; defensive description styles
  (`img { max-width: 100% }`); `color-beige` contact card with auto-fit grid;
  category pills via `theme-cobalt-categories-*`; light-border footer.
  No mobile breakpoint on purpose — site body has `min-width: 800px`.
- `_module.styl` — added `event_single()` to the mixin call list.
- Rebuilt `dist/css/app.css` (+ map) via `npx stylus styles/styl/master.styl -o
  dist/css/app.css -m` (same as the `build-css-task:stylus-compile` npm script;
  `node_modules` not installed in ccswm). 22 `ch-event*` selectors emitted,
  correctly nested under `body .ch-module-wrapper.ch-event_single`.

## Follow-ups / next steps

- Run a site locally and eyeball `/events/<id>` with a fully-populated event and
  a sparse one to confirm conditional sections.
- cema has no event_single theme yet — copy the starter skeleton from
  `ai_docs/theming_event_single.md` or adapt ccswm's mixin.
- Consider "Add to Calendar" JS using `data-event-id` + `time[datetime]`.
