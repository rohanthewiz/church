package easy_tabs

import (
	"fmt"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/article"
	"github.com/rohanthewiz/church/util/stringops"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
)

const ModuleTypeEasyTabs = "easy-tabs"

type ModuleEasyTabs struct {
	module.Presenter
}

func NewModuleEasyTabs(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleEasyTabs)
	mod.Name = pres.Name
	mod.Opts = pres.Opts

	// Work out local condition
	cond := "1 = 1"
	if !mod.Opts.IsAdmin && !mod.Opts.ShowUnpublished{
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
func (m ModuleEasyTabs) getData() ([]article.Presenter, error) {
	if len(m.Opts.ItemIds) > 0 {
		//fmt.Println("*|* About to run presentersFromIds", "m.Opts.ItemIds", m.Opts.ItemIds)
		return article.PresentersFromIds(m.Opts.ItemIds)
	}
	return article.QueryArticles(m.Opts.Condition, "updated_at " + m.Order(), m.Opts.Limit, m.Opts.Offset)
}

func (m *ModuleEasyTabs) Render(params map[string]map[string]string, loggedIn bool) (out string) {
	if opts, ok := params[m.Opts.Slug]; ok {  // params addressed to this module
		m.SetLimitAndOffset(opts)
	}
	articles, err := m.getData()
	if err != nil {
		logger.LogErr(err, "Error obtaining article data for EasyTabs module", "module_options",  fmt.Sprintf("%#v", m.Opts))
		return
	}

	if len(articles) > 0 {
		e := element.New
		out = e("div", "class", "ch-module-wrapper ch-" + ModuleTypeEasyTabs).R(
			e("ul", "class", "tabs").R(
				func() (str string) {
					for _, art := range articles {
						str += e("li").R(
							e("a", "href", "#" + stringops.Slugify(art.Summary)).R(art.Summary), // Put the tab id in the article summary
						)
					}
					return
				}(),
			),
			func() (str string) {
				for _, art := range articles {
					str += e("div", "id", stringops.Slugify(art.Summary)).R(art.Body)
				}
				return
			}(),
		)
		out +=	e("script", "type", "text/javascript").R(
			`(function ($) {
            $.fn.easyTabs = function (option) {
                var param = jQuery.extend({fadeSpeed: "fast", defaultContent: 1, activeClass: 'active'}, option);
                $(this).each(function () {
                    var thisId = "#" + this.id;
                    if (param.defaultContent == '') {
                        param.defaultContent = 1;
                    }
                    if (typeof param.defaultContent == "number") {
                        var defaultTab = $(thisId + " .tabs li:eq(" + (param.defaultContent - 1) + ") a").attr('href').substr(1);
                    } else {
                        var defaultTab = param.defaultContent;
                    }
                    $(thisId + " .tabs li a").each(function () {
                        var tabToHide = $(this).attr('href').substr(1);
                        $("#" + tabToHide).addClass('easytabs-tab-content');
                    });
                    hideAll();
                    changeContent(defaultTab);

                    function hideAll() {
                        $(thisId + " .easytabs-tab-content").hide();
                    }

                    function changeContent(tabId) {
                        hideAll();
                        $(thisId + " .tabs li").removeClass(param.activeClass);
                        $(thisId + " .tabs li a[href=#" + tabId + "]").closest('li').addClass(param.activeClass);
                        if (param.fadeSpeed != "none") {
                            $(thisId + " #" + tabId).fadeIn(param.fadeSpeed);
                        } else {
                            $(thisId + " #" + tabId).show();
                        }
                    }

                    $(thisId + " .tabs li").click(function () {
                        var tabId = $(this).find('a').attr('href').substr(1);
                        changeContent(tabId);
                        return false;
                    });
                });
            }
        })(jQuery);`,
	)}

	return
}