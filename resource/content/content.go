package content

import (
	"github.com/rohanthewiz/church/util/stringops"
)

type Content struct {
	Id string
	Title string
	Slug string
	Summary string
	Body string
	Published bool
	Categories []string
	CreatedAt string
	UpdatedAt string
	UpdatedBy string
}

func (c * Content) CreateSlug() {
	c.Slug = stringops.SlugWithRandomString(c.Title)
}

type ModuleContentType string // How to filter module content

const (
	ModuleContentByForm       ModuleContentType = "Form"
	ModuleContentBySingleId   ModuleContentType = "SingleId"
	ModuleContentByMultiId    ModuleContentType = "MultiId"
	ModuleContentByPagination ModuleContentType = "Pagination"
)