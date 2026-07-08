package article

import (
	"net/http"
	"strconv"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/resource/apiv1"
	"github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
	"github.com/vattle/sqlboiler/queries/qm"
)

// ArticleAPI is the public JSON DTO for an article — a deliberate subset of
// the model; never serialize presenters/models directly.
//
// body is Summernote-produced HTML. The mobile app renders it with
// flutter_html (webview fallback if markup proves too rich), so it is only
// sent on the detail endpoint to keep list payloads small.
type ArticleAPI struct {
	ID         int64    `json:"id"`
	Title      string   `json:"title"`
	Slug       string   `json:"slug"`
	Summary    string   `json:"summary"`
	Categories []string `json:"categories"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
	Body       string   `json:"body,omitempty"`
}

func articleToAPI(art *models.Article, includeBody bool) ArticleAPI {
	a := ArticleAPI{
		ID:         art.ID,
		Title:      art.Title,
		Slug:       art.Slug,
		Summary:    art.Summary,
		Categories: art.Categories,
	}
	if a.Categories == nil {
		a.Categories = []string{}
	}
	// created/updated are nullable in the schema; empty string means unknown
	if art.CreatedAt.Valid {
		a.CreatedAt = art.CreatedAt.Time.Format(timeutil.ISO8601DateTime)
	}
	if art.UpdatedAt.Valid {
		a.UpdatedAt = art.UpdatedAt.Time.Format(timeutil.ISO8601DateTime)
	}
	if includeBody {
		a.Body = art.Body.String
	}
	return a
}

// GET /api/v1/articles?limit&offset
// Published articles, newest first (by creation — "newest articles" feed).
func APIArticlesRWeb(ctx rweb.Context) error {
	limit, offset := apiv1.ParseLimitOffset(ctx, 20, 100)

	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	arts, err := models.Articles(dbH, qm.Where("published = true"),
		qm.OrderBy("created_at DESC"), qm.Limit(limit), qm.Offset(offset)).All()
	if err != nil {
		return serr.Wrap(err, "Error obtaining articles")
	}

	articles := make([]ArticleAPI, 0, len(arts))
	for _, art := range arts {
		articles = append(articles, articleToAPI(art, false))
	}

	return ctx.WriteJSON(map[string]any{
		"articles": articles,
		"limit":    limit,
		"offset":   offset,
	})
}

// GET /api/v1/articles/:id — single article including HTML body.
func APIArticleRWeb(ctx rweb.Context) error {
	id, err := strconv.ParseInt(ctx.Request().Param("id"), 10, 64)
	if err != nil {
		return apiv1.Error(ctx, http.StatusBadRequest, "article id must be an integer")
	}

	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	// Drafts 404 identically to nonexistent ids — no oracle for unpublished content
	art, err := models.Articles(dbH, qm.Where("id = ? AND published = true", id)).One()
	if err != nil {
		logger.LogErr(err, "article not found for API", "id", ctx.Request().Param("id"))
		return apiv1.Error(ctx, http.StatusNotFound, "Article not found")
	}

	return ctx.WriteJSON(articleToAPI(art, true))
}

// RecentArticlesAPI returns the newest published articles as API DTOs.
// Exported for the /api/v1/feed aggregator.
func RecentArticlesAPI(limit int) ([]ArticleAPI, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	arts, err := models.Articles(dbH, qm.Where("published = true"),
		qm.OrderBy("created_at DESC"), qm.Limit(limit)).All()
	if err != nil {
		return nil, serr.Wrap(err, "Error obtaining recent articles")
	}
	out := make([]ArticleAPI, 0, len(arts))
	for _, art := range arts {
		out = append(out, articleToAPI(art, false))
	}
	return out, nil
}
