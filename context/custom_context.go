package context

import "github.com/labstack/echo"

type CustomContext struct {
	echo.Context
	Admin bool
	Username string
	FormReferrer string
}

func UseCustomNonAdminContext(handler echo.HandlerFunc) echo.HandlerFunc {  // use custom context
	return func(c echo.Context) error {
		cc := &CustomContext{
			c, false, "", "",
		}
		return handler(cc)
	}
}
