package view

import (
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/element"
)

var PageFrame frame

func init() {
	PageFrame = frame{}
}

// The Page Frame - Header and Footer info
type frame struct {
	banner    string // cache rendered banner
	copyright string
}

// Provide cached banner
func (f frame) GetBanner() (out string) {
	if f.banner != "" {
		return f.banner
	} // return cached
	b := element.NewBuilder()
	e := b.Ele
	e("div", "id", "banner", "class", "theme-"+config.Options.Theme).R(
		e("div", "id", "banner-wrapper").R(
			config.Options.BannerInnerHTML,
		),
		e("div", "id", "logo-extension").R(
		// "Join us on Sundays at 9:45",
		// e("a", "href", "/pages/contact-us").R("Contact Us"),
		),
	)

	return b.String()
}

// Return the copyright that goes in the footer
func (f frame) GetCopyright() (out string) {
	if f.copyright != "" {
		return f.copyright
	} // cached

	return element.New("div", "id", "copyright").R(
		"&copy; " + time.Now().Format("2006") + " " + config.Options.CopyrightOwner,
	)
	return
}
