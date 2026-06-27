package sermoncleanup

// Static CSS/JS for the Sermon Cleanup module, kept out of the render function for
// readability. The JS is plain (no jQuery dependency) so it works regardless of
// what the surrounding page chrome loads.

const cleanupCSS = `
.ch-` + ModuleTypeSermonCleanup + ` .sc-toolbar {
  display: flex; align-items: center; gap: 1rem; flex-wrap: wrap;
  margin: 0.5rem 0 1rem; padding: 0.5rem 0.75rem;
  background: rgba(0,0,0,0.04); border-radius: 4px;
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-summary { color: #555; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-delete-btn {
  margin-left: auto; padding: 0.4rem 0.9rem; cursor: pointer;
  background: #c0392b; color: #fff; border: none; border-radius: 4px;
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-delete-btn[disabled] { background: #bbb; cursor: not-allowed; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-year-group { margin-bottom: 1.5rem; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-year-heading { margin: 0.5rem 0; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-year-count { color: #888; font-weight: normal; font-size: 0.85em; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-table { width: 100%; border-collapse: collapse; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-table th,
.ch-` + ModuleTypeSermonCleanup + ` .sc-table td {
  text-align: left; padding: 0.35rem 0.5rem; border-bottom: 1px solid #eee; font-size: 0.9em;
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-cb-cell { width: 2rem; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-path { color: #666; font-family: monospace; font-size: 0.82em; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-size { white-space: nowrap; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-accessed { white-space: nowrap; color: #666; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-empty { color: #555; font-style: italic; }
`

const cleanupJS = `
function scUpdateCount() {
  var n = document.querySelectorAll('.sc-cb:checked').length;
  document.getElementById('sc-count').textContent = n;
  document.getElementById('sc-submit').disabled = (n === 0);
}
function scToggleAll(master) {
  var boxes = document.querySelectorAll('.sc-cb');
  for (var i = 0; i < boxes.length; i++) { boxes[i].checked = master.checked; }
  var yearMasters = document.querySelectorAll('.sc-year-master');
  for (var j = 0; j < yearMasters.length; j++) { yearMasters[j].checked = master.checked; }
  scUpdateCount();
}
function scToggleYear(master) {
  var year = master.getAttribute('data-year');
  var boxes = document.querySelectorAll('.sc-cb[data-year="' + year + '"]');
  for (var i = 0; i < boxes.length; i++) { boxes[i].checked = master.checked; }
  scUpdateCount();
}
function scPrepare() {
  var boxes = document.querySelectorAll('.sc-cb:checked');
  if (boxes.length === 0) { return false; }
  var specs = [];
  for (var i = 0; i < boxes.length; i++) { specs.push(boxes[i].value); }
  document.getElementById('selected_specs').value = specs.join('\n');
  return confirm('Delete ' + boxes.length + ' local sermon copy(ies)?\n\nThe verified copies on IDrive e2 are kept; only the local cache is removed.');
}
`
