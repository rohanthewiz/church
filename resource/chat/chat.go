// Package chat implements live, channel-scoped discussion for logged-in
// members. The same machinery serves three placements:
//
//   - a top-level chat module on any page (ModuleTypeChat)
//   - an embedded "live discussion" strip under another module
//     (ModuleTypeChatDiscussion) — e.g. beneath the Prayer Wall
//   - per-article comments (the discussion module with an "article-<id>"
//     channel, wired in page.ArticleShow)
//
// Everything is keyed by a channel string, so a placement is nothing more
// than a channel name. Messages are ephemeral: a background sweep deletes
// anything older than RetentionTTL unless an editor-or-above marked it keep.
//
// Delivery model (both web and mobile share the same primitives):
//
//	POST /chat/messages ──► moderation ──► INSERT ──► SSE broadcast to channel hub
//	                                                      │
//	web widget  ◄── EventSource /chat/stream?channel=X ◄──┤
//	mobile app  ◄── same SSE endpoint, or after_id polling on the JSON list
//
// Data access is hand-written SQL over db.Executor (no SQLBoiler model) —
// chat_messages postdates the generated models; same precedent as api_tokens
// and event_recurrences.
package chat

import (
	"regexp"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// RetentionTTL is how long an unkept chat message lives. One day: chat is a
// live conversation, not a record — anything worth preserving gets pinned by
// an editor (keep = true) and is then exempt from the sweep.
const RetentionTTL = 24 * time.Hour

// sweepInterval is how often the retention sweep runs. Much shorter than the
// TTL so messages expire close to their 24-hour mark rather than in daily
// cliffs.
const sweepInterval = 15 * time.Minute

// Message is the chat resource's own row type (hand-written access — no
// SQLBoiler model exists for chat_messages).
type Message struct {
	Id          int64
	Channel     string
	UserId      int64
	Username    string
	DisplayName string
	Body        string
	Keep        bool
	CreatedAt   time.Time
}

// channelPattern constrains channel keys to slug-safe strings. Channels are
// client-supplied on every list/post/stream call, so this is the guard that
// keeps junk keys (and anything path- or log-hostile) out of the table and
// the hub registry.
var channelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ValidChannel reports whether a client-supplied channel key is acceptable.
func ValidChannel(channel string) bool {
	return channelPattern.MatchString(channel)
}

// CanModerate reports whether a role may pin (keep) and delete messages.
// The role scale is inverted (lower = more privileged: Admin 1, Publisher 5,
// Author/Editor 7, RegisteredUser 9) EXCEPT SuperAdmin at 99, so a simple
// <= comparison would wrongly exclude SuperAdmin — hence the explicit check.
// Zero (no role loaded) never moderates.
func CanModerate(role int) bool {
	if role == user.Roles.SuperAdmin {
		return true
	}
	return role >= user.Roles.Admin && role <= user.Roles.Author
}

// StartRetentionSweep launches the background goroutine that enforces
// RetentionTTL. Mirrors idrive.StartCacheCleanup: started once from
// ServeRWeb, runs for the life of the process. An immediate first sweep
// clears any backlog from downtime before the ticker cadence takes over.
func StartRetentionSweep() {
	go func() {
		sweep()
		ticker := time.NewTicker(sweepInterval)
		defer ticker.Stop()
		for range ticker.C {
			sweep()
		}
	}()
	logger.Info("Chat retention sweep started", "ttl", RetentionTTL.String(), "interval", sweepInterval.String())
}

// sweep deletes expired, unkept messages. Errors are logged, never fatal —
// a failed sweep just means the next tick retries.
func sweep() {
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(serr.Wrap(err, "chat sweep: could not obtain DB handle"))
		return
	}
	n, err := DeleteExpired(dbH, time.Now().Add(-RetentionTTL))
	if err != nil {
		logger.LogErr(err, "chat sweep failed")
		return
	}
	if n > 0 {
		logger.Info("Chat retention sweep", "deleted", n)
	}
}
