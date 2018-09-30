package page

import (
	"fmt"
	"strings"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/chweb/config"
	"github.com/rohanthewiz/serr"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/chweb/module"
	"gopkg.in/nullbio/null.v6"
	"encoding/json"
	"errors"
	"strconv"
)

func PageFromId(id string) (*Page, error) {
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil { return nil, serr.Wrap(err, "Error converting page id to int", "location", FunctionLoc()) }
	model, err := findPageById(intId)
	if err != nil { return nil, err }
	pres, err := presenterFromModel(model)
	if err != nil {
		return nil, serr.Wrap(err, "Error in page model to presenter", "location", FunctionLoc())
	}
	return pageFromPresenter(pres), nil
}

func PageFromSlug(slug string) (pg *Page, err error) {
	fmt.Printf("In PageFromSlug - slug: '%s'\n", slug)
	pres, err := presenterFromSlug(slug)
	if err != nil { return pg, err }
	return pageFromPresenter(pres), nil
}

func presenterFromSlug(slug string) (pres Presenter, err error) {
	model, err := findPageBySlug(slug)
	if err != nil {
		return pres, serr.Wrap(err, "Error finding page by slug", "location", FunctionLoc())
	}
	pres, err = presenterFromModel(model)
	if err != nil {
		return pres, serr.Wrap(err, "Error in page model to presenter", "location", FunctionLoc())
	}
	return
}

func presenterFromModel(model *models.Page) (pres Presenter, err error) {
	pres.Id = fmt.Sprintf("%d", model.ID)
	if model.CreatedAt.Valid {
		pres.CreatedAt = model.CreatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	if model.UpdatedAt.Valid {
		pres.UpdatedAt = model.UpdatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	pres.UpdatedBy = model.UpdatedBy
	pres.Published = model.Published
	pres.IsHome = model.IsHome
	pres.IsAdmin = model.IsAdmin
	pres.Title = model.Title
	pres.Slug = model.Slug

	availPos := []string{}
	for _, pos := range model.AvailablePositions {
		availPos = append(availPos, pos)
	}
	pres.AvailablePositions = availPos

	// The JSON approach
	modPresenters := []module.Presenter{}
	model.Data.Unmarshal(&modPresenters)
	pres.Modules = modPresenters

	return
}

func modelFromPresenter(pres Presenter) (model *models.Page, create_op bool, err error) {
	model = findPageByIdOrCreate(pres.Id)
	if model.ID < 1 {
		create_op = true
	}

	if updatedBy := strings.TrimSpace(pres.UpdatedBy); updatedBy != "" {
		model.UpdatedBy = updatedBy
	}

	if title := strings.TrimSpace(pres.Title); title != "" {
		model.Title = title
	} else {
		er := serr.Wrap(errors.New("Page title should not be blank"), "location", FunctionLoc())
		return nil, create_op, er
	}
	if create_op {  // Allow slug update only on create to maintain external references
		pres.CreateSlug() // slug has to be unique only on the page
		model.Slug = pres.Slug  // todo: optimize
	}
	model.Published = pres.Published
	model.IsAdmin = pres.IsAdmin
	model.IsHome = pres.IsHome
	availPos := []string{}
	for _, pos := range pres.AvailablePositions {
		if trimmed := strings.TrimSpace(pos); trimmed != "" {
			availPos = append(availPos, trimmed)
		}
	}
	model.AvailablePositions = availPos
	// JSON approach
	modulesAsJsonBytes, err := json.Marshal(pres.Modules)
	if err != nil {
		return nil, create_op, serr.Wrap(err, "Error marshalling page presenter modules")
	}
	model.Data = null.NewJSON(modulesAsJsonBytes, true)

	return
}
