package cookie

import (
	"net/http"
	"time"

	"github.com/rohanthewiz/rweb"
)

// SetRWeb sets a cookie in RWeb context
func SetRWeb(ctx rweb.Context, name, val string) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(ctx.Response(), cookie)
}

// GetRWeb retrieves a cookie value from RWeb context
func GetRWeb(ctx rweb.Context, name string) (string, error) {
	cookie, err := ctx.Request().Request().Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// ClearRWeb removes a cookie by setting it with an expired time
func ClearRWeb(ctx rweb.Context, name string) error {
	cookie := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	}
	http.SetCookie(ctx.Response(), cookie)
	return nil
}

// GetAndClearRWeb retrieves a cookie value and then clears it
func GetAndClearRWeb(ctx rweb.Context, name string) (string, error) {
	val, err := GetRWeb(ctx, name)
	if err != nil {
		return "", err
	}
	ClearRWeb(ctx, name)
	return val, nil
}