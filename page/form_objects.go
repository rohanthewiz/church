package page

import (
	"github.com/rohanthewiz/church/chweb/module"
	"encoding/json"
	"strings"
	"github.com/rohanthewiz/logger"
	"strconv"
)

// Structures for directly interfacing with page forms

// This object is for unmarshalling form data produced by the JavaScript serializer
// The serializer serializes the whole form, but we are only interested in Modules here
type formPageObject struct {
	//PageId             string           `json:"page_id"`
	//PageTitle          string           `json:"page_title"`
	//AvailablePositions string           `json:"available_positions"`
	//MainModuleSlug     string           `json:"main_module_slug"`
	//Published          bool             `json:"published"`
	Modules            []ModuleReceiver `json:"mods"`
	//Admin              bool             `json:"admin"`
}

type ModuleReceiver struct {
	Title           string `json:"title"`
	//Slug            string `json:"slug"` // If slug is empty, it will be created at the resource level (modelFromPresenter)
	ModuleType      string `json:"module_type"`
	IsAdmin         bool   `json:"is_admin"`
	IsMainModule    bool   `json:"main_module"`
	Published       bool   `json:"published"`
	LayoutColumn    string `json:"layout_column"`
	ItemsURLPath    string `json:"items_url_path"`
	//Condition       string `json:"condition"`
	ItemIds         string `json:"item_ids"`
	ItemSlug        string `json:"item_slug"`
	Limit           string `json:"limit"`
	Offset          string `json:"offset"` // we'll conv these to int64
	ShowUnpublished bool   `json:"show_unpublished"`
	Ascending       bool   `json:"ascending"`
}

func ModulePresentersFromJson(formJson string) (modPresenter []module.Presenter) {
	form := formPageObject{}
	err := json.Unmarshal([]byte(formJson), &form)
	if err != nil { return }
	println("|* Num of modules:", len(form.Modules))

	for _, mod := range form.Modules {
		modPres := module.Presenter{}

		ids := []int64{}
		arrIds := strings.Split(mod.ItemIds, ",")
		for _, strId := range arrIds {
			if trimmedId := strings.TrimSpace(strId); trimmedId != "" {
				id, err := strconv.ParseInt(trimmedId, 10, 64)
				if err != nil {
					logger.LogErr(err, "Error converting id to int64", "id", "strId")
				} else {
					ids = append(ids, id)
				}
			}
		}
		var limit int64
		if trimmedLimit := strings.TrimSpace(mod.Limit); trimmedLimit != "" {
			limit, err = strconv.ParseInt(trimmedLimit, 10, 64)
			if err != nil {
				logger.LogErrAsync(err, "Error converting limit to int64", "limit", mod.Limit)
				limit = 0
			}
		}
		var offset int64
		if trimmedOffset := strings.TrimSpace(mod.Offset); trimmedOffset != "" {
			offset, err = strconv.ParseInt(trimmedOffset, 10, 64)
			if err != nil {
				logger.LogErrAsync(err, "Error converting offset to int64", "offset", mod.Offset)
				offset = 0
			}
		}
		title := strings.TrimSpace(mod.Title)
		// mslug := strings.TrimSpace(mod.Slug)
		// if mslug == "" { mslug = stringops.SlugWithRandomString(title) }
		// fmt.Println("*|* mod.ItemIds", mod.ItemIds)
		modPres.Opts = module.Opts{
			Title:           title,
			//Slug:            mslug,
			ModuleType:      strings.TrimSpace(mod.ModuleType),
			IsAdmin:         false, // dynamic pages can't be admin //mod.IsAdmin,
			Published:       mod.Published,
			IsMainModule:    mod.IsMainModule,
			LayoutColumn:    strings.TrimSpace(mod.LayoutColumn),
			ItemsURLPath:    strings.TrimSpace(mod.ItemsURLPath),
			ItemIds:         ids,
			ItemSlug:        strings.TrimSpace(mod.ItemSlug),
			//Condition:       strings.TrimSpace(mod.Condition),
			Limit:           limit,
			Offset:          offset,
			ShowUnpublished: mod.ShowUnpublished,
			Ascending:       mod.Ascending,
		}
		modPresenter = append(modPresenter, modPres)
	}
	return
}
