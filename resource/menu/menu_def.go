package menu

import (
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/chweb/util/stringops"
	"github.com/rohanthewiz/serr"
	"github.com/rohanthewiz/church/models"
	"fmt"
	"strings"
	"encoding/json"
	"gopkg.in/nullbio/null.v6"
	"errors"
	"github.com/rohanthewiz/church/chweb/config"
)

// This is a interim struct that sits closer to the database
type MenuDef struct {
	Id        string
	CreatedAt string
	UpdatedAt string
	UpdatedBy string
	Title     string // label in parent menu
	Slug      string // guid - derived from title
	Published bool
	IsAdmin   bool
	Items     []MenuItemDef
}

// This will exist only as part of the Menu Presenter/Definition
type MenuItemDef struct {
	Label          string `json:"label"`
	Url            string `json:"url"`
	SubMenuSlug    string `json:"sub_menu_slug"`
}

func (m *MenuDef) CreateSlug() {
	if m.Title == "" { logger.Log("Warn", "Title should be set before Slug"); return }
	m.Slug = stringops.SlugWithRandomString(m.Title)
}


func menuDefFromSlug(slug string) (pres MenuDef, err error) {
	model, err := findModelBySlug(slug)
	if err != nil {
		return pres, serr.Wrap(err, "Error finding menuDef by slug")
	}
	pres, err = menuDefFromModel(model)
	if err != nil {
		return pres, serr.Wrap(err, "Error in menuDef from model")
	}
	return
}

func menuDefFromModel(model *models.MenuDef) (pres MenuDef, err error) {
	pres.Id = fmt.Sprintf("%d", model.ID)
	if model.CreatedAt.Valid {
		pres.CreatedAt = model.CreatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	if model.UpdatedAt.Valid {
		pres.UpdatedAt = model.UpdatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	pres.UpdatedBy = model.UpdatedBy
	pres.Published = model.Published
	pres.IsAdmin = model.IsAdmin
	pres.Title = model.Title
	pres.Slug = model.Slug

	menuItemDefs := []MenuItemDef{}
	model.Items.Unmarshal(&menuItemDefs)
	pres.Items = menuItemDefs

	return
}

func modelFromMenuDef(pres MenuDef) (model *models.MenuDef, create_op bool, err error) {
	model = findModelByIdOrCreate(pres.Id)
	if model.ID < 1 {
		create_op = true
	}

	if updatedBy := strings.TrimSpace(pres.UpdatedBy); updatedBy != "" {
		model.UpdatedBy = updatedBy
	}

	if title := strings.TrimSpace(pres.Title); title != "" {
		model.Title = title
	} else {
		er := serr.Wrap(errors.New("Menu title should not be blank"))
		return nil, create_op, er
	}
	if create_op {  // Allow slug update only on create to maintain external references
		pres.CreateSlug() // slug has to be unique only on the page
		model.Slug = pres.Slug  // todo: optimize
	}
	model.Published = pres.Published
	model.IsAdmin = pres.IsAdmin
	itemsAsJsonBytes, err := json.Marshal(pres.Items)
	if err != nil {
		return nil, create_op, serr.Wrap(err, "Error marshalling menuDef items")
	}
	model.Items = null.NewJSON(itemsAsJsonBytes, true)

	return
}
