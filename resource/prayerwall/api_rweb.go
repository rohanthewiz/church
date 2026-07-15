package prayerwall

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/apitoken"
	"github.com/rohanthewiz/church/resource/apiv1"
	"github.com/rohanthewiz/church/resource/chat"
	"github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/rweb"
)

// Mobile JSON API for the prayer wall (/api/v1/prayer-requests). Reads are
// public like the web wall; posting and moderation ride the Bearer guard.
// The wall's live discussion is plain chat — the app uses the chat API with
// channel "prayer-wall".

// RequestAPI is the public JSON DTO — a deliberate subset of the row
// (user_id stays server-side; ownership is expressed as the "mine" flag so
// the app can offer Withdraw without learning other members' ids).
type RequestAPI struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	Answered     bool   `json:"answered"`
	AnsweredNote string `json:"answered_note"`
	CreatedAt    string `json:"created_at"`
	Mine         bool   `json:"mine"`
}

func toAPI(r Request, viewerId int64) RequestAPI {
	return RequestAPI{
		ID:           r.Id,
		Username:     r.Username,
		DisplayName:  r.DisplayName,
		Title:        r.Title,
		Body:         r.Body,
		Answered:     r.Answered,
		AnsweredNote: r.AnsweredNote,
		CreatedAt:    r.CreatedAt.Format(timeutil.ISO8601DateTime),
		Mine:         viewerId != 0 && viewerId == r.UserId,
	}
}

// APIPrayerRequestsRWeb handles GET /api/v1/prayer-requests?limit&offset.
// Public. When a valid Bearer token happens to be presented anyway, the
// "mine" flags are computed for that user (the route is outside the guard,
// so the token is resolved opportunistically here).
// 200 → {"prayer_requests": [...], "limit", "offset", "has_more"}.
func APIPrayerRequestsRWeb(ctx rweb.Context) error {
	limit, offset := apiv1.ParseLimitOffset(ctx, 20, 100)

	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not load prayer requests")
	}

	var viewerId int64
	const scheme = "Bearer "
	if authz := ctx.Request().Header("Authorization"); len(authz) > len(scheme) &&
		strings.EqualFold(authz[:len(scheme)], scheme) {
		if tu, found, err := apitoken.LookupUser(dbH, strings.TrimSpace(authz[len(scheme):])); err == nil && found {
			viewerId = tu.UserID
		}
	}

	// limit+1 probe answers has_more without a COUNT(*) query
	reqs, err := ListRequests(dbH, limit+1, offset)
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not load prayer requests")
	}
	hasMore := false
	if len(reqs) > limit {
		hasMore = true
		reqs = reqs[:limit]
	}
	out := make([]RequestAPI, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, toAPI(r, viewerId))
	}
	return ctx.WriteJSON(map[string]any{
		"prayer_requests": out,
		"limit":           limit,
		"offset":          offset,
		"has_more":        hasMore,
	})
}

// prayerPostRequest is the POST /api/v1/prayer-requests body.
type prayerPostRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// APIPrayerPostRWeb handles POST /api/v1/prayer-requests (Bearer-guarded).
// 201 → {"prayer_request": {...}} / 422 carries the validation reason.
func APIPrayerPostRWeb(ctx rweb.Context) error {
	tu, ok := apitoken.CurrentUser(ctx)
	if !ok { // only reachable if routed without APIGuard — a wiring bug
		return apiv1.Error(ctx, http.StatusUnauthorized, "Authentication required")
	}

	var req prayerPostRequest
	if body := ctx.Request().Body(); len(body) > 0 {
		_ = json.Unmarshal(body, &req)
	}
	if req.Title == "" && req.Body == "" {
		req.Title = ctx.Request().FormValue("title")
		req.Body = ctx.Request().FormValue("body")
	}

	title, body, reason := Validate(req.Title, req.Body)
	if reason == "" && (chat.ContainsBannedWord(title) || chat.ContainsBannedWord(body)) {
		reason = "Your request contains language that isn't allowed here"
	}
	if reason != "" {
		return apiv1.Error(ctx, http.StatusUnprocessableEntity, reason)
	}

	name := strings.TrimSpace(strings.TrimSpace(tu.FirstName) + " " + strings.TrimSpace(tu.LastName))
	if name == "" {
		name = tu.Username
	}
	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not post prayer request")
	}
	stored, err := InsertRequest(dbH, Request{
		UserId:      tu.UserID,
		Username:    tu.Username,
		DisplayName: name,
		Title:       title,
		Body:        body,
	})
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not post prayer request")
	}
	return ctx.Status(http.StatusCreated).WriteJSON(map[string]any{"prayer_request": toAPI(stored, tu.UserID)})
}

// APIPrayerAnsweredRWeb handles POST /api/v1/prayer-requests/:id/answered
// (Bearer-guarded, editor+). Body {"answered": bool, "note": string};
// missing body marks answered with no note.
func APIPrayerAnsweredRWeb(ctx rweb.Context) error {
	tu, ok := apitoken.CurrentUser(ctx)
	if !ok {
		return apiv1.Error(ctx, http.StatusUnauthorized, "Authentication required")
	}
	if !chat.CanModerate(tu.Role) {
		return apiv1.Error(ctx, http.StatusForbidden, "Editor role required")
	}
	id, err := strconv.ParseInt(ctx.Request().Param("id"), 10, 64)
	if err != nil {
		return apiv1.Error(ctx, http.StatusBadRequest, "request id must be an integer")
	}

	answered, note := true, ""
	var req struct {
		Answered *bool  `json:"answered"`
		Note     string `json:"note"`
	}
	if body := ctx.Request().Body(); len(body) > 0 && json.Unmarshal(body, &req) == nil {
		if req.Answered != nil {
			answered = *req.Answered
		}
		note = req.Note
	}

	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not update prayer request")
	}
	if _, found, err := GetRequest(dbH, id); err != nil {
		return apiv1.ServerError(ctx, err, "Could not update prayer request")
	} else if !found {
		return apiv1.Error(ctx, http.StatusNotFound, "Prayer request not found")
	}
	if err = SetAnswered(dbH, id, answered, note); err != nil {
		return apiv1.ServerError(ctx, err, "Could not update prayer request")
	}
	return ctx.WriteJSON(map[string]any{"ok": true, "id": id, "answered": answered})
}

// APIPrayerDeleteRWeb handles DELETE /api/v1/prayer-requests/:id
// (Bearer-guarded). Editors remove any request; a member may withdraw their
// own. Idempotent.
func APIPrayerDeleteRWeb(ctx rweb.Context) error {
	tu, ok := apitoken.CurrentUser(ctx)
	if !ok {
		return apiv1.Error(ctx, http.StatusUnauthorized, "Authentication required")
	}
	id, err := strconv.ParseInt(ctx.Request().Param("id"), 10, 64)
	if err != nil {
		return apiv1.Error(ctx, http.StatusBadRequest, "request id must be an integer")
	}

	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not remove prayer request")
	}
	req, found, err := GetRequest(dbH, id)
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not remove prayer request")
	}
	if !found {
		return ctx.WriteJSON(map[string]any{"ok": true, "id": id})
	}
	if !chat.CanModerate(tu.Role) && tu.UserID != req.UserId {
		return apiv1.Error(ctx, http.StatusForbidden, "You may only withdraw your own request")
	}
	if err = DeleteRequest(dbH, id); err != nil {
		return apiv1.ServerError(ctx, err, "Could not remove prayer request")
	}
	return ctx.WriteJSON(map[string]any{"ok": true, "id": id})
}
