package chat

import (
	"net/http"
	"strconv"
	"strings"

	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

// Web-side JSON handlers for the chat widget (session-cookie auth, routes
// under /chat wrapped in UseCustomContextRWeb). The widget is JS-driven, so
// unlike the rest of the web UI these answer JSON, not HTML — same shapes as
// the mobile API so there is exactly one message contract.

// webError mirrors apiv1.Error's {"error": msg} shape without importing
// apiv1 (that package is scoped to /api/v1; these are web routes).
func webError(ctx rweb.Context, status int, msg string) error {
	return ctx.Status(status).WriteJSON(map[string]string{"error": msg})
}

// webIdentity resolves the session to a full AuthUser (the session stores
// only the username; chat needs id for rate limiting and role for
// moderation). loggedIn=false with no error is the anonymous case.
func webIdentity(ctx rweb.Context) (au user.AuthUser, loggedIn bool, err error) {
	sess, sessErr := cctx.GetSessionFromRWeb(ctx)
	if sessErr != nil || sess == nil || sess.Username == "" {
		return au, false, nil
	}
	dbH, err := db.Db()
	if err != nil {
		return au, false, err
	}
	au, found, err := user.AuthUserByUsername(dbH, sess.Username)
	if err != nil || !found {
		return au, false, err
	}
	return au, true, nil
}

// sameOriginOK is a lightweight CSRF gate for the widget's fetch() POSTs.
// The classic form-token flow doesn't fit a long-lived JS widget, so we rely
// on Sec-Fetch-Site — browsers set it on all requests and it cannot be
// forged cross-site; a hostile site's form/img/fetch arrives as
// "cross-site". Requests without the header (curl, old browsers) pass:
// curl has no ambient cookie to ride, which is the only thing CSRF steals.
func sameOriginOK(ctx rweb.Context) bool {
	switch ctx.Request().Header("Sec-Fetch-Site") {
	case "", "same-origin", "same-site", "none":
		return true
	}
	return false
}

func displayName(au user.AuthUser) string {
	name := strings.TrimSpace(strings.TrimSpace(au.FirstName) + " " + strings.TrimSpace(au.LastName))
	if name == "" {
		name = au.Username
	}
	return name
}

// ListMessagesRWeb handles GET /chat/messages?channel&after_id&limit.
// Public read (matching article comments being visible to visitors); the
// "me" block tells the widget what controls to draw — the server re-checks
// every privileged action anyway, so this is UI hinting, not enforcement.
// 200 → {"messages": [...], "channel": ..., "me": {"logged_in", "username", "can_moderate"}}
func ListMessagesRWeb(ctx rweb.Context) error {
	channel := ctx.Request().QueryParam("channel")
	if !ValidChannel(channel) {
		return webError(ctx, http.StatusBadRequest, "invalid channel")
	}
	afterId, _ := strconv.ParseInt(ctx.Request().QueryParam("after_id"), 10, 64)
	limit := 50
	if l, err := strconv.Atoi(ctx.Request().QueryParam("limit")); err == nil && l > 0 && l <= 200 {
		limit = l
	}

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "chat: could not obtain DB handle")
		return webError(ctx, http.StatusInternalServerError, "Could not load messages")
	}
	msgs, err := RecentMessages(dbH, channel, afterId, limit)
	if err != nil {
		logger.LogErr(err, "chat: could not load messages")
		return webError(ctx, http.StatusInternalServerError, "Could not load messages")
	}

	au, loggedIn, err := webIdentity(ctx)
	if err != nil {
		logger.LogErr(err, "chat: could not resolve identity") // non-fatal: degrade to anonymous view
	}

	out := make([]MessageAPI, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toAPI(m))
	}
	return ctx.WriteJSON(map[string]any{
		"channel":  channel,
		"messages": out,
		"me": map[string]any{
			"logged_in":    loggedIn,
			"username":     au.Username,
			"can_moderate": loggedIn && CanModerate(au.Role),
		},
	})
}

// PostMessageRWeb handles POST /chat/messages (form fields: channel, body).
// Login required; the message passes rule-based moderation before insert,
// then fans out over SSE. 201 → {"message": {...}}; 422 carries the
// moderation reason for the widget to show inline.
func PostMessageRWeb(ctx rweb.Context) error {
	if !sameOriginOK(ctx) {
		return webError(ctx, http.StatusForbidden, "Cross-site request refused")
	}
	au, loggedIn, err := webIdentity(ctx)
	if err != nil {
		logger.LogErr(err, "chat: could not resolve identity")
		return webError(ctx, http.StatusInternalServerError, "Could not post message")
	}
	if !loggedIn {
		return webError(ctx, http.StatusUnauthorized, "Please log in to join the conversation")
	}

	channel := ctx.Request().FormValue("channel")
	if !ValidChannel(channel) {
		return webError(ctx, http.StatusBadRequest, "invalid channel")
	}

	cleaned, reason := Moderate(au.ID, au.Username, ctx.Request().FormValue("body"))
	if reason != "" {
		return webError(ctx, http.StatusUnprocessableEntity, reason)
	}

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "chat: could not obtain DB handle")
		return webError(ctx, http.StatusInternalServerError, "Could not post message")
	}
	msg, err := InsertMessage(dbH, Message{
		Channel:     channel,
		UserId:      au.ID,
		Username:    au.Username,
		DisplayName: displayName(au),
		Body:        cleaned,
	})
	if err != nil {
		logger.LogErr(err, "chat: could not insert message")
		return webError(ctx, http.StatusInternalServerError, "Could not post message")
	}

	broadcastMessage(msg)
	return ctx.Status(http.StatusCreated).WriteJSON(map[string]any{"message": toAPI(msg)})
}

// requireModerator resolves the session and enforces editor-or-above.
// Shared by keep/delete below. ok=false means the denial response has
// already been written and the handler must return resp immediately. (The
// explicit flag matters: webError returns nil on a successfully WRITTEN
// denial, so a `!= nil` check on its result would fall through the guard.)
func requireModerator(ctx rweb.Context) (au user.AuthUser, ok bool, resp error) {
	if !sameOriginOK(ctx) {
		return au, false, webError(ctx, http.StatusForbidden, "Cross-site request refused")
	}
	au, loggedIn, err := webIdentity(ctx)
	if err != nil {
		logger.LogErr(err, "chat: could not resolve identity")
		return au, false, webError(ctx, http.StatusInternalServerError, "Could not verify permissions")
	}
	if !loggedIn {
		return au, false, webError(ctx, http.StatusUnauthorized, "Please log in")
	}
	if !CanModerate(au.Role) {
		return au, false, webError(ctx, http.StatusForbidden, "Editor role required")
	}
	return au, true, nil
}

// KeepMessageRWeb handles POST /chat/keep/:id (form field keep=true|false).
// Editor-or-above pins a message so the retention sweep spares it.
func KeepMessageRWeb(ctx rweb.Context) error {
	au, ok, resp := requireModerator(ctx)
	if !ok {
		return resp
	}
	id, err := strconv.ParseInt(ctx.Request().Param("id"), 10, 64)
	if err != nil {
		return webError(ctx, http.StatusBadRequest, "message id must be an integer")
	}
	keep := ctx.Request().FormValue("keep") != "false" // default true: "keep this"

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "chat: could not obtain DB handle")
		return webError(ctx, http.StatusInternalServerError, "Could not update message")
	}
	msg, found, err := GetMessage(dbH, id)
	if err != nil {
		logger.LogErr(err, "chat: could not load message for keep")
		return webError(ctx, http.StatusInternalServerError, "Could not update message")
	}
	if !found {
		return webError(ctx, http.StatusNotFound, "Message not found")
	}
	if err = SetKeep(dbH, id, keep); err != nil {
		logger.LogErr(err, "chat: could not set keep")
		return webError(ctx, http.StatusInternalServerError, "Could not update message")
	}

	logger.Info("Chat message keep toggled", "id", ctx.Request().Param("id"), "keep",
		strconv.FormatBool(keep), "by", au.Username)
	broadcastKeep(msg.Channel, id, keep)
	return ctx.WriteJSON(map[string]any{"ok": true, "id": id, "keep": keep})
}

// DeleteMessageRWeb handles POST /chat/delete/:id — moderation removal.
// POST (not DELETE) to match the site's existing web delete convention.
func DeleteMessageRWeb(ctx rweb.Context) error {
	au, ok, resp := requireModerator(ctx)
	if !ok {
		return resp
	}
	id, err := strconv.ParseInt(ctx.Request().Param("id"), 10, 64)
	if err != nil {
		return webError(ctx, http.StatusBadRequest, "message id must be an integer")
	}

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "chat: could not obtain DB handle")
		return webError(ctx, http.StatusInternalServerError, "Could not delete message")
	}
	msg, found, err := GetMessage(dbH, id)
	if err != nil {
		logger.LogErr(err, "chat: could not load message for delete")
		return webError(ctx, http.StatusInternalServerError, "Could not delete message")
	}
	if !found { // already gone — idempotent success, nothing to broadcast
		return ctx.WriteJSON(map[string]any{"ok": true, "id": id})
	}
	if err = DeleteMessage(dbH, id); err != nil {
		logger.LogErr(err, "chat: could not delete message")
		return webError(ctx, http.StatusInternalServerError, "Could not delete message")
	}

	logger.Info("Chat message deleted", "id", ctx.Request().Param("id"), "by", au.Username)
	broadcastDelete(msg.Channel, id)
	return ctx.WriteJSON(map[string]any{"ok": true, "id": id})
}
