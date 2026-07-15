package chat

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/apitoken"
	"github.com/rohanthewiz/church/resource/apiv1"
	"github.com/rohanthewiz/rweb"
)

// Mobile JSON API for chat (/api/v1/chat/*), consumed by church_mobile.
// Same MessageAPI DTO as the web widget — one contract for both clients.
//
// Reads are public (matching the web widget: a placed chat is visible like
// article comments); posting and moderation ride the Bearer guard
// (apitoken.APIGuard, wired in the router). For live updates the app can
// either use the same SSE endpoint the web widget uses (/chat/stream — it is
// auth-free) or poll the list endpoint with after_id.

// APIChatMessagesRWeb handles GET /api/v1/chat/messages?channel&after_id&limit.
// 200 → {"messages": [...oldest→newest...], "channel", "limit", "has_more"}.
// has_more follows the API's limit+1 probe convention; with after_id it means
// "more newer messages exist", so the app keeps paging forward.
func APIChatMessagesRWeb(ctx rweb.Context) error {
	channel := ctx.Request().QueryParam("channel")
	if !ValidChannel(channel) {
		return apiv1.Error(ctx, http.StatusBadRequest, "invalid channel")
	}
	limit, _ := apiv1.ParseLimitOffset(ctx, 50, 200) // offset unused: chat pages by after_id
	afterId, _ := strconv.ParseInt(ctx.Request().QueryParam("after_id"), 10, 64)

	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not load messages")
	}
	// limit+1 probe answers has_more without a COUNT(*) query
	msgs, err := RecentMessages(dbH, channel, afterId, limit+1)
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not load messages")
	}

	hasMore := false
	if len(msgs) > limit {
		hasMore = true
		if afterId > 0 {
			msgs = msgs[:limit] // forward paging: drop the newest probe row
		} else {
			msgs = msgs[1:] // initial window: drop the OLDEST (probe) row
		}
	}
	out := make([]MessageAPI, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toAPI(m))
	}
	return ctx.WriteJSON(map[string]any{
		"channel":  channel,
		"messages": out,
		"limit":    limit,
		"has_more": hasMore,
	})
}

// chatPostRequest is the POST /api/v1/chat/messages body.
type chatPostRequest struct {
	Channel string `json:"channel"`
	Body    string `json:"body"`
}

// APIChatPostRWeb handles POST /api/v1/chat/messages (Bearer-guarded).
// JSON body {"channel", "body"}; form values accepted as a curl-friendly
// fallback (same convention as the login endpoint).
// 201 → {"message": {...}} / 422 carries the moderation reason.
func APIChatPostRWeb(ctx rweb.Context) error {
	tu, ok := apitoken.CurrentUser(ctx)
	if !ok { // only reachable if routed without APIGuard — a wiring bug
		return apiv1.Error(ctx, http.StatusUnauthorized, "Authentication required")
	}

	var req chatPostRequest
	if body := ctx.Request().Body(); len(body) > 0 {
		_ = json.Unmarshal(body, &req)
	}
	if req.Channel == "" && req.Body == "" {
		req.Channel = ctx.Request().FormValue("channel")
		req.Body = ctx.Request().FormValue("body")
	}
	if !ValidChannel(req.Channel) {
		return apiv1.Error(ctx, http.StatusBadRequest, "invalid channel")
	}

	cleaned, reason := Moderate(tu.UserID, tu.Username, req.Body)
	if reason != "" {
		return apiv1.Error(ctx, http.StatusUnprocessableEntity, reason)
	}

	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not post message")
	}
	name := strings.TrimSpace(strings.TrimSpace(tu.FirstName) + " " + strings.TrimSpace(tu.LastName))
	if name == "" {
		name = tu.Username
	}
	msg, err := InsertMessage(dbH, Message{
		Channel:     req.Channel,
		UserId:      tu.UserID,
		Username:    tu.Username,
		DisplayName: name,
		Body:        cleaned,
	})
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not post message")
	}

	broadcastMessage(msg) // web and mobile listeners share the hubs
	return ctx.Status(http.StatusCreated).WriteJSON(map[string]any{"message": toAPI(msg)})
}

// requireAPIModerator enforces editor-or-above for the Bearer identity.
// ok=false means the denial response has already been written and the
// handler must return resp immediately. (The explicit flag matters: the
// error-writing helpers return nil on a successfully WRITTEN denial, so a
// `!= nil` check on their result would fall straight through the guard.)
func requireAPIModerator(ctx rweb.Context) (tu apitoken.TokenUser, ok bool, resp error) {
	tu, found := apitoken.CurrentUser(ctx)
	if !found {
		return tu, false, apiv1.Error(ctx, http.StatusUnauthorized, "Authentication required")
	}
	if !CanModerate(tu.Role) {
		return tu, false, apiv1.Error(ctx, http.StatusForbidden, "Editor role required")
	}
	return tu, true, nil
}

// APIChatKeepRWeb handles POST /api/v1/chat/messages/:id/keep
// (Bearer-guarded, editor+). Body {"keep": bool}; missing body pins (true).
func APIChatKeepRWeb(ctx rweb.Context) error {
	_, ok, resp := requireAPIModerator(ctx)
	if !ok {
		return resp
	}
	id, err := strconv.ParseInt(ctx.Request().Param("id"), 10, 64)
	if err != nil {
		return apiv1.Error(ctx, http.StatusBadRequest, "message id must be an integer")
	}

	keep := true
	var req struct {
		Keep *bool `json:"keep"`
	}
	if body := ctx.Request().Body(); len(body) > 0 {
		if json.Unmarshal(body, &req) == nil && req.Keep != nil {
			keep = *req.Keep
		}
	}

	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not update message")
	}
	msg, found, err := GetMessage(dbH, id)
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not update message")
	}
	if !found {
		return apiv1.Error(ctx, http.StatusNotFound, "Message not found")
	}
	if err = SetKeep(dbH, id, keep); err != nil {
		return apiv1.ServerError(ctx, err, "Could not update message")
	}
	broadcastKeep(msg.Channel, id, keep)
	return ctx.WriteJSON(map[string]any{"ok": true, "id": id, "keep": keep})
}

// APIChatDeleteRWeb handles DELETE /api/v1/chat/messages/:id
// (Bearer-guarded, editor+). Idempotent like the web delete.
func APIChatDeleteRWeb(ctx rweb.Context) error {
	_, ok, resp := requireAPIModerator(ctx)
	if !ok {
		return resp
	}
	id, err := strconv.ParseInt(ctx.Request().Param("id"), 10, 64)
	if err != nil {
		return apiv1.Error(ctx, http.StatusBadRequest, "message id must be an integer")
	}

	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not delete message")
	}
	msg, found, err := GetMessage(dbH, id)
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not delete message")
	}
	if !found {
		return ctx.WriteJSON(map[string]any{"ok": true, "id": id})
	}
	if err = DeleteMessage(dbH, id); err != nil {
		return apiv1.ServerError(ctx, err, "Could not delete message")
	}
	broadcastDelete(msg.Channel, id)
	return ctx.WriteJSON(map[string]any{"ok": true, "id": id})
}
