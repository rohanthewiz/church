package church

import (
	"net/http"

	"github.com/rohanthewiz/church/admin"
	"github.com/rohanthewiz/church/admin_controller"
	"github.com/rohanthewiz/church/article_controller"
	authctlr "github.com/rohanthewiz/church/auth_controller"
	"github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/core/idrive"
	"github.com/rohanthewiz/church/event_controller"
	"github.com/rohanthewiz/church/menu_controller"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/page_controller"
	"github.com/rohanthewiz/church/payment_controller"
	"github.com/rohanthewiz/church/resource/calendar"
	"github.com/rohanthewiz/church/resource/sermon"
	"github.com/rohanthewiz/church/sermon_controller"
	"github.com/rohanthewiz/church/user_controller"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

// todo !! setup cert renew on a chron job

//go:generate go run pack/packer.go

func ServeRWeb() {
	admin.AuthBootstrap()
	page.RegisterModules()

	idrive.InitClient()

	// Create RWeb server
	s := rweb.NewServer(rweb.ServerOptions{
		Address: ":" + config.Options.Server.Port,
		Verbose: config.Options.Debug,
		TLS: rweb.TLSCfg{
			UseTLS:   config.Options.Server.UseTLS && config.AppEnv != "development",
			KeyFile:  config.Options.Server.KeyFile,
			CertFile: config.Options.Server.CertFile,
		},
	})

	// Static files
	s.StaticFiles("/assets/", "dist", 1)
	s.StaticFiles("/media/", "sermons", 1) // TODO - path_from_proj_root(config.Options.IDrive.LocalSermonsDir)

	// Home page
	s.Get("/", page_controller.HomePageRWeb)

	// Debug routes
	s.Get("/debug/set", func(ctx rweb.Context) error {
		element.DebugSet()
		return ctx.WriteHTML("<h3>Debug mode set.</h3> <a href='/'>Home</a>")
	})

	s.Get("/debug/show", func(ctx rweb.Context) error {
		return ctx.WriteHTML(element.DebugShow())
	})

	s.Get("/debug/clear", func(ctx rweb.Context) error {
		element.DebugClear()
		return ctx.WriteHTML("<h3>Debug mode is off.</h3> <a href='/'>Home</a>")
	})

	s.Get("/debug/clear-issues", func(ctx rweb.Context) error {
		element.DebugClearIssues()
		return ctx.WriteHTML("<h3>Issues cleared (debug mode still active).</h3> <a href='/'>Home</a> | <a href='/debug/show'>View Debug</a>")
	})

	// Authentication routes
	s.Get("/login", authctlr.LoginHandlerRWeb)
	s.Get("/logout", authctlr.LogoutHandlerRWeb)
	s.Post("/auth", authctlr.AuthHandlerRWeb) // Attempt login

	// Super admin setup
	s.Get("/super", admin_controller.SetupSuperAdminRWeb) // (API) Establish first SuperAdmin

	// API routes
	s.Get("/api/v1/sermons", sermon.APISermonsRWeb)
	s.Get("/calendar", calendar.GetFullCalendarEventsRWeb)

	// Non-admin dynamic pages (the majority of the pages) are handled here
	pgs := s.Group("/pages", authctlr.UseCustomContextRWeb)
	pgs.Get("/:slug", page_controller.PageHandlerRWeb)

	// Articles
	art := s.Group("/articles", authctlr.UseCustomContextRWeb)
	art.Get("", article_controller.ListArticlesRWeb)
	art.Get("/:id", article_controller.ShowArticleRWeb)

	// Events
	evt := s.Group("/events", authctlr.UseCustomContextRWeb)
	evt.Get("", event_controller.ListEventsRWeb)
	evt.Get("/:id", event_controller.ShowEventRWeb)

	// Payments
	pay := s.Group("/payments", authctlr.UseCustomContextRWeb)
	pay.Get("/new", payment_controller.NewPaymentRWeb)
	pay.Post("/create", payment_controller.UpsertPaymentRWeb) // create
	pay.Get("/receipt", payment_controller.PaymentReceiptRWeb)

	// Sermons
	ser := s.Group("/sermons", authctlr.UseCustomContextRWeb)
	ser.Get("", sermon_controller.ListSermonsRWeb)
	ser.Get("/:id", sermon_controller.ShowSermonRWeb)

	// Sermon media files
	ser.Get("/:year/:filename", func(ctx rweb.Context) error {
		year := ctx.Request().PathParam("year")
		filename := ctx.Request().PathParam("filename")

		byts, err := idrive.GetSermon(year, filename)
		if err != nil {
			logger.Err(err, "error getting sermon", "year", year, "sermon", filename)
			return ctx.Status(http.StatusNotImplemented).WriteJSON(map[string]string{
				"error_message":     "Sorry, we couldn't find the sermon you requested.",
				"technical_details": err.Error(),
			})
		}

		return basectlr.SendAudioFileRWeb(ctx, filename, byts)
	})

	// Admin group uses authentication middleware
	ad := s.Group(config.AdminPrefix, authctlr.UseCustomContextRWeb, authctlr.AdminGuardRWeb)

	ad.Get("/home", admin_controller.AdminHandlerRWeb)
	ad.Get("/logout", authctlr.LogoutHandlerRWeb)

	// Admin Users
	ad.Get("/users", user_controller.ListUsersRWeb)
	ad.Get("/users/new", user_controller.NewUserRWeb)
	ad.Post("/users", user_controller.UpsertUserRWeb) // create
	ad.Get("/users/edit/:id", user_controller.EditUserRWeb)
	ad.Post("/users/update/:id", user_controller.UpsertUserRWeb) // update
	ad.Get("/users/delete/:id", user_controller.DeleteUserRWeb)

	// Admin Articles
	ad.Get("/articles", article_controller.AdminListArticlesRWeb)
	ad.Get("/articles/new", article_controller.NewArticleRWeb)
	ad.Post("/articles", article_controller.UpsertArticleRWeb) // create
	ad.Get("/articles/edit/:id", article_controller.EditArticleRWeb)
	ad.Post("/articles/update/:id", article_controller.UpsertArticleRWeb) // update
	ad.Get("/articles/delete/:id", article_controller.DeleteArticleRWeb)

	// Admin Sermons
	ad.Get("/sermons", sermon_controller.AdminListSermonsRWeb)
	ad.Get("/sermons/new", sermon_controller.NewSermonRWeb)
	ad.Get("/sermons/import", sermon_controller.ImportRWeb)
	ad.Post("/sermons", sermon_controller.UpsertSermonRWeb) // create
	ad.Get("/sermons/edit/:id", sermon_controller.EditSermonRWeb)
	ad.Post("/sermons/update/:id", sermon_controller.UpsertSermonRWeb) // update
	ad.Get("/sermons/delete/:id", sermon_controller.DeleteSermonRWeb)

	// Admin Events
	ad.Get("/events", event_controller.AdminListEventsRWeb)
	ad.Get("/events/new", event_controller.NewEventRWeb)
	ad.Post("/events", event_controller.UpsertEventRWeb) // create
	ad.Get("/events/edit/:id", event_controller.EditEventRWeb)
	ad.Post("/events/update/:id", event_controller.UpsertEventRWeb) // update
	ad.Get("/events/delete/:id", event_controller.DeleteEventRWeb)

	// Admin Pages
	ad.Get("/pages", page_controller.AdminListPagesRWeb)
	ad.Get("/pages/new", page_controller.NewPageRWeb)
	ad.Post("/pages", page_controller.UpsertPageRWeb) // create
	ad.Get("/pages/:id", page_controller.AdminShowPageRWeb) // preview
	ad.Get("/pages/edit/:id", page_controller.EditPageRWeb)
	ad.Post("/pages/update/:id", page_controller.UpsertPageRWeb) // update
	ad.Get("/pages/delete/:id", page_controller.DeletePageRWeb)

	// Admin Menus
	ad.Get("/menus", menu_controller.AdminListMenusRWeb)
	ad.Get("/menus/new", menu_controller.NewMenuRWeb)
	ad.Post("/menus", menu_controller.UpsertMenuRWeb) // create
	ad.Get("/menus/edit/:id", menu_controller.EditMenuRWeb)
	ad.Post("/menus/update/:id", menu_controller.UpsertMenuRWeb) // update
	ad.Get("/menus/delete/:id", menu_controller.DeleteMenuRWeb)

	// Start the server
	logger.Fatal(s.Run())
}