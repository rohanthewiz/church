package event

import (
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/chweb/db"
	. "github.com/vattle/sqlboiler/queries/qm"
	. "github.com/rohanthewiz/logger"
	"fmt"
	"github.com/rohanthewiz/church/chweb/util/stringops"
	"github.com/rohanthewiz/serr"
	"strconv"
)

func UpComingEvents() ([]Presenter, error) {
	efs := []Presenter{}
	db, err := db.Db()
	if err != nil {
		return efs, err
	}
	events, err := models.Events(db, OrderBy("event_date DESC"), Limit(8)).All()
	if err != nil {
		Log("Error", "Error obtaining upcoming events", "err", err.Error())
		return efs, err
	}
	for _, evt := range events {
		efs = append(efs, presenterFromModel(evt))
	}
	return efs, err
}

// Condition is the condition expression without leading/trailing WHERE and AND
func QueryEvents(condition, order string, limit int64, offset int64) ([]Presenter, error) {
	fmt.Println("condition:", condition, " order:", order, " limit:", limit, " offset:", offset)
	pres := []Presenter{}
	db, err := db.Db()
	if err != nil {
		return pres, err
	}
	events, err := models.Events(db, Where(condition), OrderBy(order), Limit(int(limit)), Offset(int(offset))).All()
	if err != nil {
		LogErr(serr.Wrap(err, "Error obtaining events"))
		return pres, err
	}
	for _, evt := range events {
		pres = append(pres, presenterFromModel(evt))
	}
	return pres, err
}


// Given a Presenter, update or insert
func (p Presenter) UpsertEvent() error {
	db, err := db.Db()
	if err != nil {
		return  err
	}
	evt, create, err := modelFromPresenter(p)
	if err != nil {
		serr.Wrap(err, "Error in event model upsert", "location", FunctionLoc())
		return err
	}
	if create {
		evt.Slug = stringops.SlugWithRandomString(evt.Title) // create the unique id for the module
		err = evt.Insert(db)
		if err != nil {
			serr.Wrap(err, "Error inserting event into DB","location", FunctionLoc())
			return err
		} else {
			Log("Info", "Successfully created event")
		}
	} else {
		err = evt.Update(db)
		if err != nil {
			serr.Wrap(err, "Error updating event in DB", "location", FunctionLoc())
		} else {
			Log("Info", "Successfully updated event")
		}
	}
	return err
}

func DeleteEventById(id string) error {
	const when = "When deleting event by id"
	dbH, err := db.Db()
	if err != nil {
		return  err
	}
	if id == "" { return serr.NewSErr("Id to delete is empty string", "when", when) }
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Event id to integer", "Id", id, "when", when)
	}
	err = models.Events(dbH, Where("id=?", intId)).DeleteAll()
	if err != nil {
		return serr.Wrap(err, "Error when deleting event by id", "id", id, "when", when)
	}
	return nil
}

func findEventById(id int64) (*models.Event, error) {
	db, err := db.Db()
	if err != nil {
		Log("Error", "Error in EventById()", "error", err.Error())
		return nil, err
	}
	evt, err := models.Events(db, Where("id = ?", id)).One()
	if err != nil {
		return nil, err
	}
	return evt, err
}

// Returns the models.Presenter with id `id` or a new models.Presenter
func findModelByIdOrCreate(id string) (*models.Event) {
	var evt *models.Event
	if id != "" {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			Log("Error", "Unable to convert Presenter id to integer", "Id", id, "error", err.Error())
			return new(models.Event)
		}
		evt, err = findEventById(intId)
		if err != nil {
			return new(models.Event)
		}
	} else {
		evt = new(models.Event)
	}
	return evt
}

//func FirstEvent() (Presenter, error) {
//	efs := Presenter{}
//	db, err := db.Db()
//	if err != nil {
//		return efs, err
//	}
//	ev, err := models.Events(db).One()
//	if err != nil {
//		return efs, err
//	}
//	return presenterFromModel(ev), err
//}


//func PresenterById(param_id string) (Presenter, error) {
//	efs := Presenter{}
//	id, err := strconv.ParseInt(strings.TrimSpace(param_id), 10, 64)
//	if err != nil {
//		Log("Error", "Could not convert param_id to int", "when", "obtaining event by id", "error", err.Error())
//		return efs, err
//	}
//	evt, err := findEventById(id)
//	if err != nil {
//		Log("Error", "Unable to obtain event with id: " + param_id, "error", err.Error())
//		return efs, err
//	}
//	return presenterFromModel(evt), err
//}

// Private

