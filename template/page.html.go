package template

import (
	"bytes"

	"github.com/rohanthewiz/church/agrid"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/flash"
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
	<script type="text/javascript" src="/assets/js/ag-grid.min.js"></script>
	<script type="text/javascript" src="/assets/js/sweetalert2-7.12.15.all.min.js"></script>`)

	b := element.NewBuilder()
	e := b.E

	e("script", "type", "text/javascript").R(
		b.WS(agrid.LinkCellRenderer),
		b.WS(agrid.ConfirmlinkCellRenderer),
		b.WS(agrid.ConfirmDelete),
	)

	b.WS(`</head><body class="theme-` + config.Options.Theme + `">`)

	// Banner
	b.WS(view.PgFrame.GetBanner())

	// Menu
	e("div", "id", "header", "class", "theme-"+config.Options.Theme).R(
		b.WS(menu.RenderNav("main-menu", loggedIn)),
	)

	// Flash
	b.WS(flsh.Render())

	e("div", "id", "mid", "class", "theme-"+config.Options.Theme).R(
		// Left
		e("div", "id", "left-side", "class", layout).R(
			b.WS(page.Render("left", params, loggedIn)),
		),
		// Center
		e("div", "id", "main", "class", layout).R(
			func() (str string) { // Page edit link
				if loggedIn && page.IsDynamic() {
					e("div", "class", "page-edit").R(
						e("a", "class", "edit-link", "href", "/admin/pages/edit/"+page.PresenterId).R(
							e("img", "class", "edit-icon", "src", "/assets/images/edit_page.svg", "title", "Edit Page").R(),
						),
					)
				}
				return
			}(),
			b.WS(page.Render("center", params, loggedIn)),
		),
		// Right
		e("div", "id", "right-side", "class", layout).R(
			b.WS(page.Render("right", params, loggedIn)),
		),
	)

	e("div", "id", "footer", "class", "theme-"+config.Options.Theme).R(
		b.WS(menu.RenderNav("footer-menu", loggedIn)),
		b.WS(view.PgFrame.GetCopyright()),
	)

	if page.IsAdmin {
		b.WS(`<script src="/assets/js/jquery.serialize-object.min.js"></script>
	    <script src="/assets/js/bootstrap.js"></script>
	    <script src="/assets/js/summernote.min.js"></script>`)
	}
	// b.WS(`<script type="text/javascript">
	//	$(document).ready(function() {
	//		$('#banner').addClass('theme-cobalt');
	//	});
	// </script>`)

	b.WS(`</body></html>`)

	buffer.WriteString(b.String())
}

// assets/js/jquery-3.2.1.min.js
