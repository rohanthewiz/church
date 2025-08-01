package flash

import (
	"encoding/base64"
	"encoding/json"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/resource/cookie"
	"github.com/rohanthewiz/element"
)

const flash_cookie_name = "flash_cookie"

type Flash struct {
	Info  string `json:"info"`
	Warn  string `json:"warn"`
	Error string `json:"error"`
}

func NewFlash() *Flash {
	return new(Flash)
}

func (f Flash) Set(c echo.Context) (err error) {
	byts, err := json.Marshal(f)
	if err != nil {
		return err
	}
	b64d := base64.StdEncoding.EncodeToString(byts)
	cookie.Set(c, flash_cookie_name, b64d)
	return
}

func GetOrNew(c echo.Context) (fl *Flash) {
	var err error
	fl, err = Get(c)
	if err != nil {
		fl = new(Flash)
	}
	return
}

func Get(c echo.Context) (*Flash, error) {
	cookie_val, err := cookie.GetAndClear(c, flash_cookie_name)
	if err != nil {
		return nil, err
	}
	byts, err := base64.StdEncoding.DecodeString(cookie_val)
	if err != nil {
		return nil, err
	}

	fl := NewFlash()
	err = json.Unmarshal(byts, fl)
	return fl, err
}

func (f Flash) Render() string {
	if f.Info == "" && f.Warn == "" && f.Error == "" {
		return ""
	}

	b := element.NewBuilder()

	b.Div("id", "flash", "onclick", "this.style.display = 'none';", "title", "Click to close").R(
		b.Wrap(func() {
			flashMessages := []struct {
				message   string
				flashType string
			}{
				{f.Info, "flash-info"},
				{f.Warn, "flash-warn"},
				{f.Error, "flash-error"},
			}

			for _, flash := range flashMessages {
				if flash.message != "" {
					b.DivClass(flash.flashType).R(
						b.T(flash.message),
						b.DivClass("flash-close").R(
							b.B().T("X"),
						),
					)
				}
			}
		}),
	)

	return b.String()
}
