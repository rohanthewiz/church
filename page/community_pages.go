package page

// Prebuilt community pages: the Prayer Wall and a top-level Community Chat.
// Prebuilt (like ArticleShow etc.) so the features work out of the box at
// /prayer-wall and /community-chat; admins can additionally place the same
// modules on any dynamic page via the page builder.

import (
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/chat"
	"github.com/rohanthewiz/church/resource/prayerwall"
	"github.com/rohanthewiz/church/util/stringops"
)

// PrayerWall is the prebuilt wall page: the prayer_wall module (which itself
// embeds a live chat discussion strip at its bottom) as the main content.
func PrayerWall() (*Page, error) {
	const title = "Prayer Wall"
	pgdef := Presenter{
		Title:              title,
		Slug:               stringops.Slugify(title),
		AvailablePositions: []string{"center"},
	}
	modPres := module.Presenter{
		Opts: module.Opts{
			ModuleType:   prayerwall.ModuleTypePrayerWall,
			Title:        title,
			Published:    true,
			IsMainModule: true,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return pageFromPresenter(pgdef), nil
}

// CommunityChat is the prebuilt top-level chat page — the chat module in its
// standalone role, on the site-wide "community" channel.
func CommunityChat() (*Page, error) {
	const title = "Community Chat"
	pgdef := Presenter{
		Title:              title,
		Slug:               stringops.Slugify(title),
		AvailablePositions: []string{"center"},
	}
	modPres := module.Presenter{
		Opts: module.Opts{
			ModuleType:   chat.ModuleTypeChat,
			Title:        title,
			ItemSlug:     "community", // the channel key
			Published:    true,
			IsMainModule: true,
		},
	}
	pgdef.Modules = []module.Presenter{modPres}
	return pageFromPresenter(pgdef), nil
}
