package grid

// Static CSS/JS for the grid, injected once per page from template.Page (the
// same slot where the AG Grid bundle and its cell renderers used to load).
// The JS is plain (no jQuery dependency) and initializes every .ch-grid on the
// page independently, so any number of grids can coexist.
//
// Progressive enhancement: toolbar, per-column filter row, and the client pager
// carry .ch-grid-jsonly and stay hidden until init adds .ch-grid-ready to the
// wrapper. Without JS the grid is still a complete, readable table, and the
// server-side Prev/Next links (plain anchors) still page through the data.

// CSS uses a neutral palette driven by custom properties so a site theme can
// re-skin grids by overriding the variables on .ch-grid.
const CSS = `
.ch-grid {
  --chg-accent: #3f6ea5;
  --chg-accent-fg: #fff;     /* text on accent-colored surfaces */
  --chg-accent-soft: #eef3f9;
  --chg-border: #e2e6ea;
  --chg-bg: #fff;            /* table + control surfaces */
  --chg-bg-alt: #fbfcfd;     /* zebra stripe */
  --chg-head-bg: #f7f9fb;
  --chg-head-fg: #345;
  --chg-hover: #f2f6fb;
  --chg-muted: #667;
  --chg-row-border: #f0f2f4;
  --chg-danger: #c0392b;
  --chg-scroll-thumb: #c9d2da;
  --chg-scroll-thumb-hover: #a9b6c2;
  /* Data grids read best in the platform UI font; sites that want the grid to
     blend with their body font can set --chg-font: inherit. */
  --chg-font: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  width: 100%;
  font-family: var(--chg-font);
  font-size: 0.92em;
  /* Without this, macOS renders light-on-dark text noticeably heavier
     (looks bolded on dark themes). */
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}
/* Form controls don't inherit font by default — pull them onto the grid font. */
.ch-grid input, .ch-grid button, .ch-grid select { font-family: inherit; }
.ch-grid:not(.ch-grid-ready) .ch-grid-jsonly { display: none; }

.ch-grid .ch-grid-toolbar {
  display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap;
  margin: 0.5rem 0; padding: 0.5rem 0.75rem;
  background: var(--chg-accent-soft);
  border: 1px solid var(--chg-border);
  border-left: 2px solid var(--chg-accent);
  border-radius: 6px;
}
.ch-grid .ch-grid-search {
  padding: 0.35rem 0.6rem; min-width: 220px;
  background: var(--chg-bg); color: inherit;
  border: 1px solid var(--chg-border); border-radius: 4px;
}
.ch-grid .ch-grid-groupbtn {
  padding: 0.35rem 0.8rem; cursor: pointer;
  background: var(--chg-bg); color: var(--chg-accent);
  border: 1px solid var(--chg-accent); border-radius: 4px; font-weight: 600;
}
.ch-grid .ch-grid-groupbtn.ch-grid-active { background: var(--chg-accent); color: var(--chg-accent-fg); }
.ch-grid .ch-grid-count { margin-left: auto; color: var(--chg-muted); font-size: 0.9em; }

/* The scroll container gives large grids their own scrollbar (with a sticky
   header) instead of stretching the page — the layout fix over AG Grid's
   fixed-height viewport divs. */
.ch-grid .ch-grid-scroll {
  overflow: auto; max-height: calc(100vh - 240px);
  border: 1px solid var(--chg-border); border-radius: 6px; background: var(--chg-bg);
  /* Slim theme-aware scrollbar. Firefox and Chrome 121+ take the standard
     properties (and then ignore the -webkit rules); Safari and older Chrome
     take the -webkit rules below. */
  scrollbar-width: thin;
  scrollbar-color: var(--chg-scroll-thumb) transparent;
}
.ch-grid .ch-grid-scroll::-webkit-scrollbar { width: 10px; height: 10px; }
.ch-grid .ch-grid-scroll::-webkit-scrollbar-track { background: transparent; }
.ch-grid .ch-grid-scroll::-webkit-scrollbar-thumb {
  /* Transparent border + padding-box clip floats the thumb off the edge,
     giving a pill look without a visible track. */
  background: var(--chg-scroll-thumb);
  border: 3px solid transparent; background-clip: padding-box;
  border-radius: 999px;
}
.ch-grid .ch-grid-scroll::-webkit-scrollbar-thumb:hover { background-color: var(--chg-scroll-thumb-hover); }
.ch-grid .ch-grid-scroll::-webkit-scrollbar-corner { background: transparent; }
.ch-grid table.ch-grid-table { width: 100%; border-collapse: separate; border-spacing: 0; }
.ch-grid .ch-grid-table th {
  position: sticky; top: 0; z-index: 1;
  background: var(--chg-head-bg);
  text-align: left; padding: 0.5rem 0.6rem;
  font-size: 0.8em; text-transform: uppercase; letter-spacing: 0.4px;
  color: var(--chg-head-fg); border-bottom: 2px solid var(--chg-border);
  white-space: nowrap;
}
.ch-grid .ch-grid-table th.ch-grid-shrink { width: 1%; }
.ch-grid .ch-grid-table th.ch-grid-sortable { cursor: pointer; user-select: none; }
.ch-grid .ch-grid-table th.ch-grid-sortable:hover { color: var(--chg-accent); }
.ch-grid .ch-grid-sort-ind { color: var(--chg-accent); font-size: 0.9em; padding-left: 0.25rem; }

/* Filter row sits under the sticky header row; sticky too so filters stay
   visible while scrolling. 2.1em ≈ the header row's rendered height.
   Shares --chg-head-bg so title row + filter row read as one themed header
   block (a hard-coded #fff here left a white bar on dark themes). */
.ch-grid .ch-grid-filter-row th { top: 2.1em; padding: 0.25rem 0.4rem; background: var(--chg-head-bg); }
.ch-grid .ch-grid-colfilter {
  width: 100%; box-sizing: border-box; padding: 0.25rem 0.4rem;
  background: var(--chg-bg); color: inherit;
  border: 1px solid var(--chg-border); border-radius: 4px; font-size: 0.95em;
}

.ch-grid .ch-grid-table td {
  padding: 0.45rem 0.6rem; border-bottom: 1px solid var(--chg-row-border); vertical-align: top;
  /* Pin the weight so site-theme table rules can't bold row content;
     inline <b>/<strong> in HTML cells still render bold. */
  font-weight: 400;
}
.ch-grid .ch-grid-table tbody tr:hover { background: var(--chg-hover); }
.ch-grid .ch-grid-table tbody tr:nth-child(even):not(:hover) { background: var(--chg-bg-alt); }
.ch-grid .ch-grid-table a { color: var(--chg-accent); text-decoration: none; }
.ch-grid .ch-grid-table a:hover { text-decoration: underline; }
.ch-grid a.ch-grid-del { color: var(--chg-danger); }
.ch-grid td.ch-grid-popup { cursor: pointer; }
.ch-grid .ch-grid-empty-row td { color: var(--chg-muted); font-style: italic; text-align: center; padding: 1.2rem; }

/* HTML cells (e.g. article summaries) are clamped in the row; the popup shows
   the full content. */
.ch-grid .ch-grid-html { max-height: 4.5em; overflow: hidden; max-width: 32rem; }

/* Year group header rows (client-side grouping) */
.ch-grid tr.ch-grid-year-row td {
  cursor: pointer; user-select: none; font-weight: 700; color: var(--chg-accent);
  background: var(--chg-accent-soft); border-bottom: 1px solid var(--chg-border);
}
.ch-grid .ch-grid-caret { display: inline-block; font-size: 0.75em; transition: transform 0.15s ease; }
.ch-grid tr.ch-grid-year-row:not(.ch-grid-collapsed) .ch-grid-caret { transform: rotate(90deg); }
.ch-grid .ch-grid-year-count {
  font-weight: 500; font-size: 0.8em; color: var(--chg-muted);
  background: var(--chg-bg); border-radius: 999px; padding: 0.05rem 0.45rem; margin-left: 0.3rem;
}

.ch-grid .ch-grid-pager, .ch-grid .ch-grid-serverpager {
  display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap;
  margin: 0.5rem 0; color: var(--chg-muted);
}
.ch-grid .ch-grid-pager button {
  padding: 0.25rem 0.7rem; cursor: pointer;
  background: var(--chg-bg); color: inherit;
  border: 1px solid var(--chg-border); border-radius: 4px;
}
.ch-grid .ch-grid-pager button:disabled { opacity: 0.45; cursor: default; }
.ch-grid .ch-grid-pager select {
  padding: 0.2rem 0.3rem; background: var(--chg-bg); color: inherit;
  border: 1px solid var(--chg-border); border-radius: 4px;
}
.ch-grid .ch-grid-serverpager a { color: var(--chg-accent); font-weight: 600; text-decoration: none; }
`

// JS implements the interactive behavior. One chGridInit() per .ch-grid.
//
// Core model: the tbody's rows are captured once into allRows; every render()
// recomputes filter → sort → (group | page) and attaches only the rows that
// should be visible. Detached rows keep their DOM nodes (and any listeners),
// so re-attachment is cheap — this is the "lazy" row handling that keeps big
// tables snappy.
const JS = `
(function() {
'use strict';

function chGridInit(root) {
  var table = root.querySelector('.ch-grid-table');
  if (!table || !table.tHead || !table.tHead.rows.length) return;
  var tbody = table.tBodies[0];
  var headerCells = Array.prototype.slice.call(table.tHead.rows[0].cells);
  var allRows = Array.prototype.slice.call(tbody.rows).filter(function(tr) {
    return !tr.classList.contains('ch-grid-empty-row');
  });
  var groupCol = root.hasAttribute('data-group-col') ? parseInt(root.getAttribute('data-group-col'), 10) : -1;

  var state = {
    sorts: [],          // [{idx, dir}] in priority order; dir 1 = asc, -1 = desc
    colFilters: {},     // column index -> lowercased needle
    quick: '',          // toolbar search box, matched against the whole row
    page: 0,
    pageSize: parseInt(root.getAttribute('data-page-size'), 10) > 0
      ? parseInt(root.getAttribute('data-page-size'), 10) : 25,
    grouped: false,
    collapsed: {}       // year -> explicit collapsed state once the user toggles
  };

  function cellVal(tr, idx) {
    var td = tr.cells[idx];
    if (!td) return '';
    return td.getAttribute('data-sort') || td.textContent.trim();
  }

  function compare(a, b, type) {
    if (type === 'num') {
      var na = parseFloat(a), nb = parseFloat(b);
      if (isNaN(na)) na = -Infinity;
      if (isNaN(nb)) nb = -Infinity;
      return na - nb;
    }
    // ISO dates (yyyy-mm-dd) order correctly as strings, so date === text here
    return a < b ? -1 : (a > b ? 1 : 0);
  }

  function filteredRows() {
    return allRows.filter(function(tr) {
      for (var idx in state.colFilters) {
        var needle = state.colFilters[idx];
        if (!needle) continue;
        var td = tr.cells[idx];
        if (!td || td.textContent.toLowerCase().indexOf(needle) < 0) return false;
      }
      if (state.quick && tr.textContent.toLowerCase().indexOf(state.quick) < 0) return false;
      return true;
    });
  }

  function sortRows(rows) {
    if (!state.sorts.length) return rows;
    var sorted = rows.slice();
    sorted.sort(function(ra, rb) {
      for (var i = 0; i < state.sorts.length; i++) {
        var s = state.sorts[i];
        var type = headerCells[s.idx] ? headerCells[s.idx].getAttribute('data-type') : 'text';
        var c = compare(cellVal(ra, s.idx).toLowerCase(), cellVal(rb, s.idx).toLowerCase(), type);
        if (c !== 0) return c * s.dir;
      }
      return allRows.indexOf(ra) - allRows.indexOf(rb); // stable: fall back to original order
    });
    return sorted;
  }

  function isCollapsed(year, groupIndex) {
    // First group open by default (matches the sermon-cleanup convention);
    // a user toggle overrides the default from then on.
    return state.collapsed.hasOwnProperty(year) ? state.collapsed[year] : groupIndex > 0;
  }

  function render() {
    var rows = sortRows(filteredRows());
    var frag = document.createDocumentFragment();
    var label;

    if (state.grouped && groupCol >= 0) {
      var groups = {}, order = [];
      rows.forEach(function(tr) {
        var y = tr.getAttribute('data-year') || 'Other';
        if (!groups[y]) { groups[y] = []; order.push(y); }
        groups[y].push(tr);
      });
      order.forEach(function(y, gi) {
        var collapsed = isCollapsed(y, gi);
        var hdr = document.createElement('tr');
        hdr.className = 'ch-grid-year-row' + (collapsed ? ' ch-grid-collapsed' : '');
        var td = document.createElement('td');
        td.colSpan = headerCells.length;
        td.innerHTML = '<span class="ch-grid-caret">▶</span> ' + y +
          '<span class="ch-grid-year-count">' + groups[y].length + '</span>';
        hdr.appendChild(td);
        hdr.addEventListener('click', function() {
          state.collapsed[y] = !isCollapsed(y, gi);
          render();
        });
        frag.appendChild(hdr);
        if (!collapsed) groups[y].forEach(function(tr) { frag.appendChild(tr); });
      });
      label = rows.length + ' rows in ' + order.length + ' year group(s)';
    } else {
      var total = rows.length;
      var pages = Math.max(1, Math.ceil(total / state.pageSize));
      if (state.page > pages - 1) state.page = pages - 1;
      if (state.page < 0) state.page = 0;
      var start = state.page * state.pageSize;
      var end = Math.min(start + state.pageSize, total);
      rows.slice(start, end).forEach(function(tr) { frag.appendChild(tr); });
      label = total ? (start + 1) + '–' + end + ' of ' + total : '0 rows';
    }
    if (rows.length < allRows.length) {
      label += ' (filtered from ' + allRows.length + ')';
    }

    while (tbody.firstChild) tbody.removeChild(tbody.firstChild);
    if (!rows.length && allRows.length) {
      var empty = document.createElement('tr');
      empty.className = 'ch-grid-empty-row';
      var etd = document.createElement('td');
      etd.colSpan = headerCells.length;
      etd.textContent = 'No matching rows';
      empty.appendChild(etd);
      frag.appendChild(empty);
    }
    tbody.appendChild(frag);

    var count = root.querySelector('.ch-grid-count');
    if (count) count.textContent = label;
    renderPager(rows.length);
    renderSortIndicators();
  }

  function renderPager(total) {
    var pager = root.querySelector('.ch-grid-pager');
    if (!pager) return;
    if (state.grouped) { // grouping owns the layout; paging is suspended
      pager.innerHTML = '';
      return;
    }
    var pages = Math.max(1, Math.ceil(total / state.pageSize));
    var sizes = [10, 25, 50, 100];
    var opts = sizes.map(function(s) {
      return '<option value="' + s + '"' + (s === state.pageSize ? ' selected' : '') + '>' + s + '</option>';
    }).join('');
    pager.innerHTML =
      '<button type="button" data-act="prev"' + (state.page === 0 ? ' disabled' : '') + '>‹ Prev</button>' +
      '<span>Page ' + (state.page + 1) + ' of ' + pages + '</span>' +
      '<button type="button" data-act="next"' + (state.page >= pages - 1 ? ' disabled' : '') + '>Next ›</button>' +
      '<span>| Show <select class="ch-grid-psize">' + opts + '</select> per page</span>';
  }

  function renderSortIndicators() {
    headerCells.forEach(function(th, idx) {
      var ind = th.querySelector('.ch-grid-sort-ind');
      if (!ind) return;
      var text = '';
      for (var i = 0; i < state.sorts.length; i++) {
        if (state.sorts[i].idx === idx) {
          text = state.sorts[i].dir === 1 ? '▲' : '▼';
          if (state.sorts.length > 1) text += (i + 1); // priority no. for multi-sort
          break;
        }
      }
      ind.textContent = text;
    });
  }

  // --- header sorting: click cycles asc → desc → off; shift-click appends the
  // column as a secondary sort instead of replacing the existing spec.
  headerCells.forEach(function(th, idx) {
    if (!th.classList.contains('ch-grid-sortable')) return;
    th.addEventListener('click', function(ev) {
      var existing = null;
      for (var i = 0; i < state.sorts.length; i++) {
        if (state.sorts[i].idx === idx) { existing = state.sorts[i]; break; }
      }
      if (ev.shiftKey) {
        if (existing) {
          if (existing.dir === 1) existing.dir = -1;
          else state.sorts.splice(state.sorts.indexOf(existing), 1);
        } else {
          state.sorts.push({ idx: idx, dir: 1 });
        }
      } else {
        if (existing && state.sorts.length === 1) {
          if (existing.dir === 1) existing.dir = -1;
          else state.sorts = [];
        } else {
          state.sorts = [{ idx: idx, dir: 1 }];
        }
      }
      render();
    });
  });

  // --- per-column filters (input row under the header)
  Array.prototype.forEach.call(root.querySelectorAll('.ch-grid-colfilter'), function(input) {
    input.addEventListener('input', function() {
      state.colFilters[input.getAttribute('data-idx')] = input.value.toLowerCase();
      state.page = 0;
      render();
    });
    // Filter inputs live inside sortable header area siblings; stop clicks from
    // reaching any ancestor handlers.
    input.addEventListener('click', function(ev) { ev.stopPropagation(); });
  });

  // --- quick search across all columns
  var search = root.querySelector('.ch-grid-search');
  if (search) {
    search.addEventListener('input', function() {
      state.quick = search.value.toLowerCase();
      state.page = 0;
      render();
    });
  }

  // --- year grouping toggle
  var groupBtn = root.querySelector('.ch-grid-groupbtn');
  if (groupBtn && groupCol >= 0) {
    groupBtn.addEventListener('click', function() {
      state.grouped = !state.grouped;
      state.collapsed = {};
      groupBtn.classList.toggle('ch-grid-active', state.grouped);
      // Grouping reads best with the date column ordering the groups; only
      // impose that when the user hasn't chosen a sort.
      if (state.grouped && !state.sorts.length) state.sorts = [{ idx: groupCol, dir: -1 }];
      render();
    });
  }

  // --- client pager (delegated: the pager's innerHTML is rebuilt every render)
  var pager = root.querySelector('.ch-grid-pager');
  if (pager) {
    pager.addEventListener('click', function(e) {
      var act = e.target.getAttribute && e.target.getAttribute('data-act');
      if (act === 'prev') { state.page--; render(); }
      else if (act === 'next') { state.page++; render(); }
    });
    pager.addEventListener('change', function(e) {
      if (e.target.classList.contains('ch-grid-psize')) {
        state.pageSize = parseInt(e.target.value, 10) || 25;
        state.page = 0;
        render();
      }
    });
  }

  // --- row-level actions, delegated on tbody so they survive row re-attachment
  tbody.addEventListener('click', function(e) {
    var del = e.target.closest ? e.target.closest('a.ch-grid-del') : null;
    if (del) {
      e.preventDefault();
      var url = del.getAttribute('data-url');
      // Deletes go over POST with the grid's CSRF token — the server routes
      // reject GET, so a bare navigation (prefetch, forged link) cannot delete.
      var doDelete = function() {
        var form = document.createElement('form');
        form.method = 'POST';
        form.action = url;
        var tok = document.createElement('input');
        tok.type = 'hidden';
        tok.name = 'csrf';
        tok.value = root.getAttribute('data-csrf') || '';
        form.appendChild(tok);
        document.body.appendChild(form);
        form.submit();
      };
      if (typeof swal === 'function') { // SweetAlert2 when the page ships it
        swal({
          title: 'Are you sure?', text: "You won't be able to undo!", type: 'warning',
          showCancelButton: true, confirmButtonColor: '#3085d6', cancelButtonColor: '#d33',
          confirmButtonText: 'Yes, delete it!'
        }).then(function(result) { if (result.value) doDelete(); });
      } else if (window.confirm('Delete this item? This cannot be undone.')) {
        doDelete();
      }
      return;
    }
    // Popup columns: show the full cell content (HTML cells are clamped in-row)
    var td = e.target.closest ? e.target.closest('td.ch-grid-popup') : null;
    if (td && !(e.target.closest && e.target.closest('a'))) {
      var content = td.querySelector('.ch-grid-html');
      var title = headerCells[td.cellIndex] ? headerCells[td.cellIndex].textContent.trim() : '';
      if (typeof swal === 'function') {
        swal({ title: title, html: content ? content.innerHTML : td.innerHTML });
      } else {
        window.alert(td.textContent.trim());
      }
    }
  });

  root.classList.add('ch-grid-ready'); // reveal .ch-grid-jsonly controls
  render();
}

function chGridBoot() {
  var grids = document.querySelectorAll('.ch-grid');
  for (var i = 0; i < grids.length; i++) chGridInit(grids[i]);
}
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', chGridBoot);
} else {
  chGridBoot();
}
})();
`
