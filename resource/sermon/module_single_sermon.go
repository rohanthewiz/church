package sermon

import (
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/module"
	"strings"
	"github.com/rohanthewiz/serr"
	"fmt"
	"errors"
	"github.com/rohanthewiz/element"
	strconv "strconv"
)

const ModuleTypeSingleSermon = "sermon_single"

type ModuleSingleSermon struct {
	module.Presenter
}

func NewModuleSingleSermon(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleSingleSermon)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

func (m ModuleSingleSermon) getData() (pres Presenter, err error) {
	// If the module instance has an item slug defined, it takes highest precedence
	if m.Opts.ItemSlug != "" {
		pres, err = PresenterFromSlug(m.Opts.ItemSlug)
	} else {
		if len(m.Opts.ItemIds) < 1 {
			return pres, serr.Wrap(errors.New("No item ids found"),
				"module_options", fmt.Sprintf("%#v", m.Opts))
		}
		pres, err = presenterFromId(m.Opts.ItemIds[0])
	}
	return
}

func (m *ModuleSingleSermon) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to us
		m.SetId(opts)
	}
	ser, err := m.getData()
	if err != nil {
		LogErr(err, "Error in module render")
		return ""
	}
	e := element.New
	out := e("h3", "class", "sermon-title").R(ser.Title)
	out += e("span", "class", "sermon-sub-title").R(
		ser.Teacher + " - " + ser.DateTaught,
		e("a", "class", "sermon-play-icon", "href", ser.AudioLink).R("download"))
	out += e("div").R(ser.Summary)
	out += e("div").R(ser.Body)
	if loggedIn && len(m.Opts.ItemIds) > 0 {
		out += e("a", "class", "edit-link", "href", m.GetEditURL() +
			strconv.FormatInt(m.Opts.ItemIds[0], 10)).R(
			e("img", "class", "edit-icon", "title", "Edit Sermon", "src", "/assets/images/edit_article.svg").R(),
		)
	}
	out += e("div", "class", "sermon-footer").R(
		e("span", "class", "scripture").R(strings.Join(ser.ScriptureRefs, ", ")),
		e("span", "class", "categories").R(strings.Join(ser.Categories, ", ")),
	)
	return out
}