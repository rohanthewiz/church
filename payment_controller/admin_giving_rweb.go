package payment_controller

import (
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/rweb"
)

// AdminListGivingRWeb renders the read-only giving records list
// (GET /admin/giving, charges.read — see router_rweb.go).
func AdminListGivingRWeb(ctx rweb.Context) error {
	pg, err := page.GivingList()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageListRWeb(pg, ctx)))
}
