package menu

import (
	"encoding/json"

	"github.com/rohanthewiz/church/agrid"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
)

const ModuleTypeMenusList = "menus_list"

type ModuleMenusList struct {
	module.Presenter
}

func NewModuleMenusList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleMenusList)
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

func (m ModuleMenusList) GetData() ([]MenuDef, error) {
	return queryMenus(m.Opts.Condition, "updated_at "+m.Order(), m.Opts.Limit, m.Opts.Offset)
}

type menusListRowDef struct {
	Id        string `json:"id,omitempty"`
	Published string `json:"published,omitempty"`
	Title     string `json:"title"`
	Slug      string `json:"slug,omitempty"`
	UpdatedBy string `json:"updatedBy"`
	Edit      string `json:"edit,omitempty"`
	Delete    string `json:"delete"`
}

func (m *ModuleMenusList) Render(params map[string]map[string]string, loggedIn bool) string {
	menusEditUrl := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/edit/"
	menusDeleteUrl := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/delete/"
	newPath := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/new"

	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	mnus, err := m.GetData()
	if err != nil {
		logger.LogErr(err, "Error obtaining data in module", "module_slug", m.Opts.Slug,
			"module_type", m.Opts.ModuleType)
		return ""
	}

	// Setup AgGrid
	var columnDefs []agrid.ColumnDef
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Id", Field: "id", Width: 105})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Published", Field: "published", Width: 190})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Title", Field: "title", CellRenderer: "linkCellRenderer"})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Slug", Field: "slug"})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Updated By", Field: "updatedBy", Width: 196})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "", Field: "edit", Width: 120, CellRenderer: "linkCellRenderer"})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "", Field: "delete", Width: 120, CellRenderer: "confirmLinkCellRenderer"})

	var rowData []menusListRowDef
	for _, mnu := range mnus {
		published := "draft"
		if mnu.Published {
			published = "published"
		}

		row := menusListRowDef{}
		if m.Opts.IsAdmin {
			row.Id = mnu.Id
		}
		row.Title = mnu.Title + "|" + menusEditUrl + mnu.Id // base64.StdEncoding.EncodeToString([]byte(title))
		row.Slug = mnu.Slug
		// row.Summary = base64.StdEncoding.EncodeToString([]byte(mnu.Summary))
		if m.Opts.IsAdmin {
			row.Slug = mnu.Slug
			row.UpdatedBy = mnu.UpdatedBy
			row.Published = published
			row.Edit = "edit|" + menusEditUrl + mnu.Id
			row.Delete = "del|" + menusDeleteUrl + mnu.Id
		}
		rowData = append(rowData, row)
	}

	columnDefsAsJson, err := json.Marshal(columnDefs)
	rowDataAsJson, err := json.Marshal(rowData)
	if err != nil {
		logger.LogErr(err, "Error converting Menu column defs to JSON")
	}
	jsConvertColumnDefs := "var menusListColumnDefs = JSON.parse(`" + string(columnDefsAsJson) + "`);"
	jsConvertRowData := "var rowData = JSON.parse(`" + string(rowDataAsJson) + "`);"
	gridOptions := `var menusListGridOptions = {
			columnDefs: menusListColumnDefs, rowData: rowData,
			enableSorting: true, enableFilter: true,
			components: {
				'linkCellRenderer': chLinkCellRenderer, 'confirmLinkCellRenderer': chConfirmLinkCellRenderer },
			onGridReady: function() { menusListGridOptions.api.sizeColumnsToFit(); },
			onCellClicked: function(event) {
				console.log(event.column);
				if (event.column.colId !== "summary" && event.column.colId !== "slug" &&
					event.column.colId !== "cats") return;
				var content = event.value;
				swal({ title: event.column.colDef.headerName, html: content }); // deliberately leaving off type here
			},
	};`

	scriptBody := `new agGrid.Grid(document.querySelector('.menu-list-grid'), menusListGridOptions);`

	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading").R(
			b.T(m.Opts.Title),
			b.Wrap(func() {
				if m.Opts.IsAdmin {
					b.AClass("btn-add", "href", newPath, "title", "Add Menu").T("+")
				}
			}),
		),
		b.DivClass("list-wrapper").R(
			b.DivClass("menu-list-grid ag-theme-material", "style", `width: 98vw; height: calc(100vh - 226px)`).R(),
			b.Script("type", "text/javascript").T(
				jsConvertColumnDefs+jsConvertRowData+gridOptions+
					`$(document).ready(function() {`+scriptBody+`});`),
		),
	)

	return b.String()
}
