package sermoncleanup

// Static CSS/JS for the Sermon Cleanup module, kept out of the render function for
// readability. The JS is plain (no jQuery dependency) so it works regardless of
// what the surrounding page chrome loads.
//
// Visual language: a light card per year group with a subtle green accent
// (--sc-green) that ties the toolbar, year headings, caret and row hovers
// together. Green reads as "safe / reclaim space"; the single destructive action
// (the delete button) stays red so it remains visually distinct.

const cleanupCSS = `
.ch-` + ModuleTypeSermonCleanup + ` {
  --sc-green: #2e8b57;
  --sc-green-dark: #246b44;
  --sc-green-soft: #eaf5ee;
  --sc-border: #e3e8e4;
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-toolbar {
  display: flex; align-items: center; gap: 1rem; flex-wrap: wrap;
  margin: 0.5rem 0 1.25rem; padding: 0.6rem 0.9rem;
  background: var(--sc-green-soft);
  border: 1px solid var(--sc-border);
  border-left: 4px solid var(--sc-green);
  border-radius: 6px;
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-selectall {
  display: inline-flex; align-items: center; gap: 0.35rem;
  font-weight: 600; color: var(--sc-green-dark); cursor: pointer;
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-summary { color: #555; font-size: 0.9em; }
.ch-` + ModuleTypeSermonCleanup + ` #sc-count { font-weight: 700; color: var(--sc-green-dark); }
.ch-` + ModuleTypeSermonCleanup + ` input[type="checkbox"] { accent-color: var(--sc-green); }

.ch-` + ModuleTypeSermonCleanup + ` .sc-delete-btn {
  margin-left: auto; padding: 0.45rem 1rem; cursor: pointer;
  background: #c0392b; color: #fff; border: none; border-radius: 5px;
  font-weight: 600; letter-spacing: 0.2px;
  box-shadow: 0 1px 2px rgba(0,0,0,0.15);
  transition: background 0.15s, opacity 0.15s;
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-delete-btn:hover { background: #a93226; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-delete-btn[disabled] {
  background: #c2c8c4; cursor: not-allowed; box-shadow: none;
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-delete-btn.sc-deleting {
  background: var(--sc-green); cursor: progress; opacity: 0.85;
}

.ch-` + ModuleTypeSermonCleanup + ` .sc-year-group {
  margin-bottom: 1rem; border: 1px solid var(--sc-border);
  border-radius: 6px; overflow: hidden; background: #fff;
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-year-heading {
  display: flex; align-items: center; gap: 0.5rem; margin: 0;
  padding: 0.5rem 0.75rem; font-size: 1.05em;
  background: linear-gradient(0deg, #fafdfb, #f2f8f4);
  border-bottom: 1px solid var(--sc-border);
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-year-title {
  display: inline-flex; align-items: center; gap: 0.45rem;
  cursor: pointer; color: var(--sc-green-dark); user-select: none;
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-caret {
  display: inline-block; color: var(--sc-green); font-size: 0.8em;
  transition: transform 0.15s ease;
}
/* Caret points right when collapsed, rotates down when the group is open. */
.ch-` + ModuleTypeSermonCleanup + ` .sc-year-group:not(.sc-collapsed) .sc-caret { transform: rotate(90deg); }
.ch-` + ModuleTypeSermonCleanup + ` .sc-year-group.sc-collapsed .sc-table { display: none; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-year-count {
  color: #777; font-weight: 500; font-size: 0.8em;
  padding: 0.05rem 0.45rem; margin-left: 0.15rem;
  background: var(--sc-green-soft); border-radius: 999px;
}

.ch-` + ModuleTypeSermonCleanup + ` .sc-table { width: 100%; border-collapse: collapse; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-table th {
  text-align: left; padding: 0.4rem 0.6rem; font-size: 0.78em;
  text-transform: uppercase; letter-spacing: 0.4px; color: var(--sc-green-dark);
  border-bottom: 2px solid var(--sc-green-soft);
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-table td {
  text-align: left; padding: 0.4rem 0.6rem; border-bottom: 1px solid #f0f2f0; font-size: 0.9em;
}
.ch-` + ModuleTypeSermonCleanup + ` .sc-table tbody tr:hover { background: var(--sc-green-soft); }
.ch-` + ModuleTypeSermonCleanup + ` .sc-cb-cell { width: 2rem; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-file { font-weight: 500; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-path { color: #666; font-family: monospace; font-size: 0.82em; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-size { white-space: nowrap; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-accessed { white-space: nowrap; color: #666; }
.ch-` + ModuleTypeSermonCleanup + ` .sc-mtime-tag {
  margin-left: 0.4rem; padding: 0.02rem 0.4rem; font-size: 0.72em; font-style: italic;
  color: #8a6d3b; background: #fcf3e3; border-radius: 999px; vertical-align: middle;
}
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
// scToggleFold collapses/expands a single year group. It walks up from the clicked
// title to the enclosing .sc-year-group and toggles the .sc-collapsed class; the CSS
// hides the table and rotates the caret based on that class.
function scToggleFold(titleEl) {
  var group = titleEl.closest('.sc-year-group');
  if (group) { group.classList.toggle('sc-collapsed'); }
}
function scPrepare() {
  var boxes = document.querySelectorAll('.sc-cb:checked');
  if (boxes.length === 0) { return false; }
  var specs = [];
  for (var i = 0; i < boxes.length; i++) { specs.push(boxes[i].value); }
  document.getElementById('selected_specs').value = specs.join('\n');
  if (!confirm('Delete ' + boxes.length + ' local sermon copy(ies)?\n\nThe verified copies on IDrive e2 are kept; only the local cache is removed.')) {
    return false;
  }
  // Submission is now committed: reflect the in-flight delete on the button so the
  // admin gets immediate feedback until the POST completes and the page reloads.
  var btn = document.getElementById('sc-submit');
  btn.textContent = 'Deleting…';
  btn.disabled = true;
  btn.classList.add('sc-deleting');
  return true;
}
`
