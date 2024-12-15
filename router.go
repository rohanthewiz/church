package church

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo"
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
	"github.com/rohanthewiz/logger"
)

// todo !! setup cert renew on a chron job

//go:generate go run pack/packer.go

func Serve() {
	admin.AuthBootstrap()
	page.RegisterModules()

	idrive.InitClient()

	e := echo.New()
	// Did not work -> e.Pre(middleware.HTTPSWWWRedirect())

	e.Static("/assets", "dist")
	e.Static("/media", "sermons")
	e.GET("/", page_controller.HomePage)

	e.GET("/login", authctlr.LoginHandler)
	e.GET("/logout", authctlr.LogoutHandler)
	e.POST("/auth", authctlr.AuthHandler) // Attempt login

	// ?username=joe&password=secret&token=abc12345678&
	e.GET("/super", admin_controller.SetupSuperAdmin) // (API) Establish first SuperAdmin

	// API
	e.GET("/api/v1/sermons", sermon.APISermons)
	e.GET("/calendar", calendar.GetFullCalendarEvents)
	// e.GET("/adduser", authctlr.RegisterUser)  // todo auth! POST (bootstrap super admin)

	// Non-admin dynamic pages (the majority of the pages) are handled here
	pgs := e.Group("pages")
	pgs.Use(authctlr.UseCustomContext) // check if logged in and store on our custom context
	pgs.GET("/:slug", page_controller.PageHandler)

	// Articles
	art := e.Group("articles")
	art.Use(authctlr.UseCustomContext) // check if logged in and store on our custom context
	art.GET("", article_controller.ListArticles)
	art.GET("/:id", article_controller.ShowArticle)

	// Events
	evt := e.Group("events")
	evt.Use(authctlr.UseCustomContext) // store authentication in custom context
	evt.GET("", event_controller.ListEvents)
	evt.GET("/:id", event_controller.ShowEvent)

	// Payments
	pay := e.Group("payments")
	pay.Use(authctlr.UseCustomContext) // store authentication in custom context
	pay.GET("/new", payment_controller.NewPayment)
	pay.POST("/create", payment_controller.UpsertPayment) // create
	pay.GET("/receipt", payment_controller.PaymentReceipt)

	// Sermons
	ser := e.Group("sermons")
	ser.Use(authctlr.UseCustomContext) // store authentication in custom context
	ser.GET("", sermon_controller.ListSermons)
	ser.GET("/:id", sermon_controller.ShowSermon)

	// TODO - move this code to the sermons controller
	sergrp := e.Group("sermons")
	sergrp.GET("/:year/:filename", func(c echo.Context) error {
		year := c.Param("year")
		filename := c.Param("filename")
		fmt.Printf("**-> year %s, sermon %s\n", year, filename)

		byts, err := idrive.GetSermon(year, filename)
		if err != nil {
			logger.Err(err, "error getting sermon", "year", year, "sermon", filename)
			return c.JSON(http.StatusNotImplemented, map[string]string{
				"message": "Sorry, we couldn't find the sermon you requested.",
				"error":   err.Error(),
			})
		}

		return basectlr.SendAudioFile(c, filename, byts)
	})

	// Admin group uses authentication middleware
	ad := e.Group(config.AdminPrefix)
	ad.Use(authctlr.UseCustomContext) // store authentication in custom context
	ad.Use(authctlr.AdminGuard)       // require admin privileges in admin - this should be the last middleware

	ad.GET("/home", admin_controller.AdminHandler)

	ad.GET("/logout", authctlr.LogoutHandler)

	ad.GET("/users", user_controller.ListUsers)
	ad.GET("/users/new", user_controller.NewUser)
	ad.POST("/users", user_controller.UpsertUser) // create
	ad.GET("/users/edit/:id", user_controller.EditUser)
	ad.POST("/users/update/:id", user_controller.UpsertUser) // update
	ad.GET("/users/delete/:id", user_controller.DeleteUser)  // update

	ad.GET("/articles", article_controller.AdminListArticles)
	ad.GET("/articles/new", article_controller.NewArticle)
	ad.POST("/articles", article_controller.UpsertArticle) // create
	ad.GET("/articles/edit/:id", article_controller.EditArticle)
	ad.POST("/articles/update/:id", article_controller.UpsertArticle) // update
	ad.GET("/articles/delete/:id", article_controller.DeleteArticle)

	ad.GET("/sermons", sermon_controller.AdminListSermons)
	ad.GET("/sermons/new", sermon_controller.NewSermon)
	ad.GET("/sermons/import", sermon_controller.Import)
	ad.POST("/sermons", sermon_controller.UpsertSermon) // create
	ad.GET("/sermons/edit/:id", sermon_controller.EditSermon)
	ad.POST("/sermons/update/:id", sermon_controller.UpsertSermon) // update
	ad.GET("/sermons/delete/:id", sermon_controller.DeleteSermon)

	ad.GET("/events", event_controller.AdminListEvents)
	ad.GET("/events/new", event_controller.NewEvent)
	ad.POST("/events", event_controller.UpsertEvent) // create
	ad.GET("/events/edit/:id", event_controller.EditEvent)
	ad.POST("/events/update/:id", event_controller.UpsertEvent) // update
	ad.GET("/events/delete/:id", event_controller.DeleteEvent)

	ad.GET("/pages", page_controller.AdminListPages)
	ad.GET("/pages/new", page_controller.NewPage)
	ad.POST("/pages", page_controller.UpsertPage)       // create
	ad.GET("/pages/:id", page_controller.AdminShowPage) // preview
	ad.GET("/pages/edit/:id", page_controller.EditPage)
	ad.POST("/pages/update/:id", page_controller.UpsertPage) // update
	ad.GET("/pages/delete/:id", page_controller.DeletePage)

	ad.GET("/menus", menu_controller.AdminListMenus)
	ad.GET("/menus/new", menu_controller.NewMenu)
	ad.POST("/menus", menu_controller.UpsertMenu) // create
	ad.GET("/menus/edit/:id", menu_controller.EditMenu)
	ad.POST("/menus/update/:id", menu_controller.UpsertMenu) // update
	ad.GET("/menus/delete/:id", menu_controller.DeleteMenu)

	if config.AppEnv != "development" && config.Options.Server.UseTLS {
		startTLS(e)
	} else {
		e.Logger.Fatal(e.Start(":" + config.Options.Server.Port))
	}
}

func startTLS(e *echo.Echo) {
	e.Logger.Fatal(e.StartTLS("0.0.0.0:"+config.Options.Server.Port,
		config.Options.Server.CertFile, config.Options.Server.KeyFile))
}

// func startAutoTLS(e *echo.Echo) {
//	e.AutoTLSManager.HostPolicy = autocert.HostWhitelist(config.Options.Server.Domain)
//	e.AutoTLSManager.Cache = autocert.DirCache("/var/certs")
//	e.Logger.Fatal(e.StartAutoTLS(":" + config.Options.Server.Port))
// }
