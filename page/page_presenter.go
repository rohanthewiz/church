package page

import (
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/util/stringops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// Presenter is the view-layer shape of a page — flattened strings for the
// form/view code and a []module.Presenter decoded from the stored JSONB.
type Presenter struct {
	Id                 string
	CreatedAt          string
	UpdatedAt          string
	UpdatedBy          string
	Title              string
	Slug               string // slug is the unique identifier to the page instance
	Published          bool
	IsHome             bool
	IsAdmin            bool
	AvailablePositions []string
	Modules            []module.Presenter
}

func (p *Presenter) CreateSlug() {
	if p.Title == "" {
		logger.Log("Warn", "Title should be set before Slug")
		return
	}
	p.Slug = stringops.SlugWithRandomString(p.Title)
}

// PresenterById builds a presenter from a page row looked up by integer id.
func PresenterById(paramId string) (presenter Presenter, err error) {
	id, err := strconv.ParseInt(strings.TrimSpace(paramId), 10, 64)
	if err != nil {
		return presenter, serr.Wrap(err, "Could not convert paramId to int", "when", "building page Presenter")
	}
	m, err := findPageById(id)
	if err != nil {
		return presenter, serr.Wrap(err, "Unable to obtain page", "id", paramId)
	}
	return presenterFromModel(m)
}

// findPageByIdOrCreate: silent-fallback contract (same as other resources)
// returns a zero-valued model when the id is empty/invalid/missing — the
// caller uses m.ID < 1 to decide create vs update.
func findPageByIdOrCreate(id string) *model.Page {
	if id == "" {
		return &model.Page{}
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		logger.LogErr(err, "Unable to convert Page id to integer", "Id", id)
		return &model.Page{}
	}
	m, err := findPageById(intId)
	if err != nil || m == nil {
		return &model.Page{}
	}
	return m
}
