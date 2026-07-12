package page

import (
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/util/stringops"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// Store Page definition
type Presenter struct {
	Id           string
	CreatedAt    string
	UpdatedAt    string
	UpdatedBy    string
	Title string
	Slug string  // slug is the unique identifier to the page instance
	Published    bool
	IsHome	bool
	IsAdmin bool
	AvailablePositions []string
	Modules []module.Presenter
}


func (p * Presenter) CreateSlug() {
	if p.Title == "" { println("Title should be set before Slug"); return }
	p.Slug = stringops.SlugWithRandomString(p.Title)
}


// Given an id, get the model and build a presenter from the model
func PresenterById(exec db.Executor, paramId string) (presenter Presenter, err error) {
	id, err := strconv.ParseInt(strings.TrimSpace(paramId), 10, 64)
	if err != nil {
		return presenter, serr.Wrap(err, "Could not convert paramId to int", "when", "building page Presenter")
	}
	model, err := findPageById(exec, id)
	if err != nil {
		return presenter, serr.Wrap(err, "Unable to obtain sermon", "id", paramId)
	}
	return presenterFromModel(model)
}

// Returns a model for id `id` or a new model
func findPageByIdOrCreate(exec db.Executor, id string) (pg *models.Page) {
	if id != "" {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			LogErr(err, "Unable to convert Page id to integer", "Id", id)
			return new(models.Page)
		}
		pg, err = findPageById(exec, intId)
		if err != nil {
			return new(models.Page)
		}
	}
	if pg == nil {
		pg = new(models.Page)
	}
	return
}
