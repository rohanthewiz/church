package flash

import (
	"encoding/base64"
	"encoding/json"
	"html"

	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/rweb"
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

// SetRWeb sets flash message for RWeb context
func (f Flash) SetRWeb(ctx rweb.Context) error {
	byts, err := json.Marshal(f)
	if err != nil {
		return err
	}
	b64d := base64.StdEncoding.EncodeToString(byts)
	return ctx.SetCookie(flash_cookie_name, b64d)
}

// GetRWeb retrieves flash message from RWeb context
func GetRWeb(ctx rweb.Context) (*Flash, error) {
	cookieVal, err := ctx.GetCookieAndClear(flash_cookie_name)
	if err != nil {
		return nil, err
	}
	byts, err := base64.StdEncoding.DecodeString(cookieVal)
	if err != nil {
		return nil, err
	}

	fl := NewFlash()
	err = json.Unmarshal(byts, fl)
	return fl, err
}

// GetOrNewRWeb retrieves flash or creates new one for RWeb
func GetOrNewRWeb(ctx rweb.Context) *Flash {
	fl, err := GetRWeb(ctx)
	if err != nil {
		fl = NewFlash()
	}
	return fl
}

func (f Flash) Render() string {
	if f.Info == "" && f.Warn == "" && f.Error == "" {
		return ""
	}

	b := element.NewBuilder()

	// role=status + aria-live announces saves/errors to screen readers; the
	// close control is a real focusable button (the whole banner stays
	// clickable for mouse users as before). Messages are escaped — they can
	// embed request-path data and element writes strings through raw.
	b.Div("id", "flash", "role", "status", "aria-live", "polite",
		"onclick", "this.style.display = 'none';", "title", "Click to close").R(
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
						b.T(html.EscapeString(flash.message)),
						b.Button("class", "flash-close", "type", "button", "aria-label", "Dismiss message",
							"onclick", "document.getElementById('flash').style.display='none';").R(
							b.B().T("X"),
						),
					)
				}
			}
		}),
	)

	return b.String()
}
