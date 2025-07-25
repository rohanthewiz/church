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

	b.H3Class("sermon-title").T(ser.Title)
	b.SpanClass("sermon-sub-title").R(
		b.T(ser.Teacher + " - " + ser.DateTaught),
	)
	// Mobile-friendly audio player with controls
	b.DivClass("sermon-audio-wrapper", "style", "margin: 20px 0; padding: 15px; background-color: #f5f5f5; border-radius: 8px;").R(
		b.Audio("controls", "controls", "style", "width: 100%; max-width: 600px;").R(
			b.Source("src", ser.AudioLink, "type", "audio/mpeg").R(),
			b.T("Your browser does not support the audio element."),
		),
		b.DivClass("sermon-download-link", "style", "margin-top: 10px;").R(
			b.A("href", ser.AudioLink, "title", "download", "style", "color: #333; text-decoration: none; font-size: 14px;").T("📥 Download Audio"),
		),
	)
	b.Div().T(ser.Summary)
	b.Div().T(ser.Body)
	b.Wrap(func() {
		if loggedIn && len(m.Opts.ItemIds) > 0 {
			b.AClass("edit-link", "href", m.GetEditURL()+
				strconv.FormatInt(m.Opts.ItemIds[0], 10)).R(
				b.ImgClass("edit-icon", "title", "Edit Sermon", "src", "/assets/images/edit_article.svg").R(),
			)
		}
	})
	b.DivClass("sermon-footer").R(
		b.SpanClass("scripture").T(strings.Join(ser.ScriptureRefs, ", ")),
		b.SpanClass("categories").T(strings.Join(ser.Categories, ", ")),
	)

	return b.String()
}
