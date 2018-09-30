package menu

var MainMenuRender string

func MainMenu() Menu {
	about_submenu := Menu{}
	about_submenu.Items = append(about_submenu.Items, MenuItem{ Label: "About Sub1", Url: "http://example.com/1" })
	about_submenu.Items = append(about_submenu.Items, MenuItem{ Label: "About Sub2", Url: "http://example.com/2" })

	menu := Menu{}
	menu.Items = append(menu.Items, MenuItem{Label: "Home", Url: "/"})
	menu.Items = append(menu.Items, MenuItem{Label: "Ministries", Url: "/"})
	menu.Items = append(menu.Items, MenuItem{Label: "Distinctives", Url: "/"})
	menu.Items = append(menu.Items, MenuItem{Label: "Contact", Url: "/"})
	menu.Items = append(menu.Items,
		MenuItem{
			Label: "About", Url: "/pages/about",
			SubMenu: &about_submenu,
		})
	//menu.Items = append(menu.Items, MenuItem{Label: "MainThree", Url: "http://example.com/3"})

	return menu
}

func FooterMenu() Menu {
	menu := Menu{}
	menu.Items = append(menu.Items, MenuItem{Label: "About Us", Url: "/pages/about"})
	menu.Items = append(menu.Items, MenuItem{Label: "Privacy", Url: "/pages/privacy"})
	menu.Items = append(menu.Items, MenuItem{Label: "Login", Url: "/login"})
	return menu
}
