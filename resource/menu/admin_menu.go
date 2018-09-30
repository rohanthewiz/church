package menu

func AdminMenu() []MenuItem {
	menu := []MenuItem{}
	menu = append(menu, MenuItem{Label: "Users", Url: "/admin/users"})
	menu = append(menu, MenuItem{Label: "AdminTwo", Url: "http://example.com/2"})
	menu = append(menu, MenuItem{Label: "AdminThree", Url: "http://example.com/3"})
	return append(menu, MenuItem{Label: "Logout", Url: "/admin/logout"})
}
