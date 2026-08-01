package template

// ChromeCSS and ChromeJS are framework-level fixes for page chrome that every
// site shares: the flash banner and the nav submenu. Like ResponsiveCSS they
// are inlined after the app.css link so they win the cascade with no site
// stylus rebuild.
//
// Flash: site CSS positions #flash absolutely at a desktop-tuned offset
// (top: 107px) that lands in arbitrary places once columns restack on a
// phone, and absolute positioning doesn't travel with #mid (the real scroll
// container). Pinning it fixed at the viewport top works at every width and
// keeps confirmations visible no matter where the user has scrolled. The
// !importants are required to beat the site's id-selector rules.
//
// Nav submenus: site menus open submenus via CSS :hover, which cannot open on
// touch screens; the parent item is href="#" so on a phone the admin submenu
// was effectively unreachable. ChromeJS adds a click-toggle that forces the
// submenu open via the .submenu-open class below — additive only, desktop
// hover behavior is untouched (first tap opens, tap elsewhere closes).
const ChromeCSS = `
#flash {
  position: fixed !important;
  top: 0.75rem !important;
  left: 50% !important;
  transform: translateX(-50%);
  width: max-content;
  max-width: min(92vw, 42rem);
  z-index: 1200 !important;
}
#flash .flash-info, #flash .flash-warn, #flash .flash-error {
  display: flex; align-items: center; gap: 0.7rem;
  padding: 0.55rem 0.9rem !important;
  border-radius: 6px !important;
  box-shadow: 0 3px 12px rgba(0,0,0,0.22);
  margin-bottom: 0.4rem;
}
#flash button.flash-close {
  background: transparent; border: none; cursor: pointer;
  font: inherit; color: inherit; opacity: 0.65; padding: 0 0.15rem;
  margin-left: auto; line-height: 1;
}
#flash button.flash-close:hover { opacity: 1; }

/* JS-toggled submenu reveal for touch/keyboard (see ChromeJS) */
#header nav li.submenu-open > ul { display: block !important; }
`

const ChromeJS = `
(function () {
	'use strict';
	function init() {
		var nav = document.querySelector('#header nav');
		if (!nav) { return; }
		var parents = nav.querySelectorAll('li > a[href="#"]');
		Array.prototype.forEach.call(parents, function (a) {
			var li = a.parentNode;
			if (!li.querySelector('ul')) { return; }
			a.setAttribute('aria-haspopup', 'true');
			a.setAttribute('aria-expanded', 'false');
			a.addEventListener('click', function (ev) {
				ev.preventDefault();
				var open = li.classList.toggle('submenu-open');
				a.setAttribute('aria-expanded', open ? 'true' : 'false');
			});
		});
		// A tap/click outside the nav closes any open submenu
		document.addEventListener('click', function (ev) {
			if (nav.contains(ev.target)) { return; }
			Array.prototype.forEach.call(nav.querySelectorAll('li.submenu-open'), function (li) {
				li.classList.remove('submenu-open');
				var a = li.querySelector('a[href="#"]');
				if (a) { a.setAttribute('aria-expanded', 'false'); }
			});
		});
	}
	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', init);
	} else {
		init();
	}
})();
`
