package menu

// Structures for directly interfacing with menu forms

// This object is for unmarshalling form data produced by the JavaScript serializer
// The serializer serializes the whole form, but we are only interested in Modules here
type FormMenuObject struct {
	//MenuId             string           `json:"menu_id"`
	//MenuTitle          string           `json:"menu_title"`
	Items []ItemReceiver `json:"items"`
}

type ItemReceiver struct {
	Label          string `json:"label"`
	Url            string `json:"url"`
	ParentMenuSlug string `json:"parent_menu_slug"`
	SubMenuSlug    string `json:"sub_menu_slug"`
}
