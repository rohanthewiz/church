package article_controller

import (
	"bytes"
	"errors"
	"strings"

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/article"
	"github.com/rohanthewiz/church/resource/chimage"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

func NewArticleRWeb(ctx rweb.Context) error {
	pg, err := page.ArticleForm()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{}, app.IsLoggedInRWeb(ctx))
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
	csrf := ctx.Request().FormValue("csrf")
	if !app.VerifyFormToken(csrf) { // check that csrf is present and valid in Redis
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		return err
	}
	artPres := article.Presenter{}
	artPres.Id = ctx.Request().FormValue("article_id")
	artPres.Title = ctx.Request().FormValue("article_title")

	summary := ctx.Request().FormValue("article_summary")
	str, err := chimage.ProcessInlineImages(summary)
	if err != nil {
		logger.LogErr(err, "Error processing summary inline image", "article_id", artPres.Id, "article_title", artPres.Title)
		artPres.Summary = summary
	} else {
		artPres.Summary = str
	}

	body := ctx.Request().FormValue("article_body")
	str, err = chimage.ProcessInlineImages(body)
	if err != nil {
		logger.LogErr(err, "Error processing body inline image", "article_id", artPres.Id, "article_title", artPres.Title)
		artPres.Body = body
	} else {
		artPres.Body = str
	}

	artPres.Categories = strings.Split(ctx.Request().FormValue("categories"), ",")
	
	// Get username from session
	sess, err := cctx.GetSessionFromRWeb(ctx)
	if err == nil && sess != nil {
		artPres.UpdatedBy = sess.Username
	}
	
	if ctx.Request().FormValue("published") == "on" {
		artPres.Published = true
	}

	err = artPres.UpsertArticle()
	if err != nil {
		return err
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
	return app.RedirectRWeb(ctx, redirectTo, "Article "+msg)
}

func DeleteArticleRWeb(ctx rweb.Context) error {
	err := article.DeleteArticleById(ctx.Request().PathParam("id"))
	msg := "Article with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete article with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, "when", "deleting article")
	}
	return app.RedirectRWeb(ctx, "/admin/articles", msg)
}