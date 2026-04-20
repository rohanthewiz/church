package page

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/serr"
)

func PageFromId(id string) (*Page, error) {
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, serr.Wrap(err, "Error converting page id to int")
	}
	m, err := findPageById(intId)
	if err != nil {
		return nil, err
	}
	pres, err := presenterFromModel(m)
	if err != nil {
		return nil, serr.Wrap(err, "Error in page model to presenter")
	}
	return pageFromPresenter(pres), nil
}

func PageFromSlug(slug string) (pg *Page, err error) {
	pres, err := presenterFromSlug(slug)
	if err != nil {
		return pg, err
	}
	return pageFromPresenter(pres), nil
}

func presenterFromSlug(slug string) (pres Presenter, err error) {
	m, err := findPageBySlug(slug)
	if err != nil {
		return pres, serr.Wrap(err, "Error finding page by slug")
	}
	pres, err = presenterFromModel(m)
	if err != nil {
		return pres, serr.Wrap(err, "Error in page model to presenter")
	}
	return
}

// presenterFromModel expands the DB row into the view-layer Presenter.
// The Data column is jsonb; nil when NULL. Decoding happens here (not in the
// model package) because module.Presenter lives in a higher layer and we want
// model/ free of that dependency.
func presenterFromModel(m *model.Page) (pres Presenter, err error) {
	pres.Id = fmt.Sprintf("%d", m.ID)
	if m.CreatedAt.Valid {
		pres.CreatedAt = m.CreatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	if m.UpdatedAt.Valid {
		pres.UpdatedAt = m.UpdatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	pres.UpdatedBy = m.UpdatedBy
	pres.Published = m.Published
	pres.IsHome = m.IsHome
	pres.IsAdmin = m.IsAdmin
	pres.Title = m.Title
	pres.Slug = m.Slug

	pres.AvailablePositions = []string(m.AvailablePositions)

	modPresenters := []module.Presenter{}
	if len(m.Data) > 0 {
		if err = json.Unmarshal(m.Data, &modPresenters); err != nil {
			return pres, serr.Wrap(err, "Error unmarshalling page modules")
		}
	}
	pres.Modules = modPresenters
	return
}

// modelFromPresenter prepares the DB row from the presenter. Slug is write-
// once on create to preserve external references; AvailablePositions is
// rebuilt (not appended) so the column doesn't accumulate stale entries.
func modelFromPresenter(pres Presenter) (m *model.Page, create_op bool, err error) {
	m = findPageByIdOrCreate(pres.Id)
	if m.ID < 1 {
		create_op = true
	}

	if updatedBy := strings.TrimSpace(pres.UpdatedBy); updatedBy != "" {
		m.UpdatedBy = updatedBy
	}

	title := strings.TrimSpace(pres.Title)
	if title == "" {
		return nil, create_op, serr.New("Page title should not be blank")
	}
	m.Title = title

	if create_op {
		pres.CreateSlug()
		m.Slug = pres.Slug
	}
	m.Published = pres.Published
	m.IsAdmin = pres.IsAdmin
	m.IsHome = pres.IsHome

	availPos := []string{}
	for _, pos := range pres.AvailablePositions {
		if trimmed := strings.TrimSpace(pos); trimmed != "" {
			availPos = append(availPos, trimmed)
		}
	}
	m.AvailablePositions = availPos

	modulesAsJsonBytes, err := json.Marshal(pres.Modules)
	if err != nil {
		return nil, create_op, serr.Wrap(err, "Error marshalling page presenter modules")
	}
	m.Data = modulesAsJsonBytes
	return
}
