package grid

import (
	"strings"
	"testing"
)

// A representative grid: date column with grouping, link/edit/delete cells,
// and a value needing HTML escaping.
func testGrid() Grid {
	return Grid{
		Class: "test-grid",
		Columns: []Column{
			{Header: "Id", Type: ColNum, Shrink: true},
			{Header: "Date", Type: ColDate, GroupBy: true},
			{Header: "Title"},
			{Header: "", NoSort: true, NoFilter: true, Shrink: true},
			{Header: "", NoSort: true, NoFilter: true, Shrink: true},
		},
		Rows: [][]Cell{
			{
				Text("1"),
				Text("2026-07-04"),
				Link(`Fish & "Chips" <Special>`, "/sermons/1"),
				EditLink("/admin/sermons/edit/1"),
				DeleteLink("/admin/sermons/delete/1"),
			},
			{
				Text("2"),
				Text("2025-01-19"),
				Link("Second", "/sermons/2"),
				EditLink("/admin/sermons/edit/2"),
				DeleteLink("/admin/sermons/delete/2"),
			},
		},
		Limit:  2,
		Offset: 2,
	}
}

func TestRenderBasicStructure(t *testing.T) {
	out := testGrid().RenderString()

	for _, want := range []string{
		`class="ch-grid test-grid"`,
		`data-group-col="1"`, // the Date column drives year grouping
		`class="ch-grid-table"`,
		`data-year="2026"`,
		`data-year="2025"`,
		`href="/sermons/1"`,
		// delete links stay inert without JS: href is '#', target in data-url
		`data-url="/admin/sermons/delete/1"`,
		`class="ch-grid-del"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered grid missing %q\n%s", want, out)
		}
	}
}

func TestRenderEscapesUserText(t *testing.T) {
	out := testGrid().RenderString()
	if !strings.Contains(out, "Fish &amp; &#34;Chips&#34; &lt;Special&gt;") {
		t.Errorf("cell text was not HTML-escaped:\n%s", out)
	}
	if strings.Contains(out, `<Special>`) {
		t.Error("raw angle brackets leaked into the output")
	}
}

func TestServerPagerLinks(t *testing.T) {
	out := testGrid().RenderString()
	// Offset 2 with limit 2: Prev goes back to offset 0.
	if !strings.Contains(out, `href="?limit=2&offset=0"`) {
		t.Errorf("missing Prev server-pager link:\n%s", out)
	}
	// A full page (len(Rows) == Limit) implies more rows may follow.
	if !strings.Contains(out, `href="?limit=2&offset=4"`) {
		t.Errorf("missing Next server-pager link:\n%s", out)
	}

	// A short page at offset 0 needs no server pager at all.
	g := testGrid()
	g.Offset = 0
	g.Rows = g.Rows[:1]
	if strings.Contains(g.RenderString(), "ch-grid-serverpager") {
		t.Error("server pager rendered when all rows fit on one page")
	}
}

func TestEmptyMessage(t *testing.T) {
	g := Grid{
		Columns:      []Column{{Header: "A"}, {Header: "B"}},
		EmptyMessage: "No sermons found",
	}
	out := g.RenderString()
	if !strings.Contains(out, "No sermons found") || !strings.Contains(out, `colspan="2"`) {
		t.Errorf("empty grid should render the empty message spanning all columns:\n%s", out)
	}
}

func TestHTMLCellRawAndPopup(t *testing.T) {
	g := Grid{
		Columns: []Column{{Header: "Summary", Popup: true}},
		Rows:    [][]Cell{{HTML("<p>Rich <b>content</b></p>")}},
	}
	out := g.RenderString()
	if !strings.Contains(out, "<p>Rich <b>content</b></p>") {
		t.Errorf("trusted HTML cell should render unescaped:\n%s", out)
	}
	if !strings.Contains(out, "ch-grid-popup") {
		t.Errorf("popup column should mark its cells:\n%s", out)
	}
}
