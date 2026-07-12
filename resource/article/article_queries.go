package article

import (
	"fmt"
	"strconv"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	. "github.com/vattle/sqlboiler/queries/qm"
)

// Query functions take the executor first (db.Executor — see db/executor.go);
// boundaries (modules, controllers, bootstrap) fetch db.Db() and pass it down.

func QueryArticles(exec db.Executor, condition, order string, limit, offset int64) (presenters []Presenter, err error) {
	articles, err := models.Articles(exec, Where(condition), OrderBy(order), Limit(int(limit)), Offset(int(offset))).All()
	if err != nil {
		return nil, serr.Wrap(err, "Error querying articles")
	}
	for _, art := range articles {
		presenters = append(presenters, presenterFromModel(art))
	}
	return
}

func RecentArticles(exec db.Executor, limit int64) (presenters []Presenter, err error) {
	condition := "1 = 1"
	order := "created_at DESC"
	return QueryArticles(exec, condition, order, limit, 0)
}

func (p Presenter) UpsertArticle(exec db.Executor) error {
	art, create, err := modelFromPresenter(exec, p)
	if err != nil {
		return serr.Wrap(err, "Error in article from presenter")
	}
	if create {
		err = art.Insert(exec)
		if err != nil {
			return serr.Wrap(err, "Error inserting new article into DB")
		} else {
			Log("Info", "Successfully created article")
		}
	} else {
		err = art.Update(exec)
		if err != nil {
			return serr.Wrap(err, "Error updating article in DB")
		} else {
			Log("Info", "Successfully updated article")
		}
	}
	return err
}

func DeleteArticleById(exec db.Executor, id string) error {
	const when = "When deleting article by id"
	if id == "" {
		return serr.New("Id to delete is empty string")
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Article id to integer", "Id", id, "when", when)
	}
	err = models.Articles(exec, Where("id=?", intId)).DeleteAll()
	if err != nil {
		return serr.Wrap(err, "Error when deleting article by id", "id", id, "when", when)
	}
	return nil
}

// Returns an article model for id `id` or a new article model
func findModelByIdOrCreate(exec db.Executor, id string) (art *models.Article) {
	if id != "" {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			LogErr(err, "Unable to convert Article id to integer", "Id", id)
			return new(models.Article)
		}
		art, err = findArticleById(exec, intId)
		if err != nil {
			return new(models.Article)
		}
	}
	if art == nil {
		art = new(models.Article)
	}
	return
}

// Returns an article for id `id` or error
func findArticleById(exec db.Executor, id int64) (*models.Article, error) {
	art, err := models.Articles(exec, Where("id = ?", id)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retreiving article by id", "id", fmt.Sprintf("%d", id))
	}
	return art, err
}

func findArticleBySlug(exec db.Executor, slug string) (*models.Article, error) {
	art, err := models.Articles(exec, Where("slug = ?", slug)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving article by slug", "slug", slug)
	}
	return art, err
}
