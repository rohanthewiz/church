package page

import (
	"fmt"
	"github.com/rohanthewiz/church/chweb/db"
	"github.com/rohanthewiz/church/models"
	. "github.com/vattle/sqlboiler/queries/qm"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	"strconv"
)
// Fixup Received data for Presenter
func UpsertPage(pres Presenter) (pgUrl string, err error) {
	dbH, err := db.Db()
	if err != nil {
		return  pgUrl, err
	}
	model, create, err := modelFromPresenter(pres)
	if err != nil {
		return pgUrl, serr.Wrap(err, "Error in page model from presenter")
	}
	if create {
		err = model.Insert(dbH)
		if err != nil {
			return pgUrl, serr.Wrap(err, "Error inserting new page into DB", "location", FunctionLoc())
		} else {
			Log("Info", "Successfully inserted page into db")
		}
	} else {
		err = model.Update(dbH)
		if err != nil {
			return pgUrl, serr.Wrap(err, "Error updating modelicle in DB", "location", FunctionLoc())
		} else {
			Log("Info", "Successfully updated page")
		}
	}
	pgUrl = "/pages/" + model.Slug
	return pgUrl, err
}

func queryPages(condition, order string, limit int64, offset int64) ([]Presenter, error) {
	fmt.Println("condition:", condition, " order:", order, " limit:", limit, " offset:", offset)
	presenters := []Presenter{}
	dbH, err := db.Db()
	if err != nil {
		return presenters, err
	}
	pages, err := models.Pages(dbH, Where(condition), OrderBy(order), Limit(int(limit)), Offset(int(offset))).All()
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
	if len(errs) > 0 { err = errs[0]}  // todo - better way
	return presenters, err
}

func DeletePageById(id string) error {
	const when = "When deleting page by id"
	dbH, err := db.Db()
	if err != nil {
		return  err
	}
	if id == "" { return serr.NewSErr("Id to delete is empty string", "when", when) }
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Page id to integer", "Id", id, "when", when)
	}
	err = models.Pages(dbH, Where("id=?", intId)).DeleteAll()
	if err != nil {
		return serr.Wrap(err, "Error when deleting page by id", "id", id, "when", when)
	}
	return nil
}

func findPageById(id int64) (*models.Page, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Error obtaining DB handle")
	}
	pg, err := models.Pages(dbH, Where("id = ?", id)).One()
	if  err != nil {
		return nil, serr.Wrap(err, "Error retrieving page by id", "id", fmt.Sprintf("%d", id))
	}
	return pg, err
}

func findPageBySlug(slug string) (*models.Page, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Error obtaining DB handle")
	}
	pg, err := models.Pages(dbH, Where("slug = ?", slug)).One()
	if  err != nil {
		return nil, serr.Wrap(err, "Error retrieving page by slug", "slug", slug)
	}
	return pg, err
}
