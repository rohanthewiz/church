package church

import (
	"fmt"
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
	"github.com/rohanthewiz/church/resource/apiv1"
	"github.com/rohanthewiz/church/resource/article"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/resource/calendar"
	"github.com/rohanthewiz/church/resource/chat"
	"github.com/rohanthewiz/church/resource/chimage"
	"github.com/rohanthewiz/church/resource/dbbackup"
	"github.com/rohanthewiz/church/resource/event"
	"github.com/rohanthewiz/church/resource/feed"
	"github.com/rohanthewiz/church/resource/payment"
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

	// Track in-flight requests so shutdown can let them finish before the
	// database closes (see shutdown_rweb.go). First in the chain, so it wraps
	// every route registered below.
	reqs := &inflight{}
	s.Use(reqs.middleware)

	// Static files
	s.StaticFiles("/assets/", "dist", 1)
	// Editor-uploaded article images: local cache first, then IDrive e2 (see
	// resource/chimage/store.go). This route is more specific than the static
	// /assets/*path route, so it takes /assets/img/ requests.
	s.Get("/assets/img/:filename", chimage.ServeImageRWeb)
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
	// Run returns on SIGINT/SIGTERM (or a listen failure): finish open
	// requests, then close the database cleanly
	reqs.shutdown(shutdownDrainTimeout)
}

// RegisterAdminRoutes wires the permission-guarded admin area onto s. It is
// split out of ServeRWeb so checks (admin_routes_smoke_test.go) drive exactly
// the production wiring, not a copy of it that could drift.
func RegisterAdminRoutes(s *rweb.Server) {
	// Admin group. AdminGuardRWeb resolves the signed-in admin and their
	// permissions; it cannot by itself stop a handler from running (see the
	// note above auth_controller.AdminGuardRWeb), so EVERY admin route is
	// registered through get/post below, which wrap it in Require(permission).
	// A route added with ad.Get/ad.Post directly is reachable by anyone.
	//
	// The permission convention (list = read, new/create = create, ...) is
	// documented with the table in resource/authz/admin_routes.go.
	ad := s.Group(config.AdminPrefix, authctlr.UseCustomContextRWeb, authctlr.AdminGuardRWeb)

	// Each route's permission comes from authz.AdminRoutes, the table the nav
	// (menu.linkPermitted) and the dashboard cards also read, so the three
	// cannot drift apart. guarded panics on a route the table lacks: that
	// fails the first boot (and the smoke test) instead of leaving a route
	// unguarded or the nav out of step.
	guarded := func(method, path string, h rweb.Handler) rweb.Handler {
		perm, ok := authz.AdminRoutePerm(method, path)
		if !ok {
			panic(fmt.Sprintf("admin route %s %s%s has no row in authz.AdminRoutes", method, config.AdminPrefix, path))
		}
		return authctlr.Require(perm, h) // AdminAccess ("") = any admin
	}
	get := func(path string, h rweb.Handler) { ad.Get(path, guarded(http.MethodGet, path, h)) }
	post := func(path string, h rweb.Handler) { ad.Post(path, guarded(http.MethodPost, path, h)) }

	get("/home", admin_controller.AdminHandlerRWeb)
	get("/logout", authctlr.LogoutHandlerRWeb)

	// Admin Users
	get("/users", user_controller.ListUsersRWeb)
	get("/users/new", user_controller.NewUserRWeb)
	post("/users", user_controller.UpsertUserRWeb) // create
	get("/users/edit/:id", user_controller.EditUserRWeb)
	post("/users/update/:id", user_controller.UpsertUserRWeb) // update
	// Deletes are POSTs (CSRF-token checked in the handlers): GET deletes are
	// trivially forgeable via <img src>, and link prefetchers can fire them.
	post("/users/delete/:id", user_controller.DeleteUserRWeb)

	// Role Management
	get("/roles", role_controller.ListRolesRWeb)
	get("/roles/new", role_controller.NewRoleRWeb)
	post("/roles", role_controller.UpsertRoleRWeb) // create
	get("/roles/edit/:id", role_controller.EditRoleRWeb)
	post("/roles/update/:id", role_controller.UpsertRoleRWeb) // update
	post("/roles/delete/:id", role_controller.DeleteRoleRWeb)

	// Giving records (read-only: charges are written by Stripe, not admins)
	get("/giving", payment_controller.AdminListGivingRWeb)
	// CSV export of the year shown on /giving (same ?year=). It exposes the
	// same donor data as the page, so it takes the same permission.
	get("/giving/csv", payment_controller.AdminGivingCSVRWeb)
	// Month totals only (same year, same data, same permission)
	get("/giving/csv/summary", payment_controller.AdminGivingSummaryCSVRWeb)

	// Admin Articles
	get("/articles", article_controller.AdminListArticlesRWeb)
	get("/articles/new", article_controller.NewArticleRWeb)
	post("/articles", article_controller.UpsertArticleRWeb) // create
	get("/articles/edit/:id", article_controller.EditArticleRWeb)
	post("/articles/update/:id", article_controller.UpsertArticleRWeb) // update
	post("/articles/delete/:id", article_controller.DeleteArticleRWeb)

	// Admin Sermons
	get("/sermons", sermon_controller.AdminListSermonsRWeb)
	get("/sermons/new", sermon_controller.NewSermonRWeb)
	get("/sermons/import", sermon_controller.ImportRWeb)     // confirmation screen
	post("/sermons/import", sermon_controller.ImportRunRWeb) // runs the import (CSRF checked)
	post("/sermons", sermon_controller.UpsertSermonRWeb)     // create
	get("/sermons/edit/:id", sermon_controller.EditSermonRWeb)
	post("/sermons/update/:id", sermon_controller.UpsertSermonRWeb) // update
	post("/sermons/delete/:id", sermon_controller.DeleteSermonRWeb)
	// Local sermon-cache cleanup tool (lists copies safe to delete, batch-deletes them).
	// It removes local cached copies, never sermons, so it rides update rather
	// than delete.
	get("/sermons/cleanup", sermon_controller.AdminSermonCleanupRWeb)
	post("/sermons/cleanup", sermon_controller.AdminSermonCleanupRunRWeb)

	// Admin Events
	get("/events", event_controller.AdminListEventsRWeb)
	get("/events/new", event_controller.NewEventRWeb)
	post("/events", event_controller.UpsertEventRWeb) // create
	get("/events/edit/:id", event_controller.EditEventRWeb)
	post("/events/update/:id", event_controller.UpsertEventRWeb) // update
	post("/events/delete/:id", event_controller.DeleteEventRWeb)

	// Admin Pages
	get("/pages", page_controller.AdminListPagesRWeb)
	get("/pages/new", page_controller.NewPageRWeb)
	post("/pages", page_controller.UpsertPageRWeb)       // create
	get("/pages/:id", page_controller.AdminShowPageRWeb) // preview
	get("/pages/edit/:id", page_controller.EditPageRWeb)
	post("/pages/update/:id", page_controller.UpsertPageRWeb) // update
	post("/pages/delete/:id", page_controller.DeletePageRWeb)

	// Admin Menus
	get("/menus", menu_controller.AdminListMenusRWeb)
	get("/menus/new", menu_controller.NewMenuRWeb)
	post("/menus", menu_controller.UpsertMenuRWeb) // create
	get("/menus/edit/:id", menu_controller.EditMenuRWeb)
	post("/menus/update/:id", menu_controller.UpsertMenuRWeb) // update
	post("/menus/delete/:id", menu_controller.DeleteMenuRWeb)
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
