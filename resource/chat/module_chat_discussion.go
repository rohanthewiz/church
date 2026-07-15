package chat

import (
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
)

const ModuleTypeChatDiscussion = "chat_discussion"

// ModuleChatDiscussion is the embeddable "live discussion" strip: a compact
// chat placed at the bottom of another module's page — comments under an
// article, discussion under the Prayer Wall, etc.
//
// Channel derivation is what distinguishes it from the top-level module.
// Opts.ItemSlug acts as the channel PREFIX (e.g. "article"); when the page
// renders a specific item, the item's id (published by the single-item
// controllers into the _global params — see basectlr.RenderPageSingleRWeb)
// is appended, yielding a per-item conversation like "article-42". With no
// item id in play the prefix alone is the channel, which is exactly right
// for singleton placements like the prayer wall.
type ModuleChatDiscussion struct {
	module.Presenter
}

func NewModuleChatDiscussion(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleChatDiscussion)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

func (m *ModuleChatDiscussion) Render(params map[string]map[string]string, loggedIn bool) string {
	prefix := m.Opts.ItemSlug
	if prefix == "" {
		prefix = "discussion"
	}

	channel := prefix
	if glob, ok := params["_global"]; ok {
		if itemId := glob["item_id"]; itemId != "" {
			channel = prefix + "-" + itemId
		}
	}
	if !ValidChannel(channel) {
		// A malformed id (or hostile query junk) must not become a channel;
		// skip rendering rather than surface a broken widget.
		return ""
	}

	title := m.Opts.Title
	if title == "" {
		title = "Live Discussion"
	}

	b := element.NewBuilder()
	b.DivClass("ch-module-wrapper ch-" + m.Opts.ModuleType).R(
		b.Wrap(func() {
			RenderWidget(b, WidgetCfg{
				Channel: channel,
				Title:   title,
				Compact: true,
			})
		}),
	)
	return b.String()
}
