package church

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/rohanthewiz/church/admin"

	"github.com/rohanthewiz/church/page_controller"
	customctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/auth_controller"
	"github.com/rohanthewiz/church/admin_controller"
	"github.com/rohanthewiz/church/user_controller"
	"github.com/rohanthewiz/church/event_controller"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/sermon_controller"
	"github.com/rohanthewiz/church/article_controller"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/menu_controller"
	"github.com/rohanthewiz/church/resource/calendar"
)

func Serve() {
	admin.AuthBootstrap()
	page.RegisterModules()

	e := echo.New()
	//e.Pre(middleware.HTTPSWWWRedirect())

	e.Static("/assets", "dist")
	e.Static("/media", "sermons")
	e.GET("/", page_controller.HomePage)

	e.GET("/login", auth_controller.LoginHandler)
	e.GET("/logout", auth_controller.LogoutHandler)
	e.POST("/auth", auth_controller.AuthHandler) // Attempt login

	//?username=joe&password=secret&token=abc12345678&
	e.GET("/super", admin_controller.SetupSuperAdmin) // (API) Establish first SuperAdmin

	// API
	e.GET("/calendar", calendar.GetFullCalendarEvents)
	//e.GET("/adduser", auth_controller.RegisterUser)  // todo auth! POST (bootstrap super admin)

	// Non-admin dynamic pages (the majority of the pages) are handled here
	pgs := e.Group("pages")
	pgs.Use(customctx.UseCustomNonAdminContext)
	pgs.Use(auth_controller.StoreSessionInContext) // check if logged in and store on our custom context
	pgs.GET("/:slug", page_controller.PageHandler)

	// Articles
	art := e.Group("articles")
	art.Use(customctx.UseCustomNonAdminContext)
	art.Use(auth_controller.StoreSessionInContext) // check if logged in and store on our custom context
	art.GET("", article_controller.ListArticles)
	art.GET("/:id", article_controller.ShowArticle)

	// Events
	evt := e.Group("events")
	evt.Use(customctx.UseCustomNonAdminContext)
	evt.Use(auth_controller.StoreSessionInContext) // store authentication in custom context
	evt.GET("", event_controller.ListEvents)
	evt.GET("/:id", event_controller.ShowEvent)

	// Sermons
	ser := e.Group("sermons")
	ser.Use(customctx.UseCustomNonAdminContext)
	ser.Use(auth_controller.StoreSessionInContext) // store authentication in custom context
	ser.GET("", sermon_controller.ListSermons)
	ser.GET("/:id", sermon_controller.ShowSermon)

	// Admin group uses authentication middleware
	ad := e.Group(config.AdminPrefix)
	ad.Use(func(handler echo.HandlerFunc) echo.HandlerFunc {  // use custom context
		return func(c echo.Context) error {
					cc := &customctx.CustomContext{ c, false, "administrator", "" }
					return handler(cc)
				}
	})
	ad.Use(auth_controller.StoreSessionInContext) // store authentication in custom context
	ad.Use(auth_controller.AuthAdmin)             // require admin privileges in admin
	ad.GET("/home", admin_controller.AdminHandler)

	ad.GET("/logout", auth_controller.LogoutHandler)

	ad.GET("/users", user_controller.ListUsers)
	ad.GET("/users/new", user_controller.NewUser)
	ad.POST("/users", user_controller.UpsertUser)  // create
	ad.GET("/users/edit/:id", user_controller.EditUser)
	ad.POST("/users/update/:id", user_controller.UpsertUser)  // update
	ad.GET("/users/delete/:id", user_controller.DeleteUser)  // update

	ad.GET("/articles", article_controller.AdminListArticles)
	ad.GET("/articles/new", article_controller.NewArticle)
	ad.POST("/articles", article_controller.UpsertArticle)  // create
	ad.GET("/articles/edit/:id", article_controller.EditArticle)
	ad.POST("/articles/update/:id", article_controller.UpsertArticle)  // update
	ad.GET("/articles/delete/:id", article_controller.DeleteArticle)

	ad.GET("/sermons", sermon_controller.AdminListSermons)
	ad.GET("/sermons/new", sermon_controller.NewSermon)
	ad.GET("/sermons/import", sermon_controller.Import)
	ad.POST("/sermons", sermon_controller.UpsertSermon)  // create
	ad.GET("/sermons/edit/:id", sermon_controller.EditSermon)
	ad.POST("/sermons/update/:id", sermon_controller.UpsertSermon)  // update
	ad.GET("/sermons/delete/:id", sermon_controller.DeleteSermon)

	ad.GET("/events", event_controller.AdminListEvents)
	ad.GET("/events/new", event_controller.NewEvent)
	ad.POST("/events", event_controller.UpsertEvent)  // create
	ad.GET("/events/edit/:id", event_controller.EditEvent)
	ad.POST("/events/update/:id", event_controller.UpsertEvent)  // update
	ad.GET("/events/delete/:id", event_controller.DeleteEvent)

	ad.GET("/pages", page_controller.AdminListPages)
	ad.GET("/pages/new", page_controller.NewPage)
	ad.POST("/pages", page_controller.UpsertPage)  // create
	ad.GET("/pages/:id", page_controller.AdminShowPage)  // preview
	ad.GET("/pages/edit/:id", page_controller.EditPage)
	ad.POST("/pages/update/:id", page_controller.UpsertPage)  // update
	ad.GET("/pages/delete/:id", page_controller.DeletePage)

	ad.GET("/menus", menu_controller.AdminListMenus)
	ad.GET("/menus/new", menu_controller.NewMenu)
	ad.POST("/menus", menu_controller.UpsertMenu)  // create
	ad.GET("/menus/edit/:id", menu_controller.EditMenu)
	ad.POST("/menus/update/:id", menu_controller.UpsertMenu)  // update
	ad.GET("/menus/delete/:id", menu_controller.DeleteMenu)

	if config.AppEnv != "development" && config.Options.Server.UseTLS {
		startTLS(e)
	} else {
		e.Logger.Fatal(e.Start(":" + config.Options.Server.Port))
	}

}

func startTLS(e *echo.Echo) {
	e.Logger.Fatal(e.StartTLS("0.0.0.0:" + config.Options.Server.Port,
		config.Options.Server.CertFile, config.Options.Server.KeyFile))
}

//func startAutoTLS(e *echo.Echo) {
//	e.AutoTLSManager.HostPolicy = autocert.HostWhitelist(config.Options.Server.Domain)
//	e.AutoTLSManager.Cache = autocert.DirCache("/var/certs")
//	e.Logger.Fatal(e.StartAutoTLS(":" + config.Options.Server.Port))
//}
