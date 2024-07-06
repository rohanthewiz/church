package sermon

import (
	"errors"
	"fmt"
	strconv "strconv"
	"strings"

	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
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
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	ser, err := m.getData()
	if err != nil {
		LogErr(err, "Error in module render")
		return ""
	}

	b := element.NewBuilder()
	e := b.Ele
	t := b.Text

	e("h3", "class", "sermon-title").R(t(ser.Title))
	e("span", "class", "sermon-sub-title").R(
		t(ser.Teacher+" - "+ser.DateTaught),
		e("a", "class", "sermon-play-icon", "href", ser.AudioLink).R(t("download")))
	e("div").R(t(ser.Summary))
	e("div").R(t(ser.Body))
	if loggedIn && len(m.Opts.ItemIds) > 0 {
		e("a", "class", "edit-link", "href", m.GetEditURL()+
			strconv.FormatInt(m.Opts.ItemIds[0], 10)).R(
			e("img", "class", "edit-icon", "title", "Edit Sermon", "src", "/assets/images/edit_article.svg").R(),
		)
	}
	e("div", "class", "sermon-footer").R(
		e("span", "class", "scripture").R(t(strings.Join(ser.ScriptureRefs, ", "))),
		e("span", "class", "categories").R(t(strings.Join(ser.Categories, ", "))),
	)

	return b.String()
}
