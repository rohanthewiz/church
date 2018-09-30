package module

// A Renderable collection of items

type Module interface {
	Render(params map[string]map[string]string, loggedIn bool) string
	IsAdminModule() bool
	IsMainModule() bool
	IsPublished() bool
	GetSlug() string
}
