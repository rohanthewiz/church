package payment

import (
	"encoding/json"
	"strings"

	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
)

const ModuleTypePaymentReceipt = "payment_receipt"

// ReceiptMeta is what the payment controller passes to this module through
// module Opts.Meta (as JSON). For back-compat, Meta may also be a bare
// receipt-URL string — parseReceiptMeta accepts both.
type ReceiptMeta struct {
	URL    string `json:"url"`
	Amount string `json:"amount"` // display string, e.g. "$25.00"
	Date   string `json:"date"`   // display string, e.g. "August 1, 2026"
	Failed bool   `json:"failed"` // Stripe redirect_status=failed — no gift happened
}

func parseReceiptMeta(meta string) (rm ReceiptMeta) {
	meta = strings.TrimSpace(meta)
	if strings.HasPrefix(meta, "{") {
		if err := json.Unmarshal([]byte(meta), &rm); err == nil {
			return rm
		}
	}
	rm.URL = meta
	return rm
}

type ModulePaymentReceipt struct {
	module.Presenter
}

func NewModulePaymentReceipt(pres module.Presenter) (module.Module, error) {
	mod := new(ModulePaymentReceipt)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return mod, nil
}

func (m ModulePaymentReceipt) Render(params map[string]map[string]string, loggerIn bool) (out string) {
	rm := parseReceiptMeta(m.Opts.Meta)

	b := element.NewBuilder()

	if rm.Failed {
		b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
			b.H3Class("article-title").T("Your payment was not completed"),
			b.PClass("receipt-info").T("No charge was made. This can happen when a card is declined or the payment was canceled."),
			b.PClass("receipt-info").R(
				b.A("href", "/payments/new").T("Try again"),
				b.T(" or "),
				b.A("href", "/").T("return to the home page"),
			),
		)
		return b.String()
	}

	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.H3Class("article-title").T("Thank you for your gift!"),
		b.Wrap(func() {
			// State the gift itself, not just a link — for many givers this
			// page is the only confirmation they read.
			if rm.Amount != "" {
				line := "Your gift of " + rm.Amount
				if rm.Date != "" {
					line += " on " + rm.Date
				}
				line += " was received."
				b.PClass("receipt-summary").T(line)
			}
			if rm.URL != "" {
				b.PClass("receipt-info").R(
					b.T("Your Stripe receipt is available"),
					b.A("href", rm.URL, "target", "_blank", "rel", "noopener").T(" here"),
					b.T(". Please save a copy for your records."),
				)
			} else {
				b.PClass("receipt-info").T("A receipt has been emailed to you.")
			}
		}),
		b.PClass("receipt-info").R(
			b.A("href", "/").T("Return to the home page"),
		),
	)
	return b.String()
}
