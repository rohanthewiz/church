package article

import (
	"fmt"
	"strconv"

	"github.com/rohanthewiz/church/model"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// QueryArticles keeps its legacy signature so module code (carousel,
// easy_tabs, blog, list, recent) doesn't need to change. Under the hood
// it now calls the hand-written DAO instead of the SQLBoiler models pkg.
func QueryArticles(condition, order string, limit, offset int64) (presenters []Presenter, err error) {
	arts, err := model.QueryArticles(condition, order, limit, offset)
	if err != nil {
		return nil, serr.Wrap(err, "Error querying articles")
	}
	for _, a := range arts {
		presenters = append(presenters, presenterFromModel(a))
	}
	return
}

func RecentArticles(limit int64) (presenters []Presenter, err error) {
	return QueryArticles("1 = 1", "created_at DESC", limit, 0)
}

// UpsertArticle decides create vs update from the presenter's Id field,
// same semantics as before. The create/update split lives in modelFromPresenter
// which returns a boolean so we don't have to re-check here.
func (p Presenter) UpsertArticle() error {
	art, create, err := modelFromPresenter(p)
	if err != nil {
		return serr.Wrap(err, "Error in article from presenter")
	}
	if create {
		if err := model.InsertArticle(art); err != nil {
			return serr.Wrap(err, "Error inserting new article into DB")
		}
		Log("Info", "Successfully created article")
	} else {
		if err := model.UpdateArticle(art); err != nil {
			return serr.Wrap(err, "Error updating article in DB")
		}
		Log("Info", "Successfully updated article")
	}
	return nil
}

func DeleteArticleById(id string) error {
	const when = "When deleting article by id"
	if id == "" {
		return serr.New("Id to delete is empty string")
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Article id to integer", "Id", id, "when", when)
	}
	if err := model.DeleteArticle(intId); err != nil {
		return serr.Wrap(err, "Error when deleting article by id", "id", id, "when", when)
	}
	return nil
}

// findModelByIdOrCreate returns the existing article for id, or a blank
// one ready for INSERT when id is empty / invalid. Silent-fallback behavior
// is intentional: it matches the prior path where any lookup error produced
// a fresh model.Article and let the upsert proceed as a create.
func findModelByIdOrCreate(id string) *model.Article {
	if id == "" {
		return &model.Article{}
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		LogErr(err, "Unable to convert Article id to integer", "Id", id)
		return &model.Article{}
	}
	art, err := findArticleById(intId)
	if err != nil || art == nil {
		return &model.Article{}
	}
	return art
}

func findArticleById(id int64) (*model.Article, error) {
	art, err := model.ArticleByID(id)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving article by id", "id", fmt.Sprintf("%d", id))
	}
	return art, nil
}

func findArticleBySlug(slug string) (*model.Article, error) {
	art, err := model.ArticleBySlug(slug)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving article by slug", "slug", slug)
	}
	return art, nil
}