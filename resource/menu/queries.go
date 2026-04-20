package menu

import (
	"strconv"

	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

func UpsertMenu(menuDef MenuDef) error {
	m, create, err := modelFromMenuDef(menuDef)
	if err != nil {
		return serr.Wrap(err, "Error in menu model from presenter")
	}
	if create {
		if err := model.InsertMenuDef(m); err != nil {
			return serr.Wrap(err, "Error inserting new menu into DB")
		}
		logger.Log("Info", "Successfully inserted menu into db")
	} else {
		if err := model.UpdateMenuDef(m); err != nil {
			return serr.Wrap(err, "Error updating menu model in DB")
		}
		logger.Log("Info", "Successfully updated menu")
	}
	return nil
}

// findModelByIdOrCreate: same silent-fallback contract as other resources.
func findModelByIdOrCreate(id string) *model.MenuDef {
	if id == "" {
		return &model.MenuDef{}
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		logger.LogErr(err, "Unable to convert Menu id to integer", "Id", id)
		return &model.MenuDef{}
	}
	mn, err := findModelById(intId)
	if err != nil || mn == nil {
		return &model.MenuDef{}
	}
	return mn
}

func DeleteMenuById(id string) error {
	const when = "When deleting menu by id"
	if id == "" {
		return serr.NewSErr("Id to delete is empty string", "when", when)
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Menu id to integer", "Id", id, "when", when)
	}
	if err := model.DeleteMenuDef(intId); err != nil {
		return serr.Wrap(err, "Error when deleting menu by id", "id", id, "when", when)
	}
	return nil
}

func findModelById(id int64) (*model.MenuDef, error) {
	return model.MenuDefByID(id)
}

func findModelBySlug(slug string) (*model.MenuDef, error) {
	return model.MenuDefBySlug(slug)
}

func queryMenus(condition, order string, limit int64, offset int64) ([]MenuDef, error) {
	presenters := []MenuDef{}
	modelMenuDefs, err := model.QueryMenuDefs(condition, order, limit, offset)
	if err != nil {
		logger.LogErr(err, "Error obtaining list of menus")
		return presenters, err
	}
	// Collect first error from conversion but continue building the list —
	// preserves old behavior where one bad row didn't blank the whole page.
	var errs []error
	for _, mnu := range modelMenuDefs {
		pres, err := menuDefFromModel(mnu)
		if err != nil {
			errs = append(errs, err)
		}
		presenters = append(presenters, pres)
	}
	if len(errs) > 0 {
		err = errs[0]
	}
	return presenters, err
}
