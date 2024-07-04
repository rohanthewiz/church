package article_controller

import (
	"bytes"
	"errors"
	"strings"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	ctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/article"
	"github.com/rohanthewiz/church/resource/chimage"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/logger"
)

func NewArticle(c echo.Context) error {
	pg, err := page.ArticleForm()
	if err != nil {
		c.Error(err)
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{}, app.IsLoggedIn(c))
	c.HTMLBlob(200, buf.Bytes())
	return nil
}

// Show a particular mrticle - for given by id
func ShowArticle(c echo.Context) error {
	pg, err := page.ArticleShow()
	if err != nil {
		c.Error(err)
		return err
	}
	c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return nil
}

func ListArticles(c echo.Context) error {
	pg, err := page.ArticlesList()
	if err != nil {
		c.Error(err)
		return err
	}
	c.HTMLBlob(200, base.RenderPageList(pg, c))
	return nil
}

func AdminListArticles(c echo.Context) error {
	pg, err := page.AdminArticlesList()
	if err != nil {
		c.Error(err)
		return err
	}
	c.HTMLBlob(200, base.RenderPageList(pg, c))
	return nil
}

func EditArticle(c echo.Context) error {
	pg, err := page.ArticleForm()
	if err != nil {
		c.Error(err)
		return err
	}
	ctx.SetFormReferrer(c) // save the referrer calling for edit
	c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return nil
}

func UpsertArticle(c echo.Context) error {
	csrf := c.FormValue("csrf")
	if !app.VerifyFormToken(csrf) { // check that csrf is present and valid in Redis
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		c.Error(err)
		return err
	}
	artPres := article.Presenter{}
	artPres.Id = c.FormValue("article_id")
	artPres.Title = c.FormValue("article_title")

	summary := c.FormValue("article_summary")
	str, err := chimage.ProcessInlineImages(summary)
	if err != nil {
		logger.LogErr(err, "Error processing summary inline image", "article_id", artPres.Id, "article_title", artPres.Title)
		artPres.Summary = summary
	} else {
		artPres.Summary = str
	}

	body := c.FormValue("article_body")
	str, err = chimage.ProcessInlineImages(body)
	if err != nil {
		logger.LogErr(err, "Error processing body inline image", "article_id", artPres.Id, "article_title", artPres.Title)
		artPres.Body = body
	} else {
		artPres.Body = str
	}

	artPres.Categories = strings.Split(c.FormValue("categories"), ",")
	artPres.UpdatedBy = c.(*ctx.CustomContext).Session.Username
	if c.FormValue("published") == "on" {
		artPres.Published = true
	}

	err = artPres.UpsertArticle()
	if err != nil {
		c.Error(err)
		return err
	}
	msg := "Created"
	if artPres.Id != "0" && artPres.Id != "" {
		msg = "Updated"
	}

	redirectTo := "/admin/articles"
	if cc, ok := c.(*ctx.CustomContext); ok && cc.Session.FormReferrer != "" {
		redirectTo = cc.Session.FormReferrer // return to the form caller
	}
	app.Redirect(c, redirectTo, "Article "+msg)
	return nil
}

func DeleteArticle(c echo.Context) error {
	err := article.DeleteArticleById(c.Param("id"))
	msg := "Article with id: " + c.Param("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete article with id: " + c.Param("id")
		logger.LogErr(err, "when", "deleting article")
	}
	app.Redirect(c, "/admin/articles", msg)
	return nil
}
