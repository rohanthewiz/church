package page

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
// boundaries (modules, controllers, the page presenters' entry points) fetch
// db.Db() and pass it down.

// Fixup Received data for Presenter
func UpsertPage(exec db.Executor, pres Presenter) (pgUrl string, err error) {
	model, create, err := modelFromPresenter(exec, pres)
	if err != nil {
		return pgUrl, serr.Wrap(err, "Error in page model from presenter")
	}
	if create {
		err = model.Insert(exec)
		if err != nil {
			return pgUrl, serr.Wrap(err, "Error inserting new page into DB")
		} else {
			Log("Info", "Successfully inserted page into db")
		}
	} else {
		err = model.Update(exec)
		if err != nil {
			return pgUrl, serr.Wrap(err, "Error updating page in DB")
		} else {
			Log("Info", "Successfully updated page")
		}
	}
	pgUrl = "/pages/" + model.Slug
	return
}

func queryPages(exec db.Executor, condition, order string, limit int64, offset int64) ([]Presenter, error) {
	presenters := []Presenter{}
	pages, err := models.Pages(exec, Where(condition), OrderBy(order), Limit(int(limit)), Offset(int(offset))).All()
	if err != nil {
		return presenters, serr.Wrap(err, "Error obtaining DB handle")
	}
	errs := []error{}
	for _, page := range pages {
		pres, err := presenterFromModel(page)
		if err != nil {
			errs = append(errs, err)
		}
		presenters = append(presenters, pres)
	}
	if len(errs) > 0 {
		err = errs[0]
	} // todo - better way
	return presenters, err
}

func DeletePageById(exec db.Executor, id string) error {
	const when = "When deleting page by id"
	if id == "" {
		return serr.New("Id to delete is empty string", "when", when)
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Page id to integer", "Id", id, "when", when)
	}
	err = models.Pages(exec, Where("id=?", intId)).DeleteAll()
	if err != nil {
		return serr.Wrap(err, "Error when deleting page by id", "id", id, "when", when)
	}
	return nil
}

func findPageById(exec db.Executor, id int64) (*models.Page, error) {
	pg, err := models.Pages(exec, Where("id = ?", id)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving page by id", "id", fmt.Sprintf("%d", id))
	}
	return pg, err
}

func findPageBySlug(exec db.Executor, slug string) (*models.Page, error) {
	pg, err := models.Pages(exec, Where("slug = ?", slug)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving page by slug", "slug", slug)
	}
	return pg, err
}
