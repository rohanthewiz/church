package payment

// Giving-domain constants and helpers shared by both HTTP layers — the web
// giving form (payment_controller) and the mobile JSON API (api_rweb.go in
// this package). They lived in payment_controller until the mobile endpoint
// needed them; controllers may import resources but never the reverse, so the
// shared home has to be here.

import (
	"strings"

	"github.com/rohanthewiz/church/config"
)

// MinChargeCents is Stripe's documented minimum charge for USD, in cents.
// Validated server-side because any client-side minimum (the form's min
// attribute, the app's input validation) is advisory only.
const MinChargeCents = 50

// TxDescription labels the charge in Stripe's dashboard and on receipts.
// Resolved at call time (not a package const) because this framework serves
// multiple church sites from one binary family -- the old hardcoded
// "CCSWM Donation" const was branding every site's gifts with one church's name.
func TxDescription() string {
	if desc := strings.TrimSpace(config.Options.Stripe.TxDescription); desc != "" {
		return desc
	}
	if owner := strings.TrimSpace(config.Options.CopyrightOwner); owner != "" {
		return owner + " Donation"
	}
	return "Donation"
}
