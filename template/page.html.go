package template

import (
	"bytes"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/grid"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/menu"
	"github.com/rohanthewiz/church/view"
	"github.com/rohanthewiz/element"
)

func Page(buffer *bytes.Buffer, page *page.Page, flsh *flash.Flash, params map[string]map[string]string, loggedIn bool) {
	layout := page.GetLayout()

	buffer.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="/assets/css/app.css">
    <link rel="stylesheet" href="/assets/css/bootstrap_scoped.css">
    <link rel="stylesheet" href="/assets/css/fullcalendar.min.css">
    <link rel="stylesheet" href="/assets/css/slick.css">
    <link rel="stylesheet" href="/assets/css/slick-theme.css">
    <link rel="stylesheet" href="/assets/css/summernote.min.css">
	<link href="https://fonts.googleapis.com/css?family=Cormorant+SC:700|Courgette|Katibeh" rel="stylesheet">
	<script type="text/javascript" src="https://cdnjs.cloudflare.com/ajax/libs/jquery/2.1.4/jquery.min.js"></script>
	<script type="text/javascript" src="https://js.stripe.com/v3/"></script>
	<script type="text/javascript" src="/assets/js/moment.min.js"></script>
	<script type="text/javascript" src="/assets/js/fullcalendar.min.js"></script>
	<script type="text/javascript" src="/assets/js/slick-1.8.1.min.js"></script>
	<script type="text/javascript" src="/assets/js/sweetalert2-7.12.15.all.min.js"></script>`)

	b := element.NewBuilder()

	// Home-grown grid (replaces the AG Grid bundle + its cell-renderer shims).
	// Inlined so list pages need no extra asset fetch; the JS enhances every
	// .ch-grid rendered by the list modules.
	b.Style().T(grid.CSS)
	b.Script("type", "text/javascript").T(grid.JS)

	// Small-screen overrides for the three-column layout (see responsive_css.go).
	// Emitted after the app.css link so it wins the cascade on every site.
	b.Style().T(ResponsiveCSS)

	b.T(`</head><body class="theme-` + config.Options.Theme + `">`)

	// Banner
	b.T(view.PgFrame.GetBanner())

	// Menu
	b.DivClass("theme-"+config.Options.Theme, "id", "header").T(
		menu.RenderNav("main-menu", loggedIn))

	// Flash
	b.T(flsh.Render())

	b.DivClass("theme-"+config.Options.Theme, "id", "mid").R(
		// Left
		b.DivClass(layout, "id", "left-side").R(
			b.T(page.Render("left", params, loggedIn)),
		),
		// Center
		b.DivClass(layout, "id", "main").R(
			b.Wrap(func() {
				if loggedIn && page.IsDynamic() {
					b.DivClass("page-edit").R(
						b.AClass("edit-link", "href", "/admin/pages/edit/"+page.PresenterId).R(
							b.ImgClass("edit-icon", "src", "/assets/images/edit_page.svg", "title", "Edit Page").R(),
						),
					)
				}
			}),
			b.T(page.Render("center", params, loggedIn)),
		),
		// Right
		b.DivClass(layout, "id", "right-side").R(
			b.T(page.Render("right", params, loggedIn)),
		),
	)

	b.DivClass("theme-"+config.Options.Theme, "id", "footer").R(
		b.T(menu.RenderNav("footer-menu", loggedIn)),
		b.T(view.PgFrame.GetCopyright()),
	)

	if page.IsAdmin {
		b.T(`<script src="/assets/js/jquery.serialize-object.min.js"></script>
	    <script src="/assets/js/bootstrap.js"></script>
	    <script src="/assets/js/summernote.min.js"></script>`)
	} else {
		// Blue Letter Bible ScriptTagger: scans the rendered page for verse
		// references (e.g. "John 3:16", "Rom 1:16-18"), links them, and shows a
		// hover tooltip with the verse text plus deep links into BLB's study
		// tools. Public pages only — on admin pages it would rewrite references
		// inside the Summernote editor's DOM and corrupt content on save.
		// The script defines the BLB global and applies settings at DOM-ready,
		// so config assignments go after the include (before it, BLB is
		// undefined and the assignments would throw). Loaded at the end of
		// <body> so page content exists when it scans.
		b.T(`<script src="https://www.blueletterbible.org/assets/scripts/blbToolTip/BLB_ScriptTagger-min.js" type="text/javascript"></script>
	    <script type="text/javascript">
	    BLB.Tagger.Translation = 'NKJV';
	    BLB.Tagger.HyperLinks = 'all';
	    BLB.Tagger.TargetNewWindow = true;
	    </script>`)
	}

	b.T(`</body></html>`)

	buffer.WriteString(b.String())
}
