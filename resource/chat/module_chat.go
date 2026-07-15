package chat

import (
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/util/stringops"
	"github.com/rohanthewiz/element"
)

const ModuleTypeChat = "chat"

// ModuleChat is the top-level chat module: a full-height live chat placed as
// a page's main content (e.g. a "Community Chat" page). The channel key
// comes from Opts.ItemSlug when the page author sets one (letting several
// pages share or isolate conversations deliberately); otherwise it derives
// from the module title, falling back to a site-wide "community" room.
type ModuleChat struct {
	module.Presenter
}

func NewModuleChat(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleChat)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

// channel resolves this placement's channel key.
func (m ModuleChat) channel() string {
	if m.Opts.ItemSlug != "" && ValidChannel(m.Opts.ItemSlug) {
		return m.Opts.ItemSlug
	}
	if slug := stringops.Slugify(m.Opts.Title); ValidChannel(slug) {
		return slug
	}
	return "community"
}

func (m *ModuleChat) Render(params map[string]map[string]string, loggedIn bool) string {
	b := element.NewBuilder()
	b.DivClass("ch-module-wrapper ch-" + m.Opts.ModuleType).R(
		b.Wrap(func() {
			RenderWidget(b, WidgetCfg{
				Channel: m.channel(),
				Title:   m.Opts.Title,
				Compact: false,
			})
		}),
	)
	return b.String()
}
