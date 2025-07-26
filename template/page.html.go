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

	b.Script("type", "text/javascript").R(
		b.T(agrid.LinkCellRenderer),
		b.T(agrid.ConfirmlinkCellRenderer),
		b.T(agrid.ConfirmDelete),
	)

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
	}

	b.T(`</body></html>`)

	buffer.WriteString(b.String())
}
