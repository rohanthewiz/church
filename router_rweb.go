package church

import (
	"net/http"
	"os"

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
	"github.com/rohanthewiz/church/resource/article"
	"github.com/rohanthewiz/church/resource/calendar"
	"github.com/rohanthewiz/church/resource/event"
	"github.com/rohanthewiz/church/resource/feed"
	"github.com/rohanthewiz/church/resource/sermon"
	"github.com/rohanthewiz/church/sermon_controller"
	"github.com/rohanthewiz/church/user_controller"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

//go:generate go run pack/packer.go

func ServeRWeb() {
	admin.AuthBootstrap()
	admin.Bootstrap() // Seed DB with essential resources (menus, home page, etc.)
	page.RegisterModules()

	idrive.InitClient()

	// Background LRU eviction of locally-cached sermons downloaded from IDrive e2.
	// Scans hourly and deletes copies idle > 4h, but only after confirming the
	// object still exists on IDrive e2. Opt-in: only runs when idrive.auto_cleanup
	// is true. The admin Sermon Cleanup tool works regardless of this flag.
	if config.Options.IDrive.Enabled && config.Options.IDrive.AutoCleanup {
		idrive.StartCacheCleanup()
	}

	// TLS (see tls_rweb.go): autocert (in-process Let's Encrypt) or hot-reloaded
	// cert files. Also starts the HTTP challenge/redirect listener when enabled.
	// A cert misconfiguration is unrecoverable, so fail startup loudly rather
	// than silently serving plain HTTP with use_tls set.
	tlsCfg, err := buildTLSCfg()
	if err != nil {
		logger.LogErr(err, "TLS configuration failed - exiting")
		os.Exit(1)
	}

	// Create RWeb server
	s := rweb.NewServer(rweb.ServerOptions{
		Address: ":" + config.Options.Server.Port,
		Verbose: true, // config.AppEnv == config.Environments.Development,
		TLS:     tlsCfg,
	})

	// Static files
	s.StaticFiles("/assets/", "dist", 1)
	// Serve cached sermon media from the same directory the IDrive cache and
	// cleanup service use, so all three always agree on where files live.
	// Fall back to the historical "sermons" dir for configs predating the key.
	sermonsDir := config.Options.IDrive.LocalSermonsDir
	if sermonsDir == "" {
		sermonsDir = "sermons"
	}
	s.StaticFiles("/media/", sermonsDir, 1)

	// Home page — wrapped in a group with the auth middleware so session/login
	// state is available for rendering admin menus when the user is logged in.
	home := s.Group("", authctlr.UseCustomContextRWeb)
	home.Get("/", page_controller.HomePageRWeb)

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

	// JSON API v1 — consumed by the church_mobile app (Phase 1: read-only,
	// published content only; see ai_docs/plans/2026-0707-mobile-app-flutter-api-plan.md).
	// Deliberately outside the session middleware: these endpoints are public
	// reads; auth arrives in Phase 2 as a Bearer-token guard on a sub-group.
	api := s.Group("/api/v1")
	api.Get("/sermons", sermon.APISermonsRWeb)
	api.Get("/sermons/:id", sermon.APISermonRWeb)
	api.Get("/articles", article.APIArticlesRWeb)
	api.Get("/articles/:id", article.APIArticleRWeb)
	api.Get("/events", event.APIEventsRWeb)
	api.Get("/events/:id", event.APIEventRWeb)
	api.Get("/feed", feed.APIFeedRWeb)

	// FullCalendar-shaped events JSON for the website's calendar widget
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
	// PaymentIntents flow: the form JS posts here for a client secret, confirms the
	// payment with Stripe directly (SCA/3DS, wallets), then Stripe redirects to
	// /receipt below, which records the completed intent locally.
	// Replaces the legacy token+Charges post:
	// pay.Post("/create", payment_controller.UpsertPaymentRWeb) // create
	pay.Post("/create-intent", payment_controller.CreatePaymentIntentRWeb)
	pay.Get("/receipt", payment_controller.PaymentReceiptRWeb)
	// Stripe server-to-server events (payment_intent.succeeded). Deliberately outside
	// the session middleware: the caller is Stripe, authenticated by signature, not cookie.
	s.Post("/webhooks/stripe", payment_controller.StripeWebhookRWeb)

	// Sermons
	ser := s.Group("/sermons", authctlr.UseCustomContextRWeb)
	ser.Get("", sermon_controller.ListSermonsRWeb)
	ser.Get("/:id", sermon_controller.ShowSermonRWeb) // "/:id" -> conflicts with "/:year/:filename" so we will use sermon-audio instead

	s.Get("/sermon-audio/:year/:filename", func(ctx rweb.Context) error {
		year := ctx.Request().Param("year")
		filename := ctx.Request().Param("filename")
		logger.Debug("Sermon audio requested", "year", year, "filename", filename)

		byts, err := idrive.GetSermon(year, filename)
		if err != nil {
			logger.Err(err, "error getting sermon", "year", year, "sermon", filename)
			// 404, not 501: a missing/unfetchable file is "not found" to the
			// client. (501 told clients the server lacks the feature, which
			// misleads mobile error handling and can be cached by proxies.)
			return ctx.Status(http.StatusNotFound).WriteJSON(map[string]string{
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
	// Local sermon-cache cleanup tool (lists copies safe to delete, batch-deletes them)
	ad.Get("/sermons/cleanup", sermon_controller.AdminSermonCleanupRWeb)
	ad.Post("/sermons/cleanup", sermon_controller.AdminSermonCleanupRunRWeb)

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
	ad.Post("/pages", page_controller.UpsertPageRWeb)       // create
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
	if err := s.Run(); err != nil {
		logger.LogErr(err, "failed to start server")
	}
}
