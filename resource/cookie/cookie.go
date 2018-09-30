package cookie

import (
	"github.com/labstack/echo"
	"net/http"
)

func Set(c echo.Context, name, val string) {
	cke := new(http.Cookie)
	cke.Name = name
	set(c, cke, val)
}

func Get(c echo.Context, name string) (str string, err error) {
	cke, err := get(c, name)
	if err != nil {
		return
	}
	str = cke.Value
	return
}

func Clear(c echo.Context, name string) (err error) {
	_, err = GetAndClear(c, name)
	return
}

func GetAndClear(c echo.Context, name string) (str string, err error) {
	cke, err := get(c, name)
	if err != nil {
		return
	}
	str = cke.Value
	clear(c, cke)
	return
}

// Private

func set(c echo.Context, cke *http.Cookie, val string) {
	cke.Value = val
	cke.Path = "/"
	// For session cookies, don't set an expiration so it might be removed on browser window close
	// cookie.Expires = time.Now().Add(24 * time.Hour)
	c.SetCookie(cke)
}

func get(c echo.Context, name string) (cke *http.Cookie, err error) {
	return c.Cookie(name)
}

func clear(c echo.Context, cke *http.Cookie) {
	set(c, cke, "")
}
