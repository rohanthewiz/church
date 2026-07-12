package event

import (
	"strconv"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/util/stringops"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	. "github.com/vattle/sqlboiler/queries/qm"
)

// Query functions take the executor first (db.Executor — see db/executor.go);
// boundaries (modules, controllers) fetch db.Db() and pass it down.

func UpComingEvents(exec db.Executor) ([]Presenter, error) {
	efs := []Presenter{}
	events, err := models.Events(exec, OrderBy("event_date DESC"), Limit(8)).All()
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
func QueryEvents(exec db.Executor, condition, order string, limit int64, offset int64) ([]Presenter, error) {
	var pres []Presenter

	events, err := models.Events(exec, Where(condition), OrderBy(order), Limit(int(limit)), Offset(int(offset))).All()
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
func (p Presenter) UpsertEvent(exec db.Executor) error {
	evt, create, err := modelFromPresenter(exec, p)
	if err != nil {
		return serr.Wrap(err)
	}
	if create {
		evt.Slug = stringops.SlugWithRandomString(evt.Title) // create the unique id for the module
		err = evt.Insert(exec)
		if err != nil {
			return serr.Wrap(err, "Error inserting event into DB")
		} else {
			Log("Info", "Successfully created event")
		}
	} else {
		err = evt.Update(exec)
		if err != nil {
			return serr.Wrap(err, "Error updating event in DB")
		} else {
			Log("Info", "Successfully updated event")
		}
	}

	// Sync the recurrence rule after the event row exists (Insert populates
	// evt.ID via RETURNING). A failure here is reported, not swallowed — an
	// admin who set "every Sunday" must know if the rule didn't stick.
	if err = p.upsertRecurrenceRule(exec, evt.ID); err != nil {
		return serr.Wrap(err, "Event saved but its recurrence rule failed to save")
	}
	return nil
}

// upsertRecurrenceRule translates the presenter's form-string recurrence
// fields into a validated rule row, or removes the rule when frequency is
// back to "None".
func (p Presenter) upsertRecurrenceRule(exec db.Executor, eventID int64) error {
	if p.RecurFreq == RecurNone {
		return DeleteRecurrence(exec, eventID)
	}

	weekday, err := strconv.Atoi(p.RecurWeekday)
	if err != nil {
		return serr.Wrap(err, "recurrence weekday must be numeric", "weekday", p.RecurWeekday)
	}
	rec := Recurrence{
		EventID: eventID,
		Freq:    p.RecurFreq,
		Weekday: time.Weekday(weekday),
	}
	if p.RecurFreq == RecurMonthly {
		// The form always submits a week value; it is only meaningful for monthly
		if rec.Week, err = strconv.Atoi(p.RecurWeek); err != nil {
			return serr.Wrap(err, "recurrence week must be numeric", "week", p.RecurWeek)
		}
	}
	if p.RecurUntil != "" {
		if rec.Until, err = time.Parse("2006-01-02", p.RecurUntil); err != nil {
			return serr.Wrap(err, "recurrence until must be YYYY-MM-DD", "until", p.RecurUntil)
		}
	}
	return UpsertRecurrence(exec, rec) // validates the assembled rule
}

func DeleteEventById(exec db.Executor, id string) error {
	const when = "When deleting event by id"
	if id == "" {
		return serr.New("Id to delete is empty string", "when", when)
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Event id to integer", "Id", id, "when", when)
	}
	err = models.Events(exec, Where("id=?", intId)).DeleteAll()
	if err != nil {
		return serr.Wrap(err, "Error when deleting event by id", "id", id, "when", when)
	}
	return nil
}

func findEventById(exec db.Executor, id int64) (*models.Event, error) {
	evt, err := models.Events(exec, Where("id = ?", id)).One()
	if err != nil {
		return nil, err
	}
	return evt, err
}

// Returns the models.Presenter with id `id` or a new models.Presenter
func findModelByIdOrCreate(exec db.Executor, id string) *models.Event {
	var evt *models.Event
	if id != "" {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			Log("Error", "Unable to convert Presenter id to integer", "Id", id, "error", err.Error())
			return new(models.Event)
		}
		evt, err = findEventById(exec, intId)
		if err != nil {
			return new(models.Event)
		}
	} else {
		evt = new(models.Event)
	}
	return evt
}

// func FirstEvent() (Presenter, error) {
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
// }

// func PresenterById(param_id string) (Presenter, error) {
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
// }

// Private
