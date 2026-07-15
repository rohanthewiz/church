package prayerwall

import (
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/app"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/chat"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

// Web handlers for the prayer wall — classic form posts with the site's CSRF
// token and flash-message redirects (matching articles/events), since the
// wall is server-rendered. Routes live under /prayer-requests wrapped in
// UseCustomContextRWeb.

// wallURL is where form posts land the user afterward — the referring page
// when known (the wall module may be placed on any page), else the prebuilt
// wall page.
func wallURL(ctx rweb.Context) string {
	if ref := ctx.Request().Header("Referer"); ref != "" {
		return ref
	}
	return "/prayer-wall"
}

// webIdentity resolves the session to a full AuthUser; loggedIn=false with
// no error is the anonymous case.
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

// PostRequestRWeb handles POST /prayer-requests — a member sharing a request.
func PostRequestRWeb(ctx rweb.Context) error {
	if !app.VerifyFormToken(ctx.Request().FormValue("csrf")) {
		return app.RedirectRWeb(ctx, wallURL(ctx),
			"Your form has expired. Please refresh the page and try again")
	}
	au, loggedIn, err := webIdentity(ctx)
	if err != nil {
		logger.LogErr(err, "prayer wall: could not resolve identity")
		return app.RedirectRWeb(ctx, wallURL(ctx), "Could not post your request")
	}
	if !loggedIn {
		return app.RedirectRWeb(ctx, "/login", "Please log in to share a prayer request")
	}

	title, body, reason := Validate(ctx.Request().FormValue("title"), ctx.Request().FormValue("body"))
	// The wall shares chat's language policy — one deny list for the site.
	if reason == "" && (chat.ContainsBannedWord(title) || chat.ContainsBannedWord(body)) {
		reason = "Your request contains language that isn't allowed here"
	}
	if reason != "" {
		return app.RedirectRWeb(ctx, wallURL(ctx), reason)
	}

	name := strings.TrimSpace(strings.TrimSpace(au.FirstName) + " " + strings.TrimSpace(au.LastName))
	if name == "" {
		name = au.Username
	}
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "prayer wall: could not obtain DB handle")
		return app.RedirectRWeb(ctx, wallURL(ctx), "Could not post your request")
	}
	if _, err = InsertRequest(dbH, Request{
		UserId:      au.ID,
		Username:    au.Username,
		DisplayName: name,
		Title:       title,
		Body:        body,
	}); err != nil {
		logger.LogErr(err, "prayer wall: could not insert request")
		return app.RedirectRWeb(ctx, wallURL(ctx), "Could not post your request")
	}
	return app.RedirectRWeb(ctx, wallURL(ctx), "Your prayer request has been posted")
}

// MarkAnsweredRWeb handles POST /prayer-requests/answered/:id (editor+).
// Form fields: answered (true|false), note (optional praise report).
func MarkAnsweredRWeb(ctx rweb.Context) error {
	if !app.VerifyFormToken(ctx.Request().FormValue("csrf")) {
		return app.RedirectRWeb(ctx, wallURL(ctx),
			"Your form has expired. Please refresh the page and try again")
	}
	au, loggedIn, err := webIdentity(ctx)
	if err != nil {
		logger.LogErr(err, "prayer wall: could not resolve identity")
		return app.RedirectRWeb(ctx, wallURL(ctx), "Could not update the request")
	}
	if !loggedIn || !chat.CanModerate(au.Role) {
		return app.RedirectRWeb(ctx, wallURL(ctx), "Editor role required")
	}

	id, err := strconv.ParseInt(ctx.Request().Param("id"), 10, 64)
	if err != nil {
		return app.RedirectRWeb(ctx, wallURL(ctx), "Invalid request id")
	}
	answered := ctx.Request().FormValue("answered") != "false"

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "prayer wall: could not obtain DB handle")
		return app.RedirectRWeb(ctx, wallURL(ctx), "Could not update the request")
	}
	if err = SetAnswered(dbH, id, answered, ctx.Request().FormValue("note")); err != nil {
		logger.LogErr(err, "prayer wall: could not set answered")
		return app.RedirectRWeb(ctx, wallURL(ctx), "Could not update the request")
	}
	msg := "Request marked answered — praise God!"
	if !answered {
		msg = "Request reopened"
	}
	logger.Info("Prayer request answered toggled", "id", ctx.Request().Param("id"), "by", au.Username)
	return app.RedirectRWeb(ctx, wallURL(ctx), msg)
}

// DeleteRequestRWeb handles POST /prayer-requests/delete/:id.
// Editors remove any request; a member may withdraw their own.
func DeleteRequestRWeb(ctx rweb.Context) error {
	if !app.VerifyFormToken(ctx.Request().FormValue("csrf")) {
		return app.RedirectRWeb(ctx, wallURL(ctx),
			"Your form has expired. Please refresh the page and try again")
	}
	au, loggedIn, err := webIdentity(ctx)
	if err != nil {
		logger.LogErr(err, "prayer wall: could not resolve identity")
		return app.RedirectRWeb(ctx, wallURL(ctx), "Could not remove the request")
	}
	if !loggedIn {
		return app.RedirectRWeb(ctx, "/login", "Please log in")
	}

	id, err := strconv.ParseInt(ctx.Request().Param("id"), 10, 64)
	if err != nil {
		return app.RedirectRWeb(ctx, wallURL(ctx), "Invalid request id")
	}

	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "prayer wall: could not obtain DB handle")
		return app.RedirectRWeb(ctx, wallURL(ctx), "Could not remove the request")
	}
	req, found, err := GetRequest(dbH, id)
	if err != nil {
		logger.LogErr(err, "prayer wall: could not load request for delete")
		return app.RedirectRWeb(ctx, wallURL(ctx), "Could not remove the request")
	}
	if !found { // already gone — idempotent
		return app.RedirectRWeb(ctx, wallURL(ctx), "Request removed")
	}
	// Owner-or-editor: the ownership check is by user id, not username, so a
	// renamed account can still withdraw its older requests.
	if !chat.CanModerate(au.Role) && au.ID != req.UserId {
		return app.RedirectRWeb(ctx, wallURL(ctx), "You may only withdraw your own request")
	}
	if err = DeleteRequest(dbH, id); err != nil {
		logger.LogErr(err, "prayer wall: could not delete request")
		return app.RedirectRWeb(ctx, wallURL(ctx), "Could not remove the request")
	}
	logger.Info("Prayer request removed", "id", ctx.Request().Param("id"), "by", au.Username)
	return app.RedirectRWeb(ctx, wallURL(ctx), "Request removed")
}
