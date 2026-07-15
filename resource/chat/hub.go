package chat

import (
	"sync"
	"time"

	"github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/rweb"
)

// One SSEHub per channel: rweb's hub fans out to every registered client, so
// scoping a hub to a channel gives channel isolation for free — an article's
// comment stream never wakes prayer-wall listeners. Hubs are created lazily
// on first use and live for the process (a hub with zero clients is just a
// map entry and an idle heartbeat goroutine — negligible at church scale,
// where the set of channels is small and stable).
var hubs = struct {
	sync.Mutex
	m map[string]*rweb.SSEHub
}{m: map[string]*rweb.SSEHub{}}

// HubFor returns (creating if needed) the SSE hub for a channel. Callers
// must have validated the channel first (ValidChannel) so junk keys can't
// grow the registry unboundedly.
func HubFor(channel string) *rweb.SSEHub {
	hubs.Lock()
	defer hubs.Unlock()
	if h, ok := hubs.m[channel]; ok {
		return h
	}
	h := rweb.NewSSEHub(rweb.SSEHubOptions{
		ChannelSize: 16,
		// Heartbeat keeps proxies/LBs from killing quiet channels' connections
		// (typical idle kill is 30-60s).
		HeartbeatInterval: 25 * time.Second,
	})
	hubs.m[channel] = h
	return h
}

// MessageAPI is the one JSON shape for a chat message, shared by the web
// widget and the mobile /api/v1 endpoints so both clients parse identically
// (snake_case keys, stable once shipped — same contract discipline as the
// other API DTOs). It also rides the SSE stream as the event payload.
type MessageAPI struct {
	Id          int64  `json:"id"`
	Channel     string `json:"channel"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Body        string `json:"body"`
	Keep        bool   `json:"keep"`
	CreatedAt   string `json:"created_at"`
}

func toAPI(m Message) MessageAPI {
	return MessageAPI{
		Id:          m.Id,
		Channel:     m.Channel,
		Username:    m.Username,
		DisplayName: m.DisplayName,
		Body:        m.Body,
		Keep:        m.Keep,
		CreatedAt:   m.CreatedAt.Format(timeutil.ISO8601DateTime),
	}
}

// SSE event types carried inside the hub's {"type": ..., "data": ...}
// envelope (rweb.SSEHub.Broadcast JSON-wraps and sends as a standard
// "message" event, so clients need only onmessage + JSON.parse).
const (
	evtMessage = "chat_message" // data: MessageAPI
	evtDelete  = "chat_delete"  // data: {"id": n} — moderator removed a message
	evtKeep    = "chat_keep"    // data: {"id": n, "keep": bool} — pin toggled
)

// broadcastMessage pushes a freshly stored message to the channel's live
// listeners.
func broadcastMessage(m Message) {
	HubFor(m.Channel).BroadcastAny(evtMessage, toAPI(m))
}

// broadcastDelete tells listeners to drop a message from their view.
func broadcastDelete(channel string, id int64) {
	HubFor(channel).BroadcastAny(evtDelete, map[string]int64{"id": id})
}

// broadcastKeep tells listeners a message's pinned state changed.
func broadcastKeep(channel string, id int64, keep bool) {
	HubFor(channel).BroadcastAny(evtKeep, map[string]any{"id": id, "keep": keep})
}

// StreamHandler returns the SSE endpoint handler (GET /chat/stream?channel=X).
// It needs the *rweb.Server (the hub handler wires per-connection cleanup
// through it), which only the router has — hence a constructor instead of a
// plain handler func.
//
// Streaming is open to anonymous visitors on purpose: reads of a placed chat
// are public (like article comments); only posting requires login.
func StreamHandler(s *rweb.Server) rweb.Handler {
	return func(ctx rweb.Context) error {
		channel := ctx.Request().QueryParam("channel")
		if !ValidChannel(channel) {
			return ctx.Status(400).WriteJSON(map[string]string{"error": "invalid channel"})
		}
		return HubFor(channel).Handler(s)(ctx)
	}
}
