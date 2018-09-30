package page

import (
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/config"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/church/chweb/agrid"
	"encoding/json"
)

const ModuleTypePagesList = "pages_list"

type ModulePagesList struct {
	module.Presenter
}

func NewModulePagesList(pres module.Presenter) (module.Module, error) {
	mod := new(ModulePagesList)
	mod.Name = pres.Name
	mod.Opts = pres.Opts

	// Work out local condition
	cond := "1 = 1"
	if !mod.Opts.IsAdmin && !mod.Opts.ShowUnpublished{
		cond = "published = true"
	}
	// merge with any incoming condition
	if mod.Opts.Condition != "" {
		cond = mod.Opts.Condition + " AND " + cond
	}
	mod.Opts.Condition = cond

	return module.Module(mod), nil
}

func (m ModulePagesList) GetData() ([]Presenter, error) {
	return queryPages(m.Opts.Condition, "updated_at " + m.Order(), m.Opts.Limit, m.Opts.Offset)
}

type pagesListRowDef struct {
	Id string `json:"id,omitempty"`
	Title string `json:"title"`
	PageURL string `json:"pageURL"`
	Published string `json:"published,omitempty"`
	UpdatedBy string `json:"updatedBy"`
	Edit string `json:"edit,omitempty"`
	Delete string `json:"delete"`
}

func (m *ModulePagesList) Render(params map[string]map[string]string, loggedIn bool) string {
	pagesEditURL := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/edit/"
	pagesDeleteURL := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/delete/"
	newPath := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/new"

	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	pgs, err := m.GetData()
	if err != nil {
		LogErr(err, "Error obtaining data in module", "module_slug",  m.Opts.Slug,
			"module_type", m.Opts.ModuleType)
		return ""
	}

	// Setup AgGrid
	var columnDefs []agrid.ColumnDef
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Id", Field: "id", Width: 105 })
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Title", Field: "title", CellRenderer: "linkCellRenderer"})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Page URL", Field: "pageURL"})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Published", Field: "published", Width: 190})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Updated By", Field: "updatedBy", Width: 196})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "", Field: "edit", Width: 120, CellRenderer: "linkCellRenderer"})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "", Field: "delete", Width: 120, CellRenderer: "confirmLinkCellRenderer"})

	var rowData []pagesListRowDef
	for _, pg := range pgs {
		published := "draft"
		if pg.Published { published = "published" }

		row := pagesListRowDef{}
		row.Id = pg.Id
		row.Title = pg.Title + "|" +  "/admin/" + m.Opts.ItemsURLPath + "/" + pg.Id
		row.PageURL = "/pages/" + pg.Slug
		row.Published = published
		row.UpdatedBy = pg.UpdatedBy
		row.Edit = "edit|" + pagesEditURL + pg.Id
		row.Delete = "del|" + pagesDeleteURL + pg.Id
		rowData = append(rowData, row)
	}

	columnDefsAsJson, err := json.Marshal(columnDefs)
	rowDataAsJson, err := json.Marshal(rowData)
	if err != nil { LogErr(err, "Error converting Page column defs to JSON") }
	jsConvertColumnDefs := "var pagesListColumnDefs = JSON.parse(`" + string(columnDefsAsJson) + "`);"
	jsConvertRowData := "var rowData = JSON.parse(`" + string(rowDataAsJson) + "`);"
	gridOptions := `var pagesListGridOptions = {
			columnDefs: pagesListColumnDefs, rowData: rowData,
			enableSorting: true, enableFilter: true,
			components: {
				'linkCellRenderer': chLinkCellRenderer, 'confirmLinkCellRenderer': chConfirmLinkCellRenderer },
			onGridReady: function() { pagesListGridOptions.api.sizeColumnsToFit(); },
			onCellClicked: function(event) {
				if (event.column.colId !== "pageURL" && event.column.colId !== "published") return;
				var content = event.value;
				swal({ title: event.column.colDef.headerName, html: content }); // deliberately leaving off type here
			},
	};`

	scriptBody := `new agGrid.Grid(document.querySelector('.list-grid'), pagesListGridOptions);`

	e := element.New
	out := e("div", "class", "ch-module-wrapper ch-" + m.Opts.ModuleType).R(
		e("div", "class", "ch-module-heading").R(
			m.Opts.Title,
			func() (s string) {
				if m.Opts.IsAdmin {
					s = e("a", "class", "btn-add", "href", newPath, "title", "Add Page").R("+")
				}
				return
			}(),
		),
		e("div", "class", "list-wrapper").R(
			e("div", "class", "list-grid ag-theme-material", "style", `width: 98vw; height: 780px`).R(),
			e("script", "type", "text/javascript").R(
				jsConvertColumnDefs, jsConvertRowData, gridOptions,
				`$(document).ready(function() {` + scriptBody + `});`),
		),
	)

	return out
}
