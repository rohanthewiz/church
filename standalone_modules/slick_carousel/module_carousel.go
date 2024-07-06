package slick_carousel

import (
	"fmt"

	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/article"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
)

const ModuleTypeSlickCarousel = "carousel"

type ModuleSlickCarousel struct {
	module.Presenter
}

func NewModuleSlickCarousel(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleSlickCarousel)
	mod.Name = pres.Name
	mod.Opts = pres.Opts

	// Work out local condition
	cond := "1 = 1"
	if !mod.Opts.IsAdmin && !mod.Opts.ShowUnpublished {
		cond = "published = true"
	}
	// merge with any incoming condition
	if mod.Opts.Condition != "" {
		cond = mod.Opts.Condition + " AND " + cond
	}
	mod.Opts.Condition = cond

	return module.Module(mod), nil
}

// Opts.ItemIds take precedence over other parameters
func (m ModuleSlickCarousel) getData() ([]article.Presenter, error) {
	if len(m.Opts.ItemIds) > 0 {
		// fmt.Println("*|* About to run presentersFromIds", "m.Opts.ItemIds", m.Opts.ItemIds)
		return article.PresentersFromIds(m.Opts.ItemIds)
	}
	return article.QueryArticles(m.Opts.Condition, "updated_at "+m.Order(), m.Opts.Limit, m.Opts.Offset)
}

func (m *ModuleSlickCarousel) Render(params map[string]map[string]string, loggedIn bool) (out string) {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to this module
		m.SetLimitAndOffset(opts)
	}
	articles, err := m.getData()
	if err != nil {
		logger.LogErr(err, "Error obtaining article data for Carousel module", "module_options", fmt.Sprintf("%#v", m.Opts))
		return
	}
	if len(articles) > 0 {
		b := element.NewBuilder()
		e := b.E
		t := b.Text

		e("div", "class", "ch-module-wrapper ch-"+ModuleTypeSlickCarousel).R(
			func() string {
				var str string
				for _, art := range articles {
					e("div", "class", "ch-carousel-item").R(
						t(art.Body))
				}
				return str
			}(),
		)

		// Todo - go ahead and move these to document head
		// out +=	e("script", "type", "text/javascript", "src", "https://code.jquery.com/jquery-2.1.3.min.js").R()
		// out +=	e("script", "type", "text/javascript", "src", "https://cdnjs.cloudflare.com/ajax/libs/slick-carousel/1.8.1/slick.min.js").R()
		e("script", "type", "text/javascript").R(
			t(`$(document).ready(function(){
				$('.ch-` + ModuleTypeSlickCarousel + `').slick({
					dots: true,
					infinite: true,
					autoplay: true,
					speed: 1200,
					fade: true,
					cssEase: 'ease-out',
					autoplaySpeed: 11000,
					slidesToShow: 1,
					slidesToScroll: 1
				});
			});`),
		)
	}

	return
}
