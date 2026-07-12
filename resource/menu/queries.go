package menu

import (
	"fmt"
	"strconv"

	theDB "github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	"github.com/vattle/sqlboiler/queries/qm"
)

// Query functions take the executor first (theDB.Executor — see db/executor.go);
// boundaries (modules, controllers, menu rendering entry points) fetch
// theDB.Db() and pass it down.

func UpsertMenu(exec theDB.Executor, menuDef MenuDef) error {
	model, create, err := modelFromMenuDef(exec, menuDef)
	if err != nil {
		return serr.Wrap(err, "Error in menu model from presenter")
	}
	if create {
		err = model.Insert(exec)
		if err != nil {
			return serr.Wrap(err, "Error inserting new menu into DB")
		} else {
			logger.Log("Info", "Successfully inserted menu into db")
		}
	} else {
		err = model.Update(exec)
		if err != nil {
			return serr.Wrap(err, "Error updating menu model in DB")
		} else {
			logger.Log("Info", "Successfully updated menu")
		}
	}
	return err
}

// Returns a model for id `id` or a new model
func findModelByIdOrCreate(exec theDB.Executor, id string) (mn *models.MenuDef) {
	if id != "" {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			logger.LogErr(err, "Unable to convert Menu id to integer", "Id", id)
			return new(models.MenuDef)
		}
		mn, err = findModelById(exec, intId)
		if err != nil {
			return new(models.MenuDef)
		}
	}
	if mn == nil {
		mn = new(models.MenuDef)
	}
	return
}

func DeleteMenuById(exec theDB.Executor, id string) error {
	const when = "When deleting menu by id"
	if id == "" {
		return serr.NewSErr("Id to delete is empty string", "when", when)
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Menu id to integer", "Id", id, "when", when)
	}
	err = models.MenuDefs(exec, qm.Where("id=?", intId)).DeleteAll()
	if err != nil {
		return serr.Wrap(err, "Error when deleting menu by id", "id", id, "when", when)
	}
	return nil
}

func findModelById(exec theDB.Executor, id int64) (*models.MenuDef, error) {
	mn, err := models.MenuDefs(exec, qm.Where("id = ?", id)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving menu definition by id", "id", fmt.Sprintf("%d", id))
	}
	return mn, err
}

func findModelBySlug(exec theDB.Executor, slug string) (*models.MenuDef, error) {
	mn, err := models.MenuDefs(exec, qm.Where("slug = ?", slug)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving menu definition by slug", "slug", slug)
	}
	return mn, err
}

func queryMenus(exec theDB.Executor, condition, order string, limit int64, offset int64) ([]MenuDef, error) {
	presenters := []MenuDef{}
	modelMenuDefs, err := models.MenuDefs(exec, qm.Where(condition), qm.OrderBy(order), qm.Limit(int(limit)),
		qm.Offset(int(offset))).All()
	if err != nil {
		logger.LogErr(err, "Error obtaining list of menus")
		return presenters, err
	}
	errs := []error{}
	for _, mnu := range modelMenuDefs {
		pres, err := menuDefFromModel(mnu)
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
