package article_controller

import (
	"bytes"
	"strings"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/core/formdraft"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/article"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/resource/chimage"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/church/util/inputerr"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

func NewArticleRWeb(ctx rweb.Context) error {
	pg, err := page.ArticleForm()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		"_global": {"user_agent": ctx.UserAgent(), "username": cctx.GetUsernameFromRWeb(ctx),
			// What the viewer may do, so admin modules offer only permitted actions
			authz.ParamKey: authz.ParamValue(ctx),
			// Typed values from a refused save of this form, if any
			formdraft.ParamKey: base.TakeFormDraft(pg, ctx)},
	}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

// Show a particular article - for given by id
func ShowArticleRWeb(ctx rweb.Context) error {
	pg, err := page.ArticleShow()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageSingleRWeb(pg, ctx)))
}

func ListArticlesRWeb(ctx rweb.Context) error {
	pg, err := page.ArticlesList()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageListRWeb(pg, ctx)))
}

func AdminListArticlesRWeb(ctx rweb.Context) error {
	pg, err := page.AdminArticlesList()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageListRWeb(pg, ctx)))
}

func EditArticleRWeb(ctx rweb.Context) error {
	pg, err := page.ArticleForm()
	if err != nil {
		return err
	}
	cctx.SetFormReferrerRWeb(ctx) // save the referrer calling for edit
	return ctx.WriteHTML(string(base.RenderPageSingleRWeb(pg, ctx)))
}

func UpsertArticleRWeb(ctx rweb.Context) error {
	// Every refusal returns to the form with what was typed (core/formdraft),
	// so the form values are read first. Reading them has no side effects;
	// inline images are only stored after the token check below.
	id := strings.TrimSpace(ctx.Request().FormValue("article_id"))
	formURL := "/admin/articles/new"
	if id != "" && id != "0" {
		formURL = "/admin/articles/edit/" + id
	}
	artPres := article.Presenter{}
	artPres.Id = ctx.Request().FormValue("article_id")
	artPres.Title = ctx.Request().FormValue("article_title")
	summary := ctx.Request().FormValue("article_summary")
	body := ctx.Request().FormValue("article_body")
	artPres.Summary, artPres.Body = summary, body
	artPres.Categories = strings.Split(ctx.Request().FormValue("categories"), ",")
	if ctx.Request().FormValue("published") == "on" {
		artPres.Published = true
	}
	// refuse sends the admin back to the form, keeping what they typed
	refuse := func(msg string) error {
		formdraft.Save(ctx, formURL, artPres)
		return app.RedirectRWebError(ctx, formURL, msg)
	}

	// An expired token is routine (a form left open too long), so send the admin
	// back to the same form with a warning rather than a bare 500. Nothing has
	// been written yet, and the draft keeps their edits.
	csrf := ctx.Request().FormValue("csrf")
	if !app.VerifyFormToken(csrf) { // check that csrf is present and valid in the in-process kvstore
		formdraft.Save(ctx, formURL, artPres)
		return app.RedirectRWebWarn(ctx, formURL,
			"Your form has expired and was not saved. Your changes are still in the form; please save again.")
	}

	str, err := chimage.ProcessInlineImages(summary)
	if err != nil {
		logger.LogErr(err, "Error processing summary inline image", "article_id", artPres.Id, "article_title", artPres.Title)
		artPres.Summary = summary
	} else {
		artPres.Summary = str
	}

	str, err = chimage.ProcessInlineImages(body)
	if err != nil {
		logger.LogErr(err, "Error processing body inline image", "article_id", artPres.Id, "article_title", artPres.Title)
		artPres.Body = body
	} else {
		artPres.Body = str
	}

	// Get username from session
	sess, err := cctx.GetSessionFromRWeb(ctx)
	if err == nil && sess != nil {
		artPres.UpdatedBy = sess.Username
	}

	// Failures go back to the form as an error flash. UpsertArticle is a single
	// Insert or Update, so a failed save wrote nothing and the form (not the
	// list) is the right place to retry from.
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return refuse("The article could not be saved: the database is unavailable.")
	}

	// Publishing is its own permission, resolved field by field rather than by
	// refusing the save: someone who may edit but not publish can still save
	// their edits, and the flag keeps its stored value (false on create). See
	// authz.ResolveFlag.
	actor, ok := authz.ActorFrom(ctx)
	if !ok {
		// Admin routes always run behind AdminGuardRWeb, so no actor means a
		// wiring bug. Fail closed rather than publish unchecked.
		return refuse("Your permissions could not be confirmed. The article was not saved.")
	}
	submittedFlag := artPres.Published
	artPres.Published, err = authz.ResolveFlag(dbH, actor, authz.ArticlesPublish, authz.FlagArticlePublished, artPres.Id, submittedFlag)
	if err != nil {
		logger.LogErr(err, "Error resolving article published flag", "article_id", artPres.Id)
		return refuse("Error saving the article. It was not saved.")
	}
	// The form renders the switch disabled (with the stored value in a hidden
	// field) for anyone lacking the permission, so a difference here means a
	// stale form or a hand-built post. Say what happened instead of silently
	// ignoring the box.
	flagNote := ""
	if artPres.Published != submittedFlag {
		flagNote = " Publishing needs the articles.publish permission, so the published setting was left as it was."
	}
	err = artPres.UpsertArticle(dbH)
	if err != nil {
		if msg, isInput := inputerr.UserMessage(err); isInput {
			// The admin's mistake, not ours: no error log, just the reason
			return refuse(msg + ". The article was not saved.")
		}
		logger.LogErr(err, "Error in article upsert", "article_id", artPres.Id, "article_title", artPres.Title)
		return refuse("Error saving the article. It was not saved.")
	}
	msg := "Created"
	if artPres.Id != "0" && artPres.Id != "" {
		msg = "Updated"
	}

	redirectTo := "/admin/articles"
	sess, _ = cctx.GetSessionFromRWeb(ctx)
	if sess != nil && sess.FormReferrer != "" {
		redirectTo = sess.FormReferrer // return to the form caller
	}
	return app.RedirectRWeb(ctx, redirectTo, "Article "+msg+flagNote)
}

func DeleteArticleRWeb(ctx rweb.Context) error {
	// POST + token: the route rejects GET, and the token ties the request to a
	// page we actually rendered (see grid CSRFToken / app.VerifyFormTokenRWeb).
	if ok, err := app.VerifyFormTokenRWeb(ctx, "/admin/articles"); !ok {
		return err
	}
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Could not obtain DB handle")
		return app.RedirectRWeb(ctx, "/admin/articles", "Error deleting article")
	}
	err = article.DeleteArticleById(dbH, ctx.Request().PathParam("id"))
	msg := "Article with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete article with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, "when", "deleting article")
	}
	return app.RedirectRWeb(ctx, "/admin/articles", msg)
}
