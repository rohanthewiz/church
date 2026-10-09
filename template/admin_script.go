package template

// AdminJS is the framework-wide script for admin screens, inlined into <head>
// for admin pages only (see Page()), next to AdminCSS and for the same reason:
// it ships with the framework, so no site needs an asset rebuild.
// (File is admin_script.go, not admin_js.go: a _js.go suffix is an implicit
// GOOS=js build constraint and the file would be silently excluded.)
//
// Today it holds one behavior: drag-and-drop reordering for admin repeaters
// (page-form module cards, menu-form items). Those lists already reorder with
// ↑/↓ buttons; the drag handle is an extra, faster path, so the arrows stay as
// the keyboard and screen-reader route and the handle is aria-hidden.
//
// Declarative contract (no per-form JS call is needed):
//
//	<div data-af-sort>                 the list; its direct children are the items
//	  <div class="...">                one item (any markup)
//	    ... <span class="af-drag-handle"></span>   grip anywhere inside the item
//	  </div>
//	</div>
//
// A single delegated pointerdown listener on document handles every list,
// including items the forms add after load. The DOM order is the source of
// truth: both forms serialize rows in document order on submit, so moving the
// node is all a drop has to do.
//
// Design choices:
//   - Pointer Events rather than the HTML5 drag-and-drop API. HTML5 DnD needs
//     draggable="true" on the item (which fights the text inputs inside it),
//     gives no control over the drag image, and is unreliable on touch. Pointer
//     Events cover mouse, pen and touch with one code path.
//   - Live reordering instead of a ghost + drop placeholder: the dragged item
//     itself moves through the list as the pointer crosses its neighbours'
//     midpoints, so what you see is exactly what will be saved.
//   - Move/up listeners go on window, not on the handle via setPointerCapture.
//     Re-inserting the dragged node (insertBefore) counts as removing it from
//     the document, which silently drops pointer capture in some browsers and
//     would end the drag on the first swap.
//   - Midpoints use offsetTop/offsetHeight (layout positions) rather than
//     getBoundingClientRect, because neighbours are mid-animation (FLIP
//     transforms) during a drag; transformed rects would make the target
//     flicker back and forth. [data-af-sort] is position:relative in AdminCSS
//     so the items' offsetParent is the list itself.
//
// Target selection, for pointer y (client coords):
//
//	┌──────────┐
//	│ item A   │ ─ mid A ─   y above mid A  → dragged goes before A
//	├──────────┤
//	│ item B   │ ─ mid B ─   else y above mid B → before B
//	├──────────┤
//	│ (end)    │             else → appended last
//
// The dragged item is skipped when scanning, which gives natural hysteresis:
// after a swap the pointer sits inside the dragged item again, so a tall item
// (an expanded module card) does not oscillate against a short neighbour.
const AdminJS = `
(function () {
	'use strict';

	var EDGE = 60;      // px from the viewport edge where auto-scroll kicks in
	var MAX_STEP = 18;  // px scrolled per animation frame at the very edge
	var SLOP = 4;       // px the pointer must travel before auto-scroll may start
	var reduceMotion = window.matchMedia &&
		window.matchMedia('(prefers-reduced-motion: reduce)').matches;

	var drag = null;    // { list, item, y, startY, moved, raf } while dragging

	// Direct children of the list other than the dragged item, in DOM order.
	function others(list, item) {
		var out = [];
		for (var c = list.firstElementChild; c; c = c.nextElementSibling) {
			if (c !== item) { out.push(c); }
		}
		return out;
	}

	// Move item before ref (null = to the end), animating the displaced
	// neighbours from their old position to the new one (FLIP: record First
	// positions, apply the Last layout, Invert with a transform, Play by
	// transitioning the transform back to none).
	function moveTo(list, item, ref) {
		var kids = others(list, item);
		var tops = reduceMotion ? null : kids.map(function (k) { return k.offsetTop; });
		list.insertBefore(item, ref);
		if (!tops) { return; }
		kids.forEach(function (k, i) {
			var d = tops[i] - k.offsetTop;
			if (!d) { return; }
			k.style.transition = 'none';
			k.style.transform = 'translateY(' + d + 'px)';
			void k.offsetHeight; // flush so the inverted position is painted first
			k.style.transition = 'transform 150ms ease';
			k.style.transform = '';
		});
	}

	function reposition() {
		var list = drag.list, item = drag.item;
		// Convert the pointer into the list's padding-box coordinates, the same
		// space offsetTop is measured in.
		var y = drag.y - list.getBoundingClientRect().top - list.clientTop;
		var kids = others(list, item);
		var ref = null;
		for (var i = 0; i < kids.length; i++) {
			if (y < kids[i].offsetTop + kids[i].offsetHeight / 2) { ref = kids[i]; break; }
		}
		// Skip no-op moves so the DOM is touched only when the order changes
		if (ref ? item.nextElementSibling === ref : list.lastElementChild === item) { return; }
		moveTo(list, item, ref);
	}

	// Keep scrolling while the pointer rests near a viewport edge, so a card
	// can be dragged past the visible area. Pointer events only fire on
	// movement, hence a rAF loop for the whole drag.
	function autoScroll() {
		if (!drag) { return; }
		var step = 0;
		// Grabbing a row that already sits near the edge must not scroll the
		// page out from under the pointer before the user has moved at all.
		if (drag.moved && drag.y < EDGE) {
			step = -Math.ceil(MAX_STEP * (EDGE - drag.y) / EDGE);
		} else if (drag.moved && drag.y > window.innerHeight - EDGE) {
			step = Math.ceil(MAX_STEP * (drag.y - (window.innerHeight - EDGE)) / EDGE);
		}
		if (step) {
			var before = window.pageYOffset;
			window.scrollBy(0, step);
			// The list moved under a stationary pointer; re-evaluate the slot
			if (window.pageYOffset !== before) { reposition(); }
		}
		drag.raf = window.requestAnimationFrame(autoScroll);
	}

	function onMove(e) {
		if (!drag) { return; }
		e.preventDefault();
		drag.y = e.clientY;
		if (Math.abs(drag.y - drag.startY) > SLOP) { drag.moved = true; }
		reposition();
	}

	function onEnd() {
		if (!drag) { return; }
		window.cancelAnimationFrame(drag.raf);
		window.removeEventListener('pointermove', onMove);
		window.removeEventListener('pointerup', onEnd);
		window.removeEventListener('pointercancel', onEnd);
		drag.item.classList.remove('af-dragging');
		document.body.classList.remove('af-drag-active');
		// Drop leftover FLIP styles so they cannot interfere with later layout
		others(drag.list, drag.item).forEach(function (k) {
			k.style.transition = ''; k.style.transform = '';
		});
		drag = null;
	}

	document.addEventListener('pointerdown', function (e) {
		if (drag || e.button !== 0 || !e.target.closest) { return; }
		var handle = e.target.closest('.af-drag-handle');
		if (!handle) { return; }
		var list = handle.closest('[data-af-sort]');
		if (!list) { return; }
		// The item is the list's direct child that contains the handle
		var item = handle;
		while (item && item.parentElement !== list) { item = item.parentElement; }
		if (!item) { return; }

		// Suppresses text selection and the focus change the press would cause
		e.preventDefault();
		drag = { list: list, item: item, y: e.clientY, startY: e.clientY,
			moved: false, raf: 0 };
		item.classList.add('af-dragging');
		document.body.classList.add('af-drag-active');
		window.addEventListener('pointermove', onMove);
		window.addEventListener('pointerup', onEnd);
		window.addEventListener('pointercancel', onEnd);
		drag.raf = window.requestAnimationFrame(autoScroll);
	});
})();
`
