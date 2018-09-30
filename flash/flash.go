package flash

import (
	"encoding/json"
	"encoding/base64"
	"github.com/labstack/echo"
	"bytes"
	"github.com/rohanthewiz/church/chweb/resource/cookie"
)

const flash_cookie_name = "flash_cookie"

type Flash struct {
	Info string `json:"info"`
	Warn string `json:"warn"`
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
	out := new(bytes.Buffer)
	ows := out.WriteString
	if f.Info != "" || f.Warn != "" || f.Error != "" {
		ows(`<div id="flash" onclick="this.style.display = 'none';" title="Click to close">`)
		if f.Info != "" {
			ows(`<div class="flash-info">`)
			ows(f.Info)
		}
		if f.Warn != "" {
			ows(`<div class="flash-warn">`)
			ows(f.Warn)
		}
		if f.Error != "" {
			ows(`<div class="flash-error">`)
			ows(f.Error)
		}
		ows(`<div class="flash-close"> <b>X</b></div>`)
		ows(`</div></div>`)
	}

	return out.String()
}