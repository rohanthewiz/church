package page

import (
	"strconv"

	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// UpsertPage inserts or updates a page row derived from the presenter, and
// returns the public URL (/pages/:slug) of the persisted row.
func UpsertPage(pres Presenter) (pgUrl string, err error) {
	m, create, err := modelFromPresenter(pres)
	if err != nil {
		return pgUrl, serr.Wrap(err, "Error in page model from presenter")
	}
	if create {
		if err := model.InsertPage(m); err != nil {
			return pgUrl, serr.Wrap(err, "Error inserting new page into DB")
		}
		logger.Log("Info", "Successfully inserted page into db")
	} else {
		if err := model.UpdatePage(m); err != nil {
			return pgUrl, serr.Wrap(err, "Error updating page in DB")
		}
		logger.Log("Info", "Successfully updated page")
	}
	return "/pages/" + m.Slug, nil
}

func queryPages(condition, order string, limit int64, offset int64) ([]Presenter, error) {
	presenters := []Presenter{}
	pages, err := model.QueryPages(condition, order, limit, offset)
	if err != nil {
		return presenters, serr.Wrap(err, "Error querying pages")
	}
	// Collect first error but keep building the list — one bad row should not
	// blank the entire page listing.
	var errs []error
	for _, p := range pages {
		pres, err := presenterFromModel(p)
		if err != nil {
			errs = append(errs, err)
		}
		presenters = append(presenters, pres)
	}
	if len(errs) > 0 {
		return presenters, errs[0]
	}
	return presenters, nil
}

func DeletePageById(id string) error {
	const when = "When deleting page by id"
	if id == "" {
		return serr.New("Id to delete is empty string", "when", when)
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Page id to integer", "Id", id, "when", when)
	}
	if err := model.DeletePage(intId); err != nil {
		return serr.Wrap(err, "Error when deleting page by id", "id", id, "when", when)
	}
	return nil
}

func findPageById(id int64) (*model.Page, error) {
	return model.PageByID(id)
}

func findPageBySlug(slug string) (*model.Page, error) {
	return model.PageBySlug(slug)
}
