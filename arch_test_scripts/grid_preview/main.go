// Preview harness for the grid package: renders a sample sermons-style grid
// (multi-year dates, links, edit/delete, popup and HTML cells) into a
// standalone HTML file so the client-side behavior (sorting, filtering,
// pagination, year grouping) can be exercised in a browser without the app's
// Postgres/Redis stack.
package main

import (
	"fmt"
	"os"

	"github.com/rohanthewiz/church/grid"
	"github.com/rohanthewiz/element"
)

func main() {
	outPath := "grid_preview.html"
	if len(os.Args) > 1 {
		outPath = os.Args[1]
	}

	g := grid.Grid{
		Class:        "sermons-list-grid",
		EmptyMessage: "No sermons found",
		Limit:        60, // simulate a full server page so Next appears
		Offset:       60,
	}
	g.Columns = []grid.Column{
		{Header: "Id", Type: grid.ColNum, Shrink: true},
		{Header: "Date Preached", Type: grid.ColDate, Width: 120, GroupBy: true},
		{Header: "Title"},
		{Header: "Scripture Refs."},
		{Header: "Categories", Popup: true},
		{Header: "Summary", Popup: true},
		{Header: "", NoSort: true, NoFilter: true, Shrink: true},
		{Header: "", NoSort: true, NoFilter: true, Shrink: true},
	}

	books := []string{"Genesis", "Psalms", "Isaiah", "Matthew", "John", "Romans", "Hebrews"}
	cats := []string{"Faith", "Grace", "Prophecy", "History", "Wisdom"}
	for i := 1; i <= 60; i++ {
		year := 2020 + (i % 6)
		month := 1 + (i % 12)
		day := 1 + (i*7)%28
		date := fmt.Sprintf("%d-%02d-%02d", year, month, day)
		g.Rows = append(g.Rows, []grid.Cell{
			grid.Text(fmt.Sprintf("%d", i)),
			grid.Text(date),
			grid.Link(fmt.Sprintf("Sermon %02d — Hope & \"Renewal\" <part %d>", i, i%3+1), fmt.Sprintf("/sermons/%d", i)),
			grid.Text(fmt.Sprintf("%s %d:%d", books[i%len(books)], i%20+1, i%30+1)),
			grid.Text(cats[i%len(cats)]),
			grid.HTML(fmt.Sprintf("<p>Summary for sermon <b>%d</b> with <i>rich</i> markup and a longer body to test the row clamp behavior of HTML cells in the grid layout.</p>", i)),
			grid.EditLink(fmt.Sprintf("/admin/sermons/edit/%d", i)),
			grid.DeleteLink(fmt.Sprintf("/admin/sermons/delete/%d", i)),
		})
	}

	b := element.NewBuilder()
	b.Html().R(
		b.Head().R(
			b.Title().T("Grid Preview"),
			b.Meta("charset", "utf-8"),
			b.Style().T(grid.CSS),
			b.Script("type", "text/javascript").T(grid.JS),
		),
		b.Body("style", "font-family: sans-serif; margin: 1.5rem;").R(
			b.H2().T("church/grid preview — sermons-style data"),
			g.Render(b),
		),
	)

	if err := os.WriteFile(outPath, []byte(b.String()), 0644); err != nil {
		fmt.Println("write failed:", err)
		os.Exit(1)
	}
	fmt.Println("wrote", outPath)
}
