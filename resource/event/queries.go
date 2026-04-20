package event

import (
	"strconv"

	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/church/util/stringops"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// UpComingEvents fetches the next batch of events ordered by most recent
// date first (legacy behavior — DESC not ASC, preserved to avoid silently
// changing admin dashboards that depend on the ordering).
func UpComingEvents() ([]Presenter, error) {
	return QueryEvents("", "event_date DESC", 8, 0)
}

// QueryEvents keeps the legacy signature (condition/order are trusted SQL
// fragments built from internal module config, not user input). Under the
// hood it delegates to the hand-written DAO.
func QueryEvents(condition, order string, limit int64, offset int64) ([]Presenter, error) {
	pres := []Presenter{}
	events, err := model.QueryEvents(condition, order, limit, offset)
	if err != nil {
		return pres, serr.Wrap(err, "Error obtaining events")
	}
	for _, evt := range events {
		pres = append(pres, presenterFromModel(evt))
	}
	return pres, nil
}

// UpsertEvent decides create vs update from pres.Id via modelFromPresenter.
// On create we synthesise a unique slug here — the old behavior did it on
// the model struct in place, but doing it here keeps the DAO agnostic of
// any business rules about slug generation.
func (p Presenter) UpsertEvent() error {
	evt, create, err := modelFromPresenter(p)
	if err != nil {
		return serr.Wrap(err)
	}
	if create {
		evt.Slug = stringops.SlugWithRandomString(evt.Title)
		if err := model.InsertEvent(evt); err != nil {
			return serr.Wrap(err, "Error inserting event into DB")
		}
		Log("Info", "Successfully created event")
	} else {
		if err := model.UpdateEvent(evt); err != nil {
			return serr.Wrap(err, "Error updating event in DB")
		}
		Log("Info", "Successfully updated event")
	}
	return nil
}

func DeleteEventById(id string) error {
	const when = "When deleting event by id"
	if id == "" {
		return serr.New("Id to delete is empty string", "when", when)
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Event id to integer", "Id", id, "when", when)
	}
	if err := model.DeleteEvent(intId); err != nil {
		return serr.Wrap(err, "Error when deleting event by id", "id", id, "when", when)
	}
	return nil
}

func findEventById(id int64) (*model.Event, error) {
	evt, err := model.EventByID(id)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving event by id")
	}
	return evt, nil
}

// findModelByIdOrCreate: same fallback-on-error semantics as the article
// path — callers treat a blank model as "prepare a new row".
func findModelByIdOrCreate(id string) *model.Event {
	if id == "" {
		return &model.Event{}
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		Log("Error", "Unable to convert Presenter id to integer", "Id", id, "error", err.Error())
		return &model.Event{}
	}
	evt, err := findEventById(intId)
	if err != nil || evt == nil {
		return &model.Event{}
	}
	return evt
}
