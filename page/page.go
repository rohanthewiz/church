package page
// A page is essentially an arrangement of modules
import (
	"strings"
	"github.com/rohanthewiz/church/chweb/module"
	"strconv"
)

// A Renderable Page - this does not directly map to the database (page_presenter does)
// - it is only for rendering
type Page struct {
	PresenterId    string
	Title          string
	slug           string
	CurrentPage    string  // for matching menu label
	modules        map[string][]module.Module
	positions      []string  // left, right, center, for now...
	layout         string
	mainModuleSlug string  // This is the one that receives any dynamic params like next Id
	IsAdmin        bool
}

func pageFromPresenter(pres Presenter) *Page {
	page := new(Page)
	page.PresenterId = pres.Id
	page.Title = pres.Title
	page.slug = pres.Slug
	page.IsAdmin = pres.IsAdmin
	page.SetPositions(pres.AvailablePositions)
	page.AddModules(pres.Modules)
	page.CurrentPage = strings.ToLower(pres.Title)  // todo: let's snakecase this or preferrably use page.Slug
	return page
}

func (p *Page) GetSlug() string {
	return p.slug
}

// Is the page information derived from the database (not hardwired)?
func (p *Page) IsDynamic() bool {
	i, err := strconv.ParseInt(p.PresenterId, 10, 64)
	return  err == nil && i > 0
}

func (p *Page) AddModule(mod module.Module, position string) {
	if p.positionValid(position) {
		p.modules[position] = append(p.modules[position], mod)
	}
	if mod.IsAdminModule() {
		p.IsAdmin = true
	}
	if mod.IsMainModule() {
		p.SetMainModule(mod)
	}
}

func (p *Page) SetMainModule(mod module.Module) {
	p.mainModuleSlug = mod.GetSlug()
}

func (p *Page) MainModuleSlug() string {
	return p.mainModuleSlug
}

func (p Page) Render(position string, params map[string]map[string]string, loggedIn bool) (outstr string) {
	for _, module := range p.modules[position] {
		if module.IsPublished() {
			outstr += module.Render(params, loggedIn)
		}
	}
	return
}

func (p Page) GetLayout() string {
	return p.layout
}

func (p Page) positionValid(position string) (valid bool) {
	for _, pos := range p.positions {
		if pos == position {
			return true
		}
	}
	return
}

// Establish layout
// positions default to ["center"]
func (p *Page) SetPositions(positions []string) {
	var has_left, has_right, has_center bool
	if len(positions) < 1 {
		positions = []string{"center"}
	}
	for _, pos := range positions {
		p.positions = append(p.positions, pos)
		switch pos {
		case "left":
			has_left = true
		case "center":
			has_center = true
		case "right":
			has_right = true
		}
	}
	if has_center && !has_left && !has_right {
		p.layout = "layout-main"
	} else if has_left && has_center && !has_right {
			p.layout = "layout-left-main"
	} else if has_left && has_center && has_right {
		p.layout = "layout-left-main-right"
	} else if !has_left && has_center && has_right {
		p.layout = "layout-main-right"
	}
}