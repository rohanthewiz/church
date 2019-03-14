package context

import (
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/resource/session"
)

type CustomContext struct {
	echo.Context
	Admin bool
	Session session.Session
	//Username string
	//FormReferrer string
}

// Wrap echo.Context in a custom context
// Use this for any routes (including all secured routes) that need to track a session
//func UseCustomContext(handler echo.HandlerFunc) echo.HandlerFunc {
//	return func(c echo.Context) error {
//		// Wrap existing context
//		cc := &CustomContext{
//			c, false, session.Session{},
//		}
//		// Get Session key
//		sessKey := auth_controller.EnsureSessionCookie()
//
//		return handler(cc)
//	}
//}

//func UseCustomContext(handler echo.HandlerFunc) echo.HandlerFunc {
//	return func(c echo.Context) error {
//		cc := &CustomContext{
//			c, false, session.Session{},
//		}
//		return handler(cc)
//	}
//}

//func UseCustomNonAdminContext(handler echo.HandlerFunc) echo.HandlerFunc {  // use custom context
//	return func(c echo.Context) error {
//		cc := &CustomContext{
//			c, false, session.Session{},
//		}
//		return handler(cc)
//	}
//}
