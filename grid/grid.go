// Package grid is a server-rendered data grid built on the element HTML builder.
// It replaces the former AG Grid integration (see the agrid package) with a plain
// semantic <table> plus a small vanilla-JS enhancer (assets.go) that provides:
//
//   - multi-column sorting (click a header; shift-click adds secondary sorts)
//   - per-column filters and a quick "search all columns" box
//   - client-side pagination with a page-size selector
//   - collapsible year grouping keyed on a date column
//   - delete links with a confirm dialog, and click-to-popup cells
//
// Design choice: all rows for the current server page are rendered into the
// document (exactly what the AG Grid version did by embedding row JSON), and the
// JS manages which rows are attached to the tbody. That keeps the DOM light for
// large row sets (lazy row attachment), degrades to a plain readable table when
// JS is unavailable, and needs no extra data endpoints. Server-side paging across
// larger datasets rides on the existing ?limit=&offset= query params that
// basectlr.RenderPageListRWeb already routes to the main module.
package grid

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/rohanthewiz/element"
)

// Column value types. The type drives the client-side comparison used when
// sorting: numbers compare numerically, dates and text lexically (our dates are
// ISO "2006-01-02" per config.DisplayDateFormat, so string order == time order).
const (
	ColText = "text"
	ColDate = "date"
	ColNum  = "num"
)

// Column describes one grid column. The zero value is a sortable, filterable
// text column.
type Column struct {
	Header   string
	Type     string // ColText (default), ColDate, or ColNum
	Width    int    // optional min-width in px (0 = let the browser decide)
	NoSort   bool   // exclude from sorting (action columns)
	NoFilter bool   // exclude from the per-column filter row
	Popup    bool   // clicking a cell in this column pops up its full content
	GroupBy  bool   // offer "Group by Year" on this column (should be ColDate)
	Shrink   bool   // keep the column as narrow as its content (ids, actions)
}

// Cell is one table cell. Text is HTML-escaped on render; HTML is trusted raw
// markup (e.g. an article summary produced by the site's own editor) and wins
// over Text when both are set.
type Cell struct {
	Text    string
	Href    string // when set the cell renders as a link
	HTML    string // trusted raw HTML content
	Confirm bool   // link asks for confirmation before navigating (deletes)
	SortVal string // optional sort key override; defaults to the cell text
}

// Convenience constructors — these encode the few cell shapes the list modules
// need, replacing the old AG Grid "label|href" string convention.
func Text(s string) Cell           { return Cell{Text: s} }
func Link(label, href string) Cell { return Cell{Text: label, Href: href} }
func EditLink(href string) Cell    { return Cell{Text: "edit", Href: href} }
func DeleteLink(href string) Cell  { return Cell{Text: "del", Href: href, Confirm: true} }
func HTML(raw string) Cell         { return Cell{HTML: raw} }

// Grid is the renderable component. It satisfies element's Component interface
// via Render(b) so it can be composed inside any element tree.
type Grid struct {
	Class        string // extra CSS class(es) on the wrapper (e.g. "sermons-list-grid")
	Columns      []Column
	Rows         [][]Cell
	PageSize     int    // client-side page size; <= 0 uses the JS default (25)
	EmptyMessage string // shown when there are no rows at all

	// Server-side paging (optional). When Limit > 0, Prev/Next links are
	// rendered that reload the current path with limit/offset query params —
	// the same contract Presenter.SetLimitAndOffset already consumes. Next is
	// only offered when the current page came back full (len(Rows) == Limit),
	// mirroring Presenter.RenderPagination.
	Limit  int64
	Offset int64

	// CSRFToken, when set, is stamped on the wrapper as data-csrf. Delete
	// links then submit a POST form carrying it (see assets.go) instead of
	// navigating — deletes must be POSTs so they can't be triggered by a bare
	// GET (link prefetch, <img src>, CSRF). The token itself comes from the
	// caller (app.GenerateFormToken) since grid stays free of app imports.
	CSRFToken string
}

// yearRe extracts a 4-digit year from a date cell for grouping. A regexp (vs
// parsing) keeps us tolerant of any display format that contains a year.
var yearRe = regexp.MustCompile(`(19|20)\d\d`)

// groupColIndex returns the index of the column marked GroupBy, or -1.
func (g Grid) groupColIndex() int {
	for i, col := range g.Columns {
		if col.GroupBy {
			return i
		}
	}
	return -1
}

// Render writes the grid. Structure (classes are what the JS/CSS key off):
//
//	div.ch-grid [data-page-size] [data-group-col]
//	├── div.ch-grid-toolbar (.ch-grid-jsonly — hidden until JS initializes)
//	│     input.ch-grid-search | button.ch-grid-groupbtn? | span.ch-grid-count
//	├── div.ch-grid-scroll > table.ch-grid-table
//	│     thead: header row (th.ch-grid-sortable) + filter row (.ch-grid-jsonly)
//	│     tbody: one tr per row [data-year on the group column's year]
//	├── div.ch-grid-pager (.ch-grid-jsonly — populated by JS)
//	└── div.ch-grid-serverpager (plain links; works without JS)
func (g Grid) Render(b *element.Builder) (x any) {
	groupCol := g.groupColIndex()

	wrapAttrs := []string{"data-page-size", strconv.Itoa(g.PageSize)}
	if groupCol >= 0 {
		wrapAttrs = append(wrapAttrs, "data-group-col", strconv.Itoa(groupCol))
	}
	if g.CSRFToken != "" {
		wrapAttrs = append(wrapAttrs, "data-csrf", esc(g.CSRFToken))
	}

	cls := "ch-grid"
	if g.Class != "" {
		cls += " " + g.Class
	}

	b.DivClass(cls, wrapAttrs...).R(
		g.renderToolbar(b, groupCol),
		b.DivClass("ch-grid-scroll").R(
			b.TableClass("ch-grid-table").R(
				g.renderHead(b),
				g.renderBody(b, groupCol),
			),
		),
		b.DivClass("ch-grid-pager ch-grid-jsonly").R(),
		g.renderServerPager(b),
	)
	return
}

// RenderString is a convenience for callers that need the grid as a standalone
// HTML fragment rather than as part of a larger element tree.
func (g Grid) RenderString() string {
	b := element.NewBuilder()
	g.Render(b)
	return b.String()
}

func (g Grid) renderToolbar(b *element.Builder, groupCol int) (x any) {
	b.DivClass("ch-grid-toolbar ch-grid-jsonly").R(
		b.Input("class", "ch-grid-search", "type", "search", "placeholder", "Search..."),
		b.Wrap(func() {
			if groupCol >= 0 {
				b.Button("class", "ch-grid-groupbtn", "type", "button").T("Group by Year")
			}
		}),
		b.SpanClass("ch-grid-count").F("%d rows", len(g.Rows)),
	)
	return
}

func (g Grid) renderHead(b *element.Builder) (x any) {
	b.THead().R(
		// Header row: each th carries its column index and type so the JS can
		// sort without any per-grid configuration.
		b.Tr().R(
			b.Wrap(func() {
				for i, col := range g.Columns {
					thCls := "ch-grid-th"
					if !col.NoSort {
						thCls += " ch-grid-sortable"
					}
					if col.Shrink {
						thCls += " ch-grid-shrink"
					}
					colType := col.Type
					if colType == "" {
						colType = ColText
					}
					attrs := []string{"data-idx", strconv.Itoa(i), "data-type", colType}
					if col.Width > 0 {
						attrs = append(attrs, "style", "min-width: "+strconv.Itoa(col.Width)+"px")
					}
					b.ThClass(thCls, attrs...).R(
						b.SpanClass("ch-grid-th-label").T(esc(col.Header)),
						// Sort indicator target: JS writes the arrow (and the
						// priority number for multi-column sorts) in here.
						b.SpanClass("ch-grid-sort-ind").R(),
					)
				}
			}),
		),
		// Filter row: a text input per filterable column. Hidden until JS
		// initializes since filtering is purely client-side.
		b.TrClass("ch-grid-filter-row ch-grid-jsonly").R(
			b.Wrap(func() {
				for i, col := range g.Columns {
					b.ThClass("ch-grid-filter-cell").R(
						b.Wrap(func() {
							if !col.NoFilter && col.Header != "" {
								b.Input("class", "ch-grid-colfilter", "data-idx", strconv.Itoa(i),
									"type", "search", "placeholder", "Filter")
							}
						}),
					)
				}
			}),
		),
	)
	return
}

func (g Grid) renderBody(b *element.Builder, groupCol int) (x any) {
	b.TBody().R(
		b.Wrap(func() {
			if len(g.Rows) == 0 {
				msg := g.EmptyMessage
				if msg == "" {
					msg = "No records found"
				}
				b.TrClass("ch-grid-empty-row").R(
					b.Td("colspan", strconv.Itoa(len(g.Columns))).T(esc(msg)),
				)
				return
			}
			for _, row := range g.Rows {
				trAttrs := []string{}
				// Stamp the row's year server-side (we know the date format);
				// the JS then groups on this attribute without date parsing.
				if groupCol >= 0 && groupCol < len(row) {
					dateVal := row[groupCol].SortVal
					if dateVal == "" {
						dateVal = row[groupCol].Text
					}
					if year := yearRe.FindString(dateVal); year != "" {
						trAttrs = append(trAttrs, "data-year", year)
					}
				}
				b.Tr(trAttrs...).R(
					b.Wrap(func() {
						for i, cl := range row {
							var col Column
							if i < len(g.Columns) {
								col = g.Columns[i]
							}
							g.renderCell(b, col, cl)
						}
					}),
				)
			}
		}),
	)
	return
}

func (g Grid) renderCell(b *element.Builder, col Column, cl Cell) {
	classes := []string{}
	if col.Popup {
		classes = append(classes, "ch-grid-popup")
	}
	if cl.HTML != "" {
		classes = append(classes, "ch-grid-htmlcell")
	}

	attrs := []string{}
	if len(classes) > 0 {
		attrs = append(attrs, "class", strings.Join(classes, " "))
	}
	// data-sort lets a cell display one thing and sort by another
	// (unused by the current modules since our dates are already ISO,
	// but essential the moment a friendlier date format is adopted).
	if cl.SortVal != "" && cl.SortVal != cl.Text {
		attrs = append(attrs, "data-sort", esc(cl.SortVal))
	}

	b.Td(attrs...).R(
		b.Wrap(func() {
			switch {
			case cl.HTML != "":
				// Trusted markup (site-authored content). The inner div gives
				// the CSS a clamp target and the popup JS a content source.
				b.DivClass("ch-grid-html").T(cl.HTML)
			case cl.Href != "" && cl.Confirm:
				// Deletes navigate via JS after a confirm dialog; href stays
				// inert so a stray click without JS cannot delete anything.
				b.AClass("ch-grid-del", "href", "#", "data-url", esc(cl.Href)).T(esc(cl.Text))
			case cl.Href != "":
				b.A("href", esc(cl.Href)).T(esc(cl.Text))
			default:
				b.T(esc(cl.Text))
			}
		}),
	)
}

// renderServerPager emits plain Prev/Next links driven by limit/offset — the
// one grid feature that must work without JS since it is how users reach rows
// beyond the current server page.
func (g Grid) renderServerPager(b *element.Builder) (x any) {
	if g.Limit <= 0 {
		return
	}
	hasPrev := g.Offset > 0
	hasNext := int64(len(g.Rows)) == g.Limit // a full page implies more may follow
	if !hasPrev && !hasNext {
		return
	}

	pageNum := g.Offset/g.Limit + 1
	fmtInt := func(n int64) string { return strconv.FormatInt(n, 10) }

	b.DivClass("ch-grid-serverpager").R(
		b.Wrap(func() {
			if hasPrev {
				prevOffset := g.Offset - g.Limit
				if prevOffset < 0 {
					prevOffset = 0
				}
				// Relative query-only href: resolves against the current path,
				// so the same link works on public and admin list pages.
				b.A("href", "?limit="+fmtInt(g.Limit)+"&offset="+fmtInt(prevOffset)).T("‹ Prev")
			}
			b.Span().F(" Page %d ", pageNum)
			if hasNext {
				b.A("href", "?limit="+fmtInt(g.Limit)+"&offset="+fmtInt(g.Offset+g.Limit)).T("Next ›")
			}
		}),
	)
	return
}

// esc HTML-escapes user data. element writes strings through verbatim, so
// escaping is this package's responsibility for any value we didn't author.
func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;")
	return r.Replace(s)
}
