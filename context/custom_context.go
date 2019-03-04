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

func UseCustomNonAdminContext(handler echo.HandlerFunc) echo.HandlerFunc {  // use custom context
	return func(c echo.Context) error {
		cc := &CustomContext{
			c, false, session.Session{},
		}
		return handler(cc)
	}
}
