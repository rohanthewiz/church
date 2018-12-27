package sermon

import (
	"encoding/json"
	"github.com/rohanthewiz/church/agrid"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"strconv"
	"strings"
)

const ModuleTypeSermonsList = "sermons_list"

type ModuleSermonsList struct {
	module.Presenter
}

func NewModuleSermonsList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleSermonsList)
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

func (m ModuleSermonsList) GetData() ([]Presenter, error) {
	return QuerySermons(m.Opts.Condition, "date_taught " + m.Order(), m.Opts.Limit, m.Opts.Offset)
}

type sermonsListRowDef struct {
	Id string `json:"id,omitempty"`
	Published string `json:"published,omitempty"`
	Slug string `json:"slug,omitempty"`
	Cats string `json:"cats,omitempty"`
	UpdatedBy string `json:"updatedBy"`
	Title string `json:"title"`
	DateTaught string `json:"dateTaught"`
	Edit string `json:"edit,omitempty"`
	Delete string `json:"delete"`
}

func (m *ModuleSermonsList) Render(params map[string]map[string]string, loggedIn bool) string {
	sermonsEditURL := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/edit/"
	sermonsDeleteURL := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/delete/"
	newPath := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/new"

	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	sermons, err := m.GetData()
	if err != nil {
		logger.LogErr(err, "Error obtaining data in module", "module_slug",  m.Opts.Slug,
			"module_type", m.Opts.ModuleType)
		return ""
	}
	if len(sermons) == 0 { logger.Log("Warn", "No sermons found")
	} else {
		logger.Log("Info", strconv.Itoa(len(sermons)) + " sermon(s) found")
	}


	// Setup AgGrid
	var columnDefs []agrid.ColumnDef
	if m.Opts.IsAdmin {
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Id", Field: "id", Width: 105 })
	}
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Date Preached", Field: "dateTaught", Width: 190})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Title", Field: "title", CellRenderer: "linkCellRenderer"})
	//columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Description", Field: "Summary", Width: 200, CellRenderer: "sermonsListRenderer"})
	if m.Opts.IsAdmin {
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Slug", Field: "slug"})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Categories", Field: "cats"})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Updated By", Field: "updatedBy", Width: 196})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Published", Field: "published", Width: 190})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "", Field: "edit", Width: 120, CellRenderer: "linkCellRenderer"})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "", Field: "delete", Width: 120, CellRenderer: "confirmLinkCellRenderer"})
	}

	var rowData []sermonsListRowDef
	for _, ser := range sermons {
		published := "draft"
		if ser.Published { published = "published" }

		row := sermonsListRowDef{}
		if m.Opts.IsAdmin {
			row.Id = ser.Id
		}
		row.DateTaught = ser.DateTaught
		row.Title = ser.Title + "|" +  "/" + m.Opts.ItemsURLPath + "/" + ser.Id //base64.StdEncoding.EncodeToString([]byte(title))
		//row.Summary = base64.StdEncoding.EncodeToString([]byte(ser.Summary))
		if m.Opts.IsAdmin {
			row.Slug = ser.Slug
			row.Cats = strings.Join(ser.Categories, ", ")
			row.UpdatedBy = ser.UpdatedBy
			row.Published = published
			row.Edit = "edit|" + sermonsEditURL + ser.Id
			row.Delete = "del|" + sermonsDeleteURL + ser.Id
		}
		rowData = append(rowData, row)
	}

	columnDefsAsJson, err := json.Marshal(columnDefs)
	rowDataAsJson, err := json.Marshal(rowData)
	if err != nil { logger.LogErr(err, "Error converting Sermon column defs to JSON") }
	jsConvertColumnDefs := "var sermonsListColumnDefs = JSON.parse(`" + string(columnDefsAsJson) + "`);"
	jsConvertRowData := "var rowData = JSON.parse(`" + string(rowDataAsJson) + "`);"
	gridOptions := `var sermonsListGridOptions = {
			columnDefs: sermonsListColumnDefs, rowData: rowData,
			enableSorting: true, enableFilter: true,
			components: {
				'linkCellRenderer': chLinkCellRenderer, 'confirmLinkCellRenderer': chConfirmLinkCellRenderer },
			onGridReady: function() { sermonsListGridOptions.api.sizeColumnsToFit(); },
			onCellClicked: function(event) {
				console.log(event.column);
				if (event.column.colId !== "summary" && event.column.colId !== "slug" &&
					event.column.colId !== "cats") return;
				var content = event.value;
				if (event.column.colId === "summary") { content = atob(content); }
				swal({ title: event.column.colDef.headerName, html: content }); // deliberately leaving off type here
			},
	};`

	scriptBody := `new agGrid.Grid(document.querySelector('.sermons-list-grid'), sermonsListGridOptions);`
	//sermonsListRenderer := `function SermonsListContentRenderer() {}
	//	SermonsListContentRenderer.prototype.init = function(params) {
	//		var content = atob(params.value)
	//		this.eGui = document.createElement('div');
	//		this.eGui.innerHTML = content;
	//	};
	//	SermonsListContentRenderer.prototype.getGui = function() {
	//		return this.eGui;
	//	};`

	e := element.New
	estr := e("div", "class", "ch-module-wrapper ch-" + m.Opts.ModuleType).R(
		e("div", "class", "ch-module-heading").R(
			m.Opts.Title,
			func() (s string) {
				if m.Opts.IsAdmin {
					s = e("a", "class", "btn-add", "href", newPath, "title", "Add Sermon").R("+")
				}
				return
			}(),
		),
		e("div", "class", "ch-sermons-list-wrapper").R(
			e("div", "class", "sermons-list-grid ag-theme-material", "style", `width: 98vw; height: calc(100vh - 310px)`).R(),
			e("script", "type", "text/javascript").R(
				jsConvertColumnDefs, jsConvertRowData, gridOptions, //sermonsListRenderer,
				`$(document).ready(function() {` + scriptBody + `});`),
		),
	)

	return estr
}
