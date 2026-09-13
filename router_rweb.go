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
	"github.com/rohanthewiz/church/resource/apitoken"
	"github.com/rohanthewiz/church/resource/payment"
	"github.com/rohanthewiz/church/resource/apiv1"
	"github.com/rohanthewiz/church/resource/article"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/resource/calendar"
	"github.com/rohanthewiz/church/resource/chat"
	"github.com/rohanthewiz/church/resource/dbbackup"
	"github.com/rohanthewiz/church/resource/event"
	"github.com/rohanthewiz/church/resource/feed"
	"github.com/rohanthewiz/church/resource/prayerwall"
	"github.com/rohanthewiz/church/resource/sermon"
	"github.com/rohanthewiz/church/role_controller"
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

	// Background retention for live chat: messages older than a day are
	// deleted unless an editor marked them keep (see resource/chat).
	chat.StartRetentionSweep()

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

	// Liveness/readiness target for container orchestrators. Deliberately the
	// cheapest possible handler: no session middleware, no DB round trip, no
	// page render. Probes fire every few seconds for the life of the pod, and
	// pointing them at "/" (as the first cut of the k8s manifests did) meant a
	// full home-page build plus its queries on every tick — and, far worse for
	// liveness, it made "the database is briefly slow" indistinguishable from
	// "the process is wedged", which is how a probe turns a hiccup into a
	// restart loop.
	//
	// Reporting DB health here is intentionally omitted rather than forgotten:
	// each site runs a single replica over a ReadWriteOnce volume, so failing
	// readiness cannot shift traffic anywhere — it only converts a degraded
	// site into a hard 503 from the ingress. Same priority ordering the
	// replication design settled on: serving outranks reporting.
	s.Get("/healthz", func(ctx rweb.Context) error {
		return ctx.WriteText("ok")
	})

	// Home page — wrapped in a group with the auth middleware so session/login
	// state is available for rendering admin menus when the user is logged in.
	home := s.Group("", authctlr.UseCustomContextRWeb)
	home.Get("/", page_controller.HomePageRWeb)

	// Debug routes — SuperAdmin-only. These toggle process-wide element debug
	// state and dump internal render diagnostics, so they must not be reachable
	// by anonymous visitors (any GET could flip debug mode on a production
	// site). Nor by every admin: the state is shared by all visitors, which
	// makes this an operator tool rather than content work (see RequireSuper).
	RegisterDebugRoutes(s)

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
	// Boot-time site config for the app (church name, Stripe publishable key,
	// feature flags). Public: fetched before any login.
	api.Get("/app-config", apiv1.APIAppConfigRWeb)
	api.Get("/sermons", sermon.APISermonsRWeb)
	api.Get("/sermons/:id", sermon.APISermonRWeb)
	api.Get("/articles", article.APIArticlesRWeb)
	api.Get("/articles/:id", article.APIArticleRWeb)
	api.Get("/events", event.APIEventsRWeb)
	api.Get("/events/:id", event.APIEventRWeb)
	api.Get("/feed", feed.APIFeedRWeb)

	// Phase 2 mobile auth: DB-backed bearer tokens (survive deploys, last
	// weeks — see resource/apitoken). Login is the one public auth route;
	// everything else wraps in the Bearer guard, which is also how any future
	// personalized endpoint (giving history, chat) gets protected. APIGuard is
	// a per-handler decorator, not group middleware — see its doc comment.
	api.Post("/auth/login", apitoken.APILoginRWeb)
	api.Get("/auth/me", apitoken.APIGuard(apitoken.APIMeRWeb))
	api.Post("/auth/logout", apitoken.APIGuard(apitoken.APILogoutRWeb))
	api.Post("/auth/logout-all", apitoken.APIGuard(apitoken.APILogoutAllRWeb)) // lost-phone remedy

	// Phase 2 payments: create-intent is public like the web giving form
	// (guest giving needs no account; abuse-limited per IP, not CSRF — see
	// resource/payment/api_rweb.go). History is personal → Bearer guard.
	api.Post("/payments/create-intent", payment.APICreateIntentRWeb)
	api.Get("/payments/history", apitoken.APIGuard(payment.APIPaymentHistoryRWeb))

	// Mobile chat + prayer wall. Reads are public (a placed chat/wall is
	// visible like article comments); writes ride the Bearer guard. Live
	// updates: the app may use the same /chat/stream SSE endpoint as the web
	// widget, or poll the list endpoint with after_id.
	api.Get("/chat/messages", chat.APIChatMessagesRWeb)
	api.Post("/chat/messages", apitoken.APIGuard(chat.APIChatPostRWeb))
	api.Post("/chat/messages/:id/keep", apitoken.APIGuard(chat.APIChatKeepRWeb))
	api.Delete("/chat/messages/:id", apitoken.APIGuard(chat.APIChatDeleteRWeb))
	api.Get("/prayer-requests", prayerwall.APIPrayerRequestsRWeb)
	api.Post("/prayer-requests", apitoken.APIGuard(prayerwall.APIPrayerPostRWeb))
	api.Post("/prayer-requests/:id/answered", apitoken.APIGuard(prayerwall.APIPrayerAnsweredRWeb))
	api.Delete("/prayer-requests/:id", apitoken.APIGuard(prayerwall.APIPrayerDeleteRWeb))

	// Ops endpoints, not part of /api/v1 (they aren't a mobile-app contract).
	// Both share one static bearer token — see gateOps for why not APIGuard.
	// backup: triggers a consistent DB snapshot to object storage, normally
	// invoked by the per-site k8s CronJob (deploy/k8s/sites/*.yaml).
	// replication: reports WAL-shipping lag for the continuous tier.
	s.Post("/api/admin/db/backup", dbbackup.APIBackupRWeb)
	s.Get("/api/admin/db/replication", dbbackup.APIReplicationStatusRWeb)

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

	// Live chat (web widget endpoints — JSON over the session cookie).
	// The SSE stream stays outside the session group: it needs no identity
	// (reads are public) and a long-lived stream shouldn't hold session
	// bookkeeping per connect.
	s.Get("/chat/stream", chat.StreamHandler(s))
	cht := s.Group("/chat", authctlr.UseCustomContextRWeb)
	cht.Get("/messages", chat.ListMessagesRWeb)
	cht.Post("/messages", chat.PostMessageRWeb)
	cht.Post("/keep/:id", chat.KeepMessageRWeb)     // editor+: exempt from the daily sweep
	cht.Post("/delete/:id", chat.DeleteMessageRWeb) // editor+: moderation removal

	// Prayer wall — prebuilt page plus form-post handlers (deletes/updates
	// are POSTs with CSRF tokens, matching the site's web convention).
	pwall := s.Group("/prayer-wall", authctlr.UseCustomContextRWeb)
	pwall.Get("", func(ctx rweb.Context) error {
		pg, err := page.PrayerWall()
		if err != nil {
			return err
		}
		return ctx.WriteHTML(string(basectlr.RenderPageNewRWeb(pg, ctx)))
	})
	preq := s.Group("/prayer-requests", authctlr.UseCustomContextRWeb)
	preq.Post("", prayerwall.PostRequestRWeb)
	preq.Post("/answered/:id", prayerwall.MarkAnsweredRWeb)
	preq.Post("/delete/:id", prayerwall.DeleteRequestRWeb)

	// Community chat — the chat module in its standalone, top-level role
	cchat := s.Group("/community-chat", authctlr.UseCustomContextRWeb)
	cchat.Get("", func(ctx rweb.Context) error {
		pg, err := page.CommunityChat()
		if err != nil {
			return err
		}
		return ctx.WriteHTML(string(basectlr.RenderPageNewRWeb(pg, ctx)))
	})

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

	// Admin area (permission-guarded); see RegisterAdminRoutes
	RegisterAdminRoutes(s)

	// Start the server
	if err := s.Run(); err != nil {
		logger.LogErr(err, "failed to start server")
	}
}

// RegisterAdminRoutes wires the permission-guarded admin area onto s. It is
// split out of ServeRWeb so checks (admin_routes_smoke_test.go) drive exactly
// the production wiring, not a copy of it that could drift.
func RegisterAdminRoutes(s *rweb.Server) {
	// Admin group. AdminGuardRWeb resolves the signed-in admin and their
	// permissions; it cannot by itself stop a handler from running (see the
	// note above auth_controller.AdminGuardRWeb), so EVERY admin route is
	// wrapped in Require(permission) or RequireAdmin. A route added here
	// without one is reachable by anyone.
	//
	// Convention: list = read, new form + create POST = create,
	// edit form + update POST = update, delete POST = delete. Publish/enable
	// are field-level and resolved inside the upsert handlers
	// (authz.ResolveFlag).
	ad := s.Group(config.AdminPrefix, authctlr.UseCustomContextRWeb, authctlr.AdminGuardRWeb)
	req := authctlr.Require

	ad.Get("/home", authctlr.RequireAdmin(admin_controller.AdminHandlerRWeb))
	ad.Get("/logout", authctlr.RequireAdmin(authctlr.LogoutHandlerRWeb))

	// Admin Users
	ad.Get("/users", req(authz.UsersRead, user_controller.ListUsersRWeb))
	ad.Get("/users/new", req(authz.UsersCreate, user_controller.NewUserRWeb))
	ad.Post("/users", req(authz.UsersCreate, user_controller.UpsertUserRWeb)) // create
	ad.Get("/users/edit/:id", req(authz.UsersUpdate, user_controller.EditUserRWeb))
	ad.Post("/users/update/:id", req(authz.UsersUpdate, user_controller.UpsertUserRWeb)) // update
	// Deletes are POSTs (CSRF-token checked in the handlers): GET deletes are
	// trivially forgeable via <img src>, and link prefetchers can fire them.
	ad.Post("/users/delete/:id", req(authz.UsersDelete, user_controller.DeleteUserRWeb))

	// Role Management
	ad.Get("/roles", req(authz.RolesRead, role_controller.ListRolesRWeb))
	ad.Get("/roles/new", req(authz.RolesCreate, role_controller.NewRoleRWeb))
	ad.Post("/roles", req(authz.RolesCreate, role_controller.UpsertRoleRWeb)) // create
	ad.Get("/roles/edit/:id", req(authz.RolesUpdate, role_controller.EditRoleRWeb))
	ad.Post("/roles/update/:id", req(authz.RolesUpdate, role_controller.UpsertRoleRWeb)) // update
	ad.Post("/roles/delete/:id", req(authz.RolesDelete, role_controller.DeleteRoleRWeb))

	// Giving records (read-only: charges are written by Stripe, not admins)
	ad.Get("/giving", req(authz.ChargesRead, payment_controller.AdminListGivingRWeb))
	// CSV export of the year shown on /giving (same ?year=). It exposes the
	// same donor data as the page, so it takes the same permission.
	ad.Get("/giving/csv", req(authz.ChargesRead, payment_controller.AdminGivingCSVRWeb))
	// Month totals only (same year, same data, same permission)
	ad.Get("/giving/csv/summary", req(authz.ChargesRead, payment_controller.AdminGivingSummaryCSVRWeb))

	// Admin Articles
	ad.Get("/articles", req(authz.ArticlesRead, article_controller.AdminListArticlesRWeb))
	ad.Get("/articles/new", req(authz.ArticlesCreate, article_controller.NewArticleRWeb))
	ad.Post("/articles", req(authz.ArticlesCreate, article_controller.UpsertArticleRWeb)) // create
	ad.Get("/articles/edit/:id", req(authz.ArticlesUpdate, article_controller.EditArticleRWeb))
	ad.Post("/articles/update/:id", req(authz.ArticlesUpdate, article_controller.UpsertArticleRWeb)) // update
	ad.Post("/articles/delete/:id", req(authz.ArticlesDelete, article_controller.DeleteArticleRWeb))

	// Admin Sermons
	ad.Get("/sermons", req(authz.SermonsRead, sermon_controller.AdminListSermonsRWeb))
	ad.Get("/sermons/new", req(authz.SermonsCreate, sermon_controller.NewSermonRWeb))
	ad.Get("/sermons/import", req(authz.SermonsCreate, sermon_controller.ImportRWeb))     // confirmation screen
	ad.Post("/sermons/import", req(authz.SermonsCreate, sermon_controller.ImportRunRWeb)) // runs the import (CSRF checked)
	ad.Post("/sermons", req(authz.SermonsCreate, sermon_controller.UpsertSermonRWeb))     // create
	ad.Get("/sermons/edit/:id", req(authz.SermonsUpdate, sermon_controller.EditSermonRWeb))
	ad.Post("/sermons/update/:id", req(authz.SermonsUpdate, sermon_controller.UpsertSermonRWeb)) // update
	ad.Post("/sermons/delete/:id", req(authz.SermonsDelete, sermon_controller.DeleteSermonRWeb))
	// Local sermon-cache cleanup tool (lists copies safe to delete, batch-deletes them).
	// It removes local cached copies, never sermons, so it rides update rather
	// than delete.
	ad.Get("/sermons/cleanup", req(authz.SermonsUpdate, sermon_controller.AdminSermonCleanupRWeb))
	ad.Post("/sermons/cleanup", req(authz.SermonsUpdate, sermon_controller.AdminSermonCleanupRunRWeb))

	// Admin Events
	ad.Get("/events", req(authz.EventsRead, event_controller.AdminListEventsRWeb))
	ad.Get("/events/new", req(authz.EventsCreate, event_controller.NewEventRWeb))
	ad.Post("/events", req(authz.EventsCreate, event_controller.UpsertEventRWeb)) // create
	ad.Get("/events/edit/:id", req(authz.EventsUpdate, event_controller.EditEventRWeb))
	ad.Post("/events/update/:id", req(authz.EventsUpdate, event_controller.UpsertEventRWeb)) // update
	ad.Post("/events/delete/:id", req(authz.EventsDelete, event_controller.DeleteEventRWeb))

	// Admin Pages
	ad.Get("/pages", req(authz.PagesRead, page_controller.AdminListPagesRWeb))
	ad.Get("/pages/new", req(authz.PagesCreate, page_controller.NewPageRWeb))
	ad.Post("/pages", req(authz.PagesCreate, page_controller.UpsertPageRWeb))     // create
	ad.Get("/pages/:id", req(authz.PagesRead, page_controller.AdminShowPageRWeb)) // preview
	ad.Get("/pages/edit/:id", req(authz.PagesUpdate, page_controller.EditPageRWeb))
	ad.Post("/pages/update/:id", req(authz.PagesUpdate, page_controller.UpsertPageRWeb)) // update
	ad.Post("/pages/delete/:id", req(authz.PagesDelete, page_controller.DeletePageRWeb))

	// Admin Menus
	ad.Get("/menus", req(authz.MenusRead, menu_controller.AdminListMenusRWeb))
	ad.Get("/menus/new", req(authz.MenusCreate, menu_controller.NewMenuRWeb))
	ad.Post("/menus", req(authz.MenusCreate, menu_controller.UpsertMenuRWeb)) // create
	ad.Get("/menus/edit/:id", req(authz.MenusUpdate, menu_controller.EditMenuRWeb))
	ad.Post("/menus/update/:id", req(authz.MenusUpdate, menu_controller.UpsertMenuRWeb)) // update
	ad.Post("/menus/delete/:id", req(authz.MenusDelete, menu_controller.DeleteMenuRWeb))
}

// RegisterDebugRoutes wires the SuperAdmin-only /debug tools onto s. It is
// split out of ServeRWeb for the same reason as RegisterAdminRoutes: the
// smoke test (admin_routes_smoke_test.go) drives the production wiring.
func RegisterDebugRoutes(s *rweb.Server) {
	dbg := s.Group("/debug", authctlr.UseCustomContextRWeb, authctlr.AdminGuardRWeb)
	dbg.Get("/set", authctlr.RequireSuper(func(ctx rweb.Context) error {
		element.DebugSet()
		return ctx.WriteHTML("<h3>Debug mode set.</h3> <a href='/'>Home</a>")
	}))

	dbg.Get("/show", authctlr.RequireSuper(func(ctx rweb.Context) error {
		return ctx.WriteHTML(element.DebugShow())
	}))

	dbg.Get("/clear", authctlr.RequireSuper(func(ctx rweb.Context) error {
		element.DebugClear()
		return ctx.WriteHTML("<h3>Debug mode is off.</h3> <a href='/'>Home</a>")
	}))

	dbg.Get("/clear-issues", authctlr.RequireSuper(func(ctx rweb.Context) error {
		element.DebugClearIssues()
		return ctx.WriteHTML("<h3>Issues cleared (debug mode still active).</h3> <a href='/'>Home</a> | <a href='/debug/show'>View Debug</a>")
	}))
}
