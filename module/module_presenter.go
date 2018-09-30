package module
// A Module is a renderable unit within a page.
// Modules will become instantiated based on an existing module initializer function
// in the module registry when it's page object is initialized.
// Modules cannot exist outside of pages. Modules are not persisted,
// but are created according to the parent page definition.

import (
	"math"
	"fmt"
	"bytes"
	"strconv"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/config"
)

// This is the module 'base class'
// Presenter satisfies most methods in the Module interface
type Presenter struct {
	//Id           string  // modules are not persisted
	Name Name // singular and plural forms of module name
	Opts Opts `json:"opts"` // Opts contains options for instantiating a module
}

// Singular and Plural forms of Module name
type Name struct{ Singular, Plural string }

// Options for instantiating a module in a page
// The ModuleType determines how options are used esp. in Rendering
type Opts struct {
	ModuleType      string  `json:"module_type"` // hyphenated module type name eg. "upcoming-events"
	Title           string  `json:"title"`
	Slug            string  `json:"slug"` // unique idfr - generated on initial creation
	Published       bool    `json:"published"`
	LayoutColumn    string  `json:"layout_column"`
	IsAdmin         bool    `json:"is_admin"`       // admin modules will make the page admin and require login
	IsMainModule    bool    `json:"is_main_module"` // is this the main module on a page (there can be only one :-))
	ItemsURLPath    string  `json:"items_url_path"` // path prefix for items
	ItemIds         []int64 `json:"item_ids"`       // id for single item, else len(0) for other parameters/conditions. If form 0 - new, > 0 - edit
	ItemSlug        string  `json:"item_slug"`      // slug for a single item - mainly (only?) for a prebuilt item
	Condition       string  `json:"condition"`
	Limit           int64   `json:"limit"` // how many - only needed for multiple
	Offset          int64   `json:"offset"`
	ShowUnpublished bool    `json:"show_unpublished"` // only admin can show unpublished
	Ascending       bool    `json:"ascending"`        // false - normally descending - only needed for multiple
	IsLoggedIn	bool `json:"-"`
}

func (m Presenter) IsAdminModule() bool {
	return m.Opts.IsAdmin
}

func (m Presenter) IsMainModule() bool {
	return m.Opts.IsMainModule
}

func (m Presenter) IsPublished() bool {
	return m.Opts.Published
}

func (m Presenter) GetSlug() string {
	return m.Opts.Slug
}

// Returns the current direction suitable for an SQL query
func (m Presenter) Order() string {
	direction := "DESC"
	if m.Opts.Ascending {
		direction = "ASC"
	}
	return direction
}

func (m *Presenter) SetId(opts map[string]string) {
	logger.LogAsync("Debug", "Params to Form module:", "params", fmt.Sprintf("%#v", opts))
	if id, ok := opts["id"]; ok {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err == nil {
			m.Opts.ItemIds = []int64{intId}
		}
	}
}

func (m *Presenter) SetLimitAndOffset(opts map[string]string) {
	if id, ok := opts["limit"]; ok {
		limit, err := strconv.ParseInt(id, 10, 64)
		if err == nil {
			m.Opts.Limit = limit
		}
	}
	if id, ok := opts["offset"]; ok {
		offset, err := strconv.ParseInt(id, 10, 64)
		if err == nil {
			m.Opts.Offset = offset
		}
	}
}

func (m Presenter) GetEditURL() (url string) {
	return config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/edit/"
}

func (m Presenter) GetDeleteURL() (url string) {
	return config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/delete/"
}

func (m Presenter) GetNewURL() (url string) {
	return config.AdminPrefix + "/" + m.Opts.ItemsURLPath + "/new"
}

func (m Presenter) RenderPagination(itemsLen int) string {
	out := new(bytes.Buffer)
	page_num := int64(math.Floor(float64(m.Opts.Offset)/float64(m.Opts.Limit))) + 1

	if m.Opts.Offset > 0 {
		prev_offset := m.Opts.Offset - m.Opts.Limit
		if prev_offset < 0 {
			prev_offset = 0
		}
		prev := `<a href="/` + m.Opts.ItemsURLPath + `?limit=` + fmt.Sprintf("%d", m.Opts.Limit) +
			`&offset=` + fmt.Sprintf("%d", prev_offset) + `">Prev</a>`
		out.WriteString(`<div class="pagination"><span>` + prev + ` | </span>`)
	}
	out.WriteString(`<span>Page `)
	out.WriteString(fmt.Sprintf("%d", page_num))
	out.WriteString("</span>")

	if m.Opts.Limit > 0 && itemsLen == int(m.Opts.Limit) { // if we haven't run out, keep going
		next_ofst := m.Opts.Offset + m.Opts.Limit
		next := `<a href="/` + m.Opts.ItemsURLPath + `?limit=` + fmt.Sprintf("%d", m.Opts.Limit) +
			`&offset=` + fmt.Sprintf("%d", next_ofst) + `">Next</a>`
		out.WriteString(`<span> | ` + next + `</span></div>`)
	}

	return out.String()
}
