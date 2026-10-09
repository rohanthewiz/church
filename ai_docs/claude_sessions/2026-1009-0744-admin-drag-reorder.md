# Admin drag-to-reorder grips

Session ID: `6d25b16d-717b-4084-b200-c054900545c9`
Date: 2026-10-09

## Request

From the owner's cats-todo backlog: in the church admin, wherever things are
ordered with arrows, keep the arrows but add a drag handle at the far right
for drag and drop.

## Where arrows order things

Only two admin lists use ↑/↓ reordering:

- **Page form** module cards: `pack/src/module_page_form.js` (packed into
  `pack/packed/module_page_form.go` via `go generate`, `router.go:41`).
- **Menu form** items: `resource/menu/module_menu_form.go` (`menuFormJS`).

Both are vanilla JS with the DOM as the source of truth. `preSubmit()`
serializes rows in document order, so reordering only has to move nodes and
no server change is needed.

## What changed

- **`template/admin_script.go`** (new): `AdminJS`, inlined for admin pages
  only, next to `AdminCSS` (`template/page.html.go`). It has one
  **declarative** behavior: any `[data-af-sort]` list whose direct children
  contain a `.af-drag-handle` becomes drag-sortable. A single delegated
  `pointerdown` on `document` covers rows added after load. Design choices
  (all documented in the file):
  - **Pointer Events, not HTML5 DnD.** HTML5 DnD needs `draggable` on the row,
    which fights the inputs inside it, and is unreliable on touch.
  - **Live reorder:** the dragged row moves past a neighbour's midpoint.
    Skipping the dragged row while scanning gives natural hysteresis, so tall
    cards don't oscillate against short ones.
  - **Listeners go on `window`, not `setPointerCapture`.** `insertBefore` of
    the dragged node counts as a removal and can drop capture.
  - **Midpoints use `offsetTop`/`offsetHeight`,** so FLIP transforms on
    neighbours mid-animation can't skew the target. `[data-af-sort]` is
    `position: relative`, so the list is the rows' `offsetParent`.
  - **FLIP slide (150ms)** for displaced rows, skipped when the OS asks for
    reduced motion.
  - **Edge auto-scroll** (rAF loop). It starts only after the pointer moves
    >4px, so grabbing a row already near the viewport edge doesn't scroll the
    page out from under it. Testing found that bug, and this gate fixes it.
- **`template/admin_css.go`:** `.af-drag-handle` is a 2×3 dot grip painted
  with `radial-gradient(currentColor …)`, so it follows theme colors. It has
  `touch-action: none` and `cursor: grab`. `.af-dragging` gets an accent
  border and shadow, and `body.af-drag-active` gets a grabbing cursor and no
  text selection. The class index comment is updated.
- **Both forms** append an `aria-hidden` handle span as the last tool, after
  ↑ ↓ (−/+) ×. The arrows stay as the keyboard and screen-reader path. The
  list divs get `data-af-sort` (`page/module_page_form.go` `#pf_modules`,
  menu form `#mf_items`).

## Gotchas

- **`admin_js.go` is silently excluded from the build:** a `_js.go` suffix is
  an implicit `GOOS=js` constraint (`undefined: AdminJS`). Hence the name
  `admin_script.go`, with a comment in the file.
- **Chrome automation:** `left_click_drag` coordinates landed about 1.33×
  off in page space (dpr 2.5), so the real drag missed the grip. In a hidden
  tab, rAF never fires (an awaiting script hung) and CSS transitions freeze
  mid-flight, so `getBoundingClientRect` of rows reflected stale FLIP
  transforms and made the test aim wrong. The fix in the test was
  `document.getAnimations().forEach(a => a.finish())` before measuring, and
  `setTimeout` instead of rAF.

## Verification

- `go build ./...`, `go vet` (template, page, menu) and `go test ./...` all
  pass. Regenerating the packed JS left `module_payment_form.go` unchanged.
- No local admin password was at hand, so a throwaway test
  (`template/zz_harness_test.go`, since deleted) rendered the real menu and
  page forms in create mode, with `AdminCSS` and `AdminJS` and seeded rows,
  into a scratchpad HTML page. A local `http.server` served it and Chrome
  drove it with synthetic `PointerEvent`s. All of these were correct:
  - drags to the top, to the bottom and into the middle
  - collapsed cards mixed with expanded ones
  - a newly added card
  - arrows still moving rows
  - right-click not starting a drag
  - `preSubmit()` order matching the new order
  - no leftover classes or inline transforms afterwards
- A mid-drag screenshot confirmed the grip sits at the far right and the
  dragged card is highlighted.
- Not done: a real drag in a running cema, and a touch drag on a phone
  (N-060).

## Also in this commit

- `ai_docs/todo/next-list.md` already had an uncommitted owner edit at
  session start: N-059 (sermon import) was moved from Open to a new Roadmap
  "Sermon import track". It is committed as-is.

## Next

Closed: None. Declined: None. Raised: N-060.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
