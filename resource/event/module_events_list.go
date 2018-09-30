package event

import (
	"strings"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/config"
	"github.com/rohanthewiz/church/chweb/agrid"
	"encoding/json"
	"github.com/rohanthewiz/element"
)

const ModuleTypeEventsList = "events_list"

type ModuleEventsList struct {
	module.Presenter
}

func NewModuleEventsList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleEventsList)
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

func (m ModuleEventsList) getData() ([]Presenter, error) {
	return QueryEvents(m.Opts.Condition, "event_date " + m.Order(), m.Opts.Limit, m.Opts.Offset)
}
type eventsListRowDef struct {
	Id string `json:"id,omitempty"`
	Published string `json:"published,omitempty"`
	Slug string `json:"slug,omitempty"`
	Cats string `json:"cats,omitempty"`
	UpdatedBy string `json:"updatedBy"`
	Title string `json:"title"`
	EventDate string `json:"eventDate"`
	Edit string `json:"edit,omitempty"`
	Delete string `json:"delete"`
}

func (m *ModuleEventsList) Render(params map[string]map[string]string, loggedIn bool) string {
	eventsEditURL := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/edit/"
	eventsDeleteURL := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/delete/"
	newPath := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/new"

	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to this module
		m.SetLimitAndOffset(opts)
	}

	evts, err := m.getData()
	if err != nil {
		Log("Error", "Error obtaining data in module", "module_slug",  m.Opts.Slug,
				"module_type", m.Opts.ModuleType, "error", err.Error())
		return ""
	}

	// Setup AgGrid
	var columnDefs []agrid.ColumnDef
	if m.Opts.IsAdmin {
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Id", Field: "id", Width: 105 })
	}
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Event Date", Field: "eventDate", Width: 190})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Title", Field: "title", CellRenderer: "linkCellRenderer"})
	//columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Description", Field: "Summary", Width: 200, CellRenderer: "eventsListRenderer"})
	if m.Opts.IsAdmin {
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Slug", Field: "slug"})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Categories", Field: "cats"})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Updated By", Field: "updatedBy", Width: 196})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Published", Field: "published", Width: 190})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "", Field: "edit", Width: 120, CellRenderer: "linkCellRenderer"})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "", Field: "delete", Width: 120, CellRenderer: "confirmLinkCellRenderer"})
	}

	var rowData []eventsListRowDef
	for _, evt := range evts {
		published := "draft"
		if evt.Published { published = "published" }

		row := eventsListRowDef{}
		if m.Opts.IsAdmin {
			row.Id = evt.Id
		}
		row.EventDate = evt.EventDate
		row.Title = evt.Title + "|" +  "/" + m.Opts.ItemsURLPath + "/" + evt.Id //base64.StdEncoding.EncodeToString([]byte(title))
		//row.Summary = base64.StdEncoding.EncodeToString([]byte(evt.Summary))
		if m.Opts.IsAdmin {
			row.Slug = evt.Slug
			row.Cats = strings.Join(evt.Categories, ", ")
			row.UpdatedBy = evt.UpdatedBy
			row.Published = published
			row.Edit = "edit|" + eventsEditURL + evt.Id
			row.Delete = "del|" + eventsDeleteURL + evt.Id
		}
		rowData = append(rowData, row)
	}

	columnDefsAsJson, err := json.Marshal(columnDefs)
	rowDataAsJson, err := json.Marshal(rowData)
	if err != nil { LogErr(err, "Error converting Event column defs to JSON") }
	jsConvertColumnDefs := "var eventsListColumnDefs = JSON.parse(`" + string(columnDefsAsJson) + "`);"
	jsConvertRowData := "var rowData = JSON.parse(`" + string(rowDataAsJson) + "`);"
	gridOptions := `var eventsListGridOptions = {
			columnDefs: eventsListColumnDefs, rowData: rowData,
			enableSorting: true, enableFilter: true,
			components: {
				'linkCellRenderer': chLinkCellRenderer, 'confirmLinkCellRenderer': chConfirmLinkCellRenderer },
			onGridReady: function() { eventsListGridOptions.api.sizeColumnsToFit(); },
			onCellClicked: function(event) {
				console.log(event.column);
				if (event.column.colId !== "summary" && event.column.colId !== "slug" &&
					event.column.colId !== "cats") return;
				var content = event.value;
				if (event.column.colId === "summary") { content = atob(content); }
				swal({ title: event.column.colDef.headerName, html: content }); // deliberately leaving off type here
			},
	};`

	scriptBody := `new agGrid.Grid(document.querySelector('.events-list-grid'), eventsListGridOptions);`
	//eventsListRenderer := `function EventsListContentRenderer() {}
	//	EventsListContentRenderer.prototype.init = function(params) {
	//		var content = atob(params.value)
	//		this.eGui = document.createElement('div');
	//		this.eGui.innerHTML = content;
	//	};
	//	EventsListContentRenderer.prototype.getGui = function() {
	//		return this.eGui;
	//	};`

	e := element.New
	estr := e("div", "class", "ch-module-wrapper ch-" + m.Opts.ModuleType).R(
		e("div", "class", "ch-module-heading").R(
			m.Opts.Title,
			func() (s string) {
				if m.Opts.IsAdmin {
					s = e("a", "class", "btn-add", "href", newPath, "title", "Add Events").R("+")
				}
				return
			}(),
		),
		e("div", "class", "list-wrapper").R(
			e("div", "class", "events-list-grid ag-theme-material", "style", `width: 98vw; height: 780px`).R(),
			e("script", "type", "text/javascript").R(
				jsConvertColumnDefs, jsConvertRowData, gridOptions, //eventsListRenderer,
				`$(document).ready(function() {` + scriptBody + `});`),
		),
	)

	return estr
}
