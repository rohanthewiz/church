package user

import (
	"github.com/rohanthewiz/church/chweb/module"
	"github.com/rohanthewiz/church/chweb/config"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/chweb/agrid"
	"encoding/json"
	"github.com/rohanthewiz/element"
)

const ModuleTypeUsersList = "users_list"

type ModuleUsersList struct {
	module.Presenter
}

func NewModuleUsersList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleUsersList)
	mod.Name = pres.Name
	mod.Opts = pres.Opts

	// Work out local condition
	cond := "1 = 1"
	if !mod.Opts.IsAdmin && !mod.Opts.ShowUnpublished{
		cond = "enabled = true"
	}
	// merge with any incoming condition
	if mod.Opts.Condition != "" {
		cond = mod.Opts.Condition + " AND " + cond
	}
	mod.Opts.Condition = cond

	return module.Module(mod), nil
}

func (m ModuleUsersList) GetData() ([]Presenter, error) {
	return QueryUsers(m.Opts.Condition, "first_name " + m.Order(), m.Opts.Limit, m.Opts.Offset)
}

type usersListRowDef struct {
	Id string `json:"id,omitempty"`
	Enabled string `json:"enabled,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	Username string `json:"username,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
	Role string `json:"role,omitempty"`
	UpdatedBy string `json:"updatedBy"`
	Delete string `json:"delete,omitempty"`
}

func (m *ModuleUsersList) Render(params map[string]map[string]string, loggedIn bool) string {
	usersEditURL := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/edit/"
	usersDeleteURL := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/delete/"
	newPath := config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/new"
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to this module
		m.SetLimitAndOffset(opts)
	}
	users, err := m.GetData()
	if err != nil {
		logger.LogErr(err, "Error obtaining data in module", "module_slug",  m.Opts.Slug,
			"module_type", m.Opts.ModuleType)
		return ""
	}


	// Setup AgGrid
	var columnDefs []agrid.ColumnDef
	if m.Opts.IsAdmin {
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Id", Field: "id", Width: 105 })
	}
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Enabled", Field: "enabled", Width: 190})
	columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "First Name", Field: "firstName", CellRenderer: "linkCellRenderer"})
	//columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Description", Field: "Summary", Width: 200, CellRenderer: "usersListRenderer"})
	if m.Opts.IsAdmin {
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Username", Field: "username"})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "EmailAddress", Field: "emailAddress"})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Role", Field: "role", Width: 196})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Enabled", Field: "enabled", Width: 190})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "Updated By", Field: "updatedBy", Width: 196})
		//columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "", Field: "edit", Width: 120, CellRenderer: "linkCellRenderer"})
		columnDefs = append(columnDefs, agrid.ColumnDef{HeaderName: "", Field: "delete", Width: 120, CellRenderer: "confirmLinkCellRenderer"})
	}

	var rowData []usersListRowDef
	for _, ser := range users {
		enabled := "disabled"
		if ser.Enabled { enabled = "enabled" }

		row := usersListRowDef{}
		row.Id = ser.Id
		row.Enabled = enabled
		row.FirstName = ser.Firstname + "|" + usersEditURL + ser.Id
		row.Username = ser.Username
		row.EmailAddress = ser.EmailAddress
		row.Role = RoleToString[ser.Role]
		row.UpdatedBy = ser.UpdatedBy
		// Edit via Firstname  //row.Edit = "edit|" + usersEditURL + ser.Id
		row.Delete = "del|" + usersDeleteURL + ser.Id
		rowData = append(rowData, row)
	}

	columnDefsAsJson, err := json.Marshal(columnDefs)
	rowDataAsJson, err := json.Marshal(rowData)
	if err != nil { logger.LogErr(err, "Error converting User column defs to JSON") }
	jsConvertColumnDefs := "var usersListColumnDefs = JSON.parse(`" + string(columnDefsAsJson) + "`);"
	jsConvertRowData := "var rowData = JSON.parse(`" + string(rowDataAsJson) + "`);"
	gridOptions := `var usersListGridOptions = {
			columnDefs: usersListColumnDefs, rowData: rowData,
			enableSorting: true, enableFilter: true,
			components: {
				'linkCellRenderer': chLinkCellRenderer, 'confirmLinkCellRenderer': chConfirmLinkCellRenderer },
			onGridReady: function() { usersListGridOptions.api.sizeColumnsToFit(); },
			onCellClicked: function(event) {
				console.log(event.column);
				if (event.column.colId !== "username" && event.column.colId !== "emailAddress" &&
					event.column.colId !== "role" && event.column.colId !== "updatedBy") return;
				var content = event.value;
				swal({ title: event.column.colDef.headerName, html: content }); // deliberately leaving off type here
			},
	};`

	scriptBody := `new agGrid.Grid(document.querySelector('.users-list-grid'), usersListGridOptions);`

	e := element.New
	estr := e("div", "class", "ch-module-wrapper ch-" + m.Opts.ModuleType).R(
		e("div", "class", "ch-module-heading").R(
			m.Opts.Title,
			func() (s string) {
				if m.Opts.IsAdmin {
					s = e("a", "class", "btn-add", "href", newPath, "title", "Add User").R("+")
				}
				return
			}(),
		),
		e("div", "class", "list-wrapper").R(
			e("div", "class", "users-list-grid ag-theme-material", "style", `width: 98vw; height: 780px`).R(),
			e("script", "type", "text/javascript").R(
				jsConvertColumnDefs, jsConvertRowData, gridOptions,
				`$(document).ready(function() {` + scriptBody + `});`),
		),
	)

	return estr
}
