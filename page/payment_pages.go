package page

import (
	"github.com/rohanthewiz/church/resource/payment"
	"github.com/rohanthewiz/church/util/stringops"
	"github.com/rohanthewiz/church/module"
)

func PaymentForm() (*Page, error) {
	title := "Payment Form"
	pgdef := Presenter{
		Title: title, Slug: stringops.Slugify(title),
		IsAdmin: true,
		AvailablePositions: []string{"center"}, //, "right"
	}
	modulePres1 := module.Presenter{
		Opts: module.Opts{
			Title:      "Show Payment",
			ModuleType: payment.ModuleTypePaymentForm,
			IsAdmin:    true,
			Published:    true,
			IsMainModule: true,
			//LayoutColumn: "center",
			//ItemId:     1, // Item id is passed via URL Param
		},
	}
	//modulePres2 := module.Presenter{
	//	Opts: module.Opts{
	//		Title:      "Recent Payments",
	//		ModuleType: payment.ModuleTypeRecentPayments,
	//		Published:    true,
	//		LayoutColumn: "right",
	//		Limit: 8,
	//	},
	//}
	pgdef.Modules = []module.Presenter{modulePres1} //, modulePres2
	return pageFromPresenter(pgdef), nil
}

//func PaymentsList() (*Page, error) {
//	title := "Payments List"
//	pgdef := Presenter{Title: title, Slug: stringops.Slugify(title), IsAdmin: true}
//	modPres := module.Presenter{
//		Opts: module.Opts{
//			Title: "Payments List",
//			ModuleType: payment.ModuleTypePaymentsList,
//			IsAdmin: true,
//			Published: true,
//			IsMainModule: true,
//			Limit: 25,
//		},
//	}
//	pgdef.Modules = []module.Presenter{modPres}
//	return  pageFromPresenter(pgdef), nil
//}
