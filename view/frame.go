package view

import (
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/church/config"
	"time"
)

var PageFrame frame

func init() {
	PageFrame = frame{}
}

// The Page Frame - Header and Footer info
type frame struct {
	banner string // cache rendered banner
	copyright string
}

// Provide cached banner
func (f frame) GetBanner() (out string) {
	if f.banner != "" { return f.banner } // return cached

	e := element.New
	out = e("div", "id", "banner", "class", "theme-" + config.Options.Theme).R(
		e("h1").R("Your Church or Business"),
		e("h3").R("We are full of life"),
		e("img", "class", "logo-right", "src", "/assets/images/bible_white_bkgnd.png").R(),
	)
	return
}

// Return the copyright that goes in the footer
func (f frame) GetCopyright() (out string) {
	if f.copyright != "" { return f.copyright } // cached

	return  element.New("div", "id", "credit").R(
		"&copy; " + time.Now().Format("2006") + " Churh or Business",
	)
	return
}