package article

import (
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/db"
	. "github.com/vattle/sqlboiler/queries/qm"
	"github.com/rohanthewiz/serr"
	"fmt"
	. "github.com/rohanthewiz/logger"
	"strconv"
	"errors"
)

func QueryArticles(condition, order string, limit, offset int64) (presenters []Presenter, err error) {
	Log("Debug", "Article query", "condition:", condition, " order:", order,
		" limit:", fmt.Sprintf("%d", limit), " offset:", fmt.Sprintf("%d", offset))
	db, err := db.Db()
	if err != nil {
		return
	}
	articles, err := models.Articles(db, Where(condition), OrderBy(order), Limit(int(limit)), Offset(int(offset))).All()
	if err != nil {
		return nil, serr.Wrap(err, "Error querying articles")
	}
	for _, art := range articles {
		presenters = append(presenters, presenterFromModel(art))
	}
	return
}

func RecentArticles(limit int64) (presenters []Presenter, err error) {
	condition := "1 = 1"
	order := "created_at DESC"
	return QueryArticles(condition, order, limit, 0)
}

func (p Presenter) UpsertArticle() error {
	db, err := db.Db()
	if err != nil {
		return  err
	}
	art, create, err := modelFromPresenter(p)
	if err != nil {
		return serr.Wrap(err, "Error in article from presenter")
	}
	if create {
		err = art.Insert(db)
		if err != nil {
			return serr.Wrap(err, "Error inserting new article into DB")
		} else {
			Log("Info", "Successfully created article")
		}
	} else {
		err = art.Update(db)
		if err != nil {
			return serr.Wrap(err, "Error updating article in DB")
		} else {
			Log("Info", "Successfully updated article")
		}
	}
	return err
}

func DeleteArticleById(id string) error {
	const when = "When deleting article by id"
	dbH, err := db.Db()
	if err != nil {
		return  err
	}
	if id == "" { return errors.New("Id to delete is empty string") }
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Article id to integer", "Id", id, "when", when)
	}
	err = models.Articles(dbH, Where("id=?", intId)).DeleteAll()
	if err != nil {
		return serr.Wrap(err, "Error when deleting article by id", "id", id, "when", when)
	}
	return nil
}

// Returns an article model for id `id` or a new article model
func findModelByIdOrCreate(id string) (art *models.Article) {
	if id != "" {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			LogErr(err, "Unable to convert Article id to integer", "Id", id)
			return new(models.Article)
		}
		art, err = findArticleById(intId)
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
func findArticleById(id int64) (*models.Article, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "location", FunctionLoc())
	}
	art, err := models.Articles(dbH, Where("id = ?", id)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retreiving article by id", "id", fmt.Sprintf("%d", id))
	}
	return art, err
}

func findArticleBySlug(slug string) (*models.Article, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Error obtaining DB handle")
	}
	art, err := models.Articles(dbH, Where("slug = ?", slug)).One()
	if  err != nil {
		return nil, serr.Wrap(err, "Error retrieving article by slug", "slug", slug)
	}
	return art, err
}
