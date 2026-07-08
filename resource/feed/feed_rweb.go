// Package feed aggregates the newest content across resources into a single
// payload for the mobile app's home screen — one request instead of three.
//
// It lives apart from apiv1 to keep the import graph acyclic:
// feed -> {sermon, article, event} -> apiv1.
package feed

import (
	"github.com/rohanthewiz/church/resource/article"
	"github.com/rohanthewiz/church/resource/event"
	"github.com/rohanthewiz/church/resource/sermon"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

// Fixed per-section sizes: the home screen shows a teaser of each section and
// links into the full paginated lists, so client-tunable sizes buy nothing.
const (
	feedArticles = 5
	feedSermons  = 5
	feedEvents   = 5
)

// GET /api/v1/feed
// Each section degrades independently — a failure in one resource returns an
// empty section rather than failing the whole home screen. Errors are logged
// server-side.
func APIFeedRWeb(ctx rweb.Context) error {
	arts, err := article.RecentArticlesAPI(feedArticles)
	if err != nil {
		logger.LogErr(err, "feed: articles section failed")
		arts = []article.ArticleAPI{}
	}
	sers, err := sermon.RecentSermonsAPI(feedSermons)
	if err != nil {
		logger.LogErr(err, "feed: sermons section failed")
		sers = []sermon.SermonAPI{}
	}
	evts, err := event.UpcomingEventsAPI(feedEvents)
	if err != nil {
		logger.LogErr(err, "feed: events section failed")
		evts = []event.EventAPI{}
	}

	return ctx.WriteJSON(map[string]any{
		"articles": arts,
		"sermons":  sers,
		"events":   evts,
	})
}
