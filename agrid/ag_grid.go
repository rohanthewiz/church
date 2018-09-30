package agrid


type ColumnDef struct {
	HeaderName string `json:"headerName"`
	Field string `json:"field"`
	CellRenderer string `json:"cellRenderer"`
	Width int `json:"width"`
}

// Some useful functions for setting up AgGrid these should run probably in head
var LinkCellRenderer = `function chLinkCellRenderer() {}
		chLinkCellRenderer.prototype.init = function(params) {
			var label, href = '#';
			var arr = params.value.split('|');
			label = arr[0];
			if(arr.length > 1) { href = arr[1]; } 
			this.eGui = document.createElement('span');
			this.eGui.innerHTML = '<a href="' + href + '">' + label + '</a>';
		};
		chLinkCellRenderer.prototype.getGui = function() {
			return this.eGui;
		};`

var ConfirmlinkCellRenderer = `function chConfirmLinkCellRenderer() {}
		chConfirmLinkCellRenderer.prototype.init = function(params) {
			var label, href = '#';
			var arr = params.value.split('|');
			label = arr[0];
			if(arr.length > 1) { href = arr[1]; } 
			this.eGui = document.createElement('span');
			this.eGui.innerHTML = '<a href="#" data-url="' + href + '" onclick="chConfirmDelete()">' + label + '</a>';
		};
		chConfirmLinkCellRenderer.prototype.getGui = function() {
			return this.eGui;
		};`

var ConfirmDelete = `function chConfirmDelete() {
	var targetUrl = event.target.dataset.url
	swal({
	  title: 'Are you sure?',
	  text: "You won't be able to undo!",
	  type: 'warning',
	  showCancelButton: true,
	  confirmButtonColor: '#3085d6',
	  cancelButtonColor: '#d33',
	  confirmButtonText: 'Yes, delete it!'
	}).then((result) => {
	  if (result.value) {
	window.location = targetUrl
		return true;
	  } else { return false; }
	});
	}`



