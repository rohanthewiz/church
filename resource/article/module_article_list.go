package article

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/rohanthewiz/church/agrid"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
)

const ModuleTypeArticlesList = "articles_list"

type ModuleArticlesList struct {
	module.Presenter
}

func NewModuleArticlesList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleArticlesList)
	mod.Name = pres.Name
	mod.Opts = pres.Opts

	// Work out local condition
	cond := "1 = 1"
	if !mod.Opts.IsAdmin && !mod.Opts.ShowUnpublished {
		cond = "published = true"
	}
	// merge with any incoming condition
	if mod.Opts.Condition != "" {
		cond = mod.Opts.Condition + " AND " + cond
	}
	mod.Opts.Condition = cond

	return module.Module(mod), nil
}

func (m ModuleArticlesList) GetData() ([]Presenter, error) {
	return QueryArticles(m.Opts.Condition, "updated_at "+m.Order(), m.Opts.Limit, m.Opts.Offset)
}

// Admin fields will be empty for normal view and so eliminated from the JSON
type rowDef struct {
	Id        string `json:"id,omitempty"`
	Published string `json:"published,omitempty"`
	Slug      string `json:"slug,omitempty"`
	Cats      string `json:"cats,omitempty"`
	UpdatedBy string `json:"updatedBy"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	Edit      string `json:"edit,omitempty"`
	Delete    string `json:"delete"`
}

func (m *ModuleArticlesList) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	articles, err := m.GetData()
	if err != nil {
		logger.LogErr(err, "Error obtaining data in module", "module_slug", m.Opts.Slug, "module_type", m.Opts.ModuleType)
		return ""
	}

	var columnDefs []agrid.ColumnDef
	if m.Opts.IsAdmin {
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Id", Field: "id", Width: 32})
	}
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Title", Field: "title", Width: 210, CellRenderer: "linkCellRenderer"})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Summary", Field: "summary", Width: 230, CellRenderer: "articleListContentRenderer"})
	if m.Opts.IsAdmin {
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Slug", Field: "slug", Width: 196})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Categories", Field: "cats", Width: 206})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Updated By", Field: "updatedBy", Width: 200})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Published", Field: "published", Width: 190})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Edit", Field: "edit", Width: 72, CellRenderer: "linkCellRenderer"})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Del", Field: "delete", Width: 70, CellRenderer: "confirmlinkCellRenderer"})
	}

	var rowData []rowDef
	for _, art := range articles {
		published := "draft"
		if art.Published {
			published = "published"
		}

		row := rowDef{}
		if m.Opts.IsAdmin {
			row.Id = art.Id
		}
		// N/A: we base64 encode bc the JS JSON parser is not handling some chars like '/' - terrible!
		row.Title = art.Title + "|" + "/" + m.Opts.ItemsURLPath + "/" + art.Id // base64.StdEncoding.EncodeToString([]byte(title))
		row.Summary = base64.StdEncoding.EncodeToString([]byte(art.Summary))
		if m.Opts.IsAdmin {
			row.Slug = art.Slug
			row.Cats = strings.Join(art.Categories, ", ")
			row.UpdatedBy = art.UpdatedBy
			row.Published = published
			row.Edit = "edit|" + m.GetEditURL() + art.Id
			row.Delete = "del|" + m.GetDeleteURL() + art.Id
		}
		rowData = append(rowData, row)
	}

	columnDefsAsJson, err := json.Marshal(columnDefs)
	rowDataAsJson, err := json.Marshal(rowData)
	if err != nil {
		logger.LogErr(err, "Error converting Article column defs to JSON")
	}
	jsConvertColumnDefs := "var columnDefs = JSON.parse(`" + string(columnDefsAsJson) + "`);"
	jsConvertRowData := "var rowData = JSON.parse(`" + string(rowDataAsJson) + "`);"

	gridOptions := `var articleListGridOptions = {
			columnDefs: columnDefs, rowData: rowData,
			enableSorting: true, enableFilter: true,
			components: { 'linkCellRenderer': chLinkCellRenderer, 'confirmlinkCellRenderer': chConfirmLinkCellRenderer,
				'articleListContentRenderer': ArticleListContentRenderer},
			onGridReady: function() { articleListGridOptions.api.sizeColumnsToFit(); },
			onCellClicked: function(event) {
				console.log(event.column);
				if (event.column.colId !== "summary" && event.column.colId !== "slug" &&
					event.column.colId !== "cats") return;
				var content = event.value;
				if (event.column.colId === "summary") { content = atob(content); }
				swal({ title: event.column.colDef.headerName, html: content }); // deliberately leaving off type here
			},
	};`

	scriptBody := `new agGrid.Grid(document.querySelector('.articles-list-grid'), articleListGridOptions);`

	contentRenderer := `function ArticleListContentRenderer() {}
		ArticleListContentRenderer.prototype.init = function(params) {
			var content = atob(params.value)
			this.eGui = document.createElement('div');
			this.eGui.innerHTML = content;
		};
		ArticleListContentRenderer.prototype.getGui = function() {
			return this.eGui;
		};`

	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading").R(
			b.Text(m.Opts.Title),
			b.Wrap(func() {
				if m.Opts.IsAdmin {
					b.A("class", "btn-add", "href", m.GetNewURL(), "title", "Add Article").T("+")
				}
			}),
		),
		b.DivClass("list-wrapper").R(
			b.DivClass("articles-list-grid ag-theme-material", "style", `width: 98vw; height: calc(100vh - 226px)`),
			b.Script("type", "text/javascript").T(
				jsConvertColumnDefs+jsConvertRowData+contentRenderer+gridOptions+
					`$(document).ready(function() {`+scriptBody+`});`),
		),
	)

	return b.String()
}
