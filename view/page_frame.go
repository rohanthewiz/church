package view

import (
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/element"
)

var PgFrame pageFrame

func init() {
	PgFrame = pageFrame{}
}

// pageFrame will hold rendered elements of the page frame, both header and footer info
type pageFrame struct {
	banner    string // cache rendered banner
	bannerExt string // typically area below banner
	copyright string // typically bottom of the page
}

// GetBanner returns the cached banner
func (f pageFrame) GetBanner() (out string) {
	if f.banner != "" {
		return f.banner // return cached
	}

	b := element.NewBuilder()
	e := b.Ele
	e("div", "id", "banner", "class", "theme-"+config.Options.Theme).R(
		e("div", "id", "banner-wrapper").R(
			b.WS(config.Options.BannerInnerHTML),
		),
		e("div", "id", "banner-extension").R(
			b.WS(config.Options.BannerExt),
		),
	)

	return b.String()
}

// Return the copyright that goes in the footer
func (f pageFrame) GetCopyright() (out string) {
	if f.copyright != "" {
		return f.copyright
	} // cached

	b := element.NewBuilder()
	b.Ele("div", "id", "copyright").R(
		b.WS("&copy; " + time.Now().Format("2006") + " " + config.Options.CopyrightOwner),
	)
	return b.String()
}
