package menu

import (
	"github.com/rohanthewiz/church/models"
	theDB "github.com/rohanthewiz/church/chweb/db"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	"fmt"
	"strconv"
	"github.com/vattle/sqlboiler/queries/qm"
)

func UpsertMenu(menuDef MenuDef) error {
	db, err := theDB.Db()
	if err != nil {
		return  err
	}
	model, create, err := modelFromMenuDef(menuDef)
	if err != nil {
		return serr.Wrap(err, "Error in menu model from presenter")
	}
	if create {
		err = model.Insert(db)
		if err != nil {
			return serr.Wrap(err, "Error inserting new menu into DB")
		} else {
			logger.Log("Info", "Successfully inserted menu into db")
		}
	} else {
		err = model.Update(db)
		if err != nil {
			return serr.Wrap(err, "Error updating menu model in DB")
		} else {
			logger.Log("Info", "Successfully updated menu")
		}
	}
	return err
}

// Returns a model for id `id` or a new model
func findModelByIdOrCreate(id string) (mn *models.MenuDef) {
	if id != "" {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			logger.LogErr(err, "Unable to convert Menu id to integer", "Id", id)
			return new(models.MenuDef)
		}
		mn, err = findModelById(intId)
		if err != nil {
			return new(models.MenuDef)
		}
	}
	if mn == nil {
		mn = new(models.MenuDef)
	}
	return
}

func DeleteMenuById(id string) error {
	const when = "When deleting menu by id"
	dbH, err := theDB.Db()
	if err != nil {
		return  err
	}
	if id == "" { return serr.NewSErr("Id to delete is empty string", "when", when) }
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Menu id to integer", "Id", id, "when", when)
	}
	err = models.MenuDefs(dbH, qm.Where("id=?", intId)).DeleteAll()
	if err != nil {
		return serr.Wrap(err, "Error when deleting menu by id", "id", id, "when", when)
	}
	return nil
}

func findModelById(id int64) (*models.MenuDef, error) {
	db, err := theDB.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Error finding menu by id")
	}
	mn, err := models.MenuDefs(db, qm.Where("id = ?", id)).One()
	if  err != nil {
		return nil, serr.Wrap(err, "Error retrieving menu definition by id", "id", fmt.Sprintf("%d", id))
	}
	return mn, err
}

func findModelBySlug(slug string) (*models.MenuDef, error) {
	dbH, err := theDB.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Error finding menu by slug")
	}
	mn, err := models.MenuDefs(dbH, qm.Where("slug = ?", slug)).One()
	if  err != nil {
		return nil, serr.Wrap(err, "Error retrieving menu definition by slug", "slug", slug)
	}
	return mn, err
}

func queryMenus(condition, order string, limit int64, offset int64) ([]MenuDef, error) {
	fmt.Println("condition:", condition, " order:", order, " limit:", limit, " offset:", offset)
	presenters := []MenuDef{}
	db, err := theDB.Db()
	if err != nil {
		return presenters, err
	}
	modelMenuDefs, err := models.MenuDefs(db, qm.Where(condition), qm.OrderBy(order), qm.Limit(int(limit)),
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
	if len(errs) > 0 { err = errs[0]}  // todo - better way
	return presenters, err
}
