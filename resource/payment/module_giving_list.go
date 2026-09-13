package payment

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// ModuleTypeGivingList is the read-only admin giving page (charges),
// hardwired into /admin/giving and gated by charges.read.
//
// Read-only on purpose: charges are written by the Stripe receipt redirect
// and webhook (payment_controller), and a local edit would drift from what
// Stripe actually settled. Corrections and refunds happen in Stripe.
//
// Layout: one calendar year at a time (default: year to date), because giving
// is reviewed and reported by year. The page reads top to bottom as:
//
//	[« 2025]  2026 Year to date  [2027 »]            [Export CSV]
//	┌ By month ─────────────────────────────────────────────────┐
//	│ Month      Gifts   Gross   Refunded   Net   (Jan → now)   │
//	│ Total                                                     │
//	└───────────────────────────────────────────────────────────┘
//	┌ September 2026 ─────────────── 12 gifts · $1,234.00 net ┐  newest month
//	│ one row per gift, newest first                          │  first; empty
//	└─────────────────────────────────────────────────────────┘  months skipped
//
// The summary is chronological like a ledger; the detail is newest first,
// because the current month is what's looked at most. The same year is
// exported by /admin/giving/csv (see payment_controller.AdminGivingCSVRWeb).
const ModuleTypeGivingList = "giving_list"

type ModuleGivingList struct {
	module.Presenter
}

func NewModuleGivingList(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleGivingList)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

// GetData loads the report for the requested year ("" = current year).
func (m ModuleGivingList) GetData(rawYear string) (GivingYear, error) {
	dbH, err := db.Db()
	if err != nil {
		return GivingYear{}, serr.Wrap(err, "Could not obtain DB handle")
	}
	return LoadGivingYear(dbH, rawYear, time.Now())
}

func (m *ModuleGivingList) Render(params map[string]map[string]string, loggedIn bool) string {
	rawYear := ""
	if opts, ok := params[m.Opts.Slug]; ok {
		rawYear = opts["year"]
	}
	gy, err := m.GetData(rawYear)
	if err != nil {
		logger.LogErr(err, "Error obtaining giving records")
		return ""
	}

	listURL := config.AdminPrefix + "/giving"
	yearURL := func(y int) string { return listURL + "?year=" + strconv.Itoa(y) }
	year := strconv.Itoa(gy.Year)

	period := "Full year"
	if gy.YTD() {
		period = "Year to date, through " + time.Now().Format("Jan 2")
	}

	b := element.NewBuilder()
	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading").T(m.Opts.Title),
		b.DivClass("af-wrap af-wrap--wide").R(
			// ---- Year navigation + export ----
			b.DivClass("af-toolbar").R(
				b.Nav("class", "af-yearnav", "aria-label", "Year").R(
					b.Wrap(func() {
						// Back stops at the first year with data (HasPrev), forward
						// at the current year (HasNext), so every link lands on a
						// year the report can show.
						if gy.HasPrev() {
							b.AClass("af-btn", "href", yearURL(gy.Year-1)).T("« " + strconv.Itoa(gy.Year-1))
						}
					}),
					b.SpanClass("af-yearnav__label").R(
						b.T(year),
						b.SpanClass("af-yearnav__period").T(period),
					),
					b.Wrap(func() {
						if gy.HasNext() {
							b.AClass("af-btn", "href", yearURL(gy.Year+1)).T(strconv.Itoa(gy.Year+1) + " »")
						}
						// A shortcut home when several years back
						if gy.Year < gy.CurrentYear-1 {
							b.AClass("af-btn", "href", listURL).T("Year to date")
						}
					}),
				),
				b.AClass("af-btn af-btn--primary", "href", listURL+"/csv?year="+year).T("Export CSV"),
			),
			b.PClass("af-help").T("Read-only. Giving records come from Stripe; refunds and corrections are made there. "+
				"Totals count paid gifts; Net subtracts refunds."),

			// ---- Summary by month ----
			b.DivClass("af-card").R(
				b.DivClass("af-card__title").R(
					b.Span().T("By month"),
					b.Span().T(dollars(gy.Totals.Net())+" net"),
				),
				b.DivClass("af-table-wrap").R(
					b.Table("class", "af-table").R(
						b.THead().R(
							b.Tr().R(
								b.Th().T("Month"),
								b.Th("class", "af-num").T("Gifts"),
								b.Th("class", "af-num").T("Gross"),
								b.Th("class", "af-num").T("Refunded"),
								b.Th("class", "af-num").T("Net"),
							),
						),
						b.TBody().R(
							b.Wrap(func() {
								for _, mo := range gy.Months {
									b.Tr().R(
										b.Td().R(
											b.Wrap(func() {
												// Months with gifts jump to their detail card
												if len(mo.Charges) > 0 {
													b.A("href", "#giving-"+mo.Key()).T(mo.Label())
												} else {
													b.T(mo.Label())
												}
											}),
										),
										b.Td("class", "af-num").T(strconv.Itoa(mo.Totals.Gifts)),
										b.Td("class", "af-num").T(dollars(mo.Totals.Gross)),
										b.Td("class", "af-num").T(dollars(mo.Totals.Refunded)),
										b.Td("class", "af-num").T(dollars(mo.Totals.Net())),
									)
								}
							}),
						),
						b.TFoot().R(
							b.Tr().R(
								b.Td().T("Total "+year),
								b.Td("class", "af-num").T(strconv.Itoa(gy.Totals.Gifts)),
								b.Td("class", "af-num").T(dollars(gy.Totals.Gross)),
								b.Td("class", "af-num").T(dollars(gy.Totals.Refunded)),
								b.Td("class", "af-num").T(dollars(gy.Totals.Net())),
							),
						),
					),
				),
				b.Wrap(func() {
					if gy.Totals.Pending > 0 {
						b.PClass("af-help").T(fmt.Sprintf("%s not yet paid %s listed below (greyed) but not counted.",
							plural(gy.Totals.Pending, "charge"), isAre(gy.Totals.Pending)))
					}
				}),
			),

			// ---- One card per month with gifts, newest month first ----
			b.Wrap(func() {
				if gy.Totals.Gifts+gy.Totals.Pending == 0 {
					b.PClass("af-help").T("No giving records for " + year + ".")
					return
				}
				for i := len(gy.Months) - 1; i >= 0; i-- {
					mo := gy.Months[i]
					if len(mo.Charges) == 0 {
						continue
					}
					renderGivingMonth(b, mo)
				}
			}),
		),
	)
	return b.String()
}

// renderGivingMonth writes one month's card: a subtotal heading and a row per
// charge, newest first. Donor-supplied text (name, email, description,
// comment) is HTML-escaped before T, which writes raw. html.EscapeString is
// used rather than element's TE because site binaries pin element releases
// that predate TE.
func renderGivingMonth(b *element.Builder, mo GivingMonth) {
	loc := mo.Start.Location()
	b.DivClass("af-card", "id", "giving-"+mo.Key()).R(
		b.DivClass("af-card__title").R(
			b.Span().T(mo.Label()),
			b.Span().T(plural(mo.Totals.Gifts, "gift")+" · "+dollars(mo.Totals.Net())+" net"),
		),
		b.DivClass("af-table-wrap").R(
			b.Table("class", "af-table").R(
				b.THead().R(
					b.Tr().R(
						b.Th().T("Date"),
						b.Th().T("Name"),
						b.Th().T("Email"),
						b.Th("class", "af-num").T("Amount"),
						b.Th().T("Status"),
						b.Th().T("Description"),
						b.Th().T("Comment"),
						b.Th().T("Receipt"),
					),
				),
				b.TBody().R(
					b.Wrap(func() {
						for i := len(mo.Charges) - 1; i >= 0; i-- {
							c := mo.Charges[i]
							rowAttrs := []string{}
							if !c.Paid.Bool {
								rowAttrs = append(rowAttrs, "class", "af-muted")
							}
							b.Tr(rowAttrs...).R(
								b.Td("style", "white-space:nowrap").T(c.CreatedAt.Time.In(loc).Format(config.DisplayDateTimeFormat)),
								b.Td().T(html.EscapeString(c.CustomerName)),
								b.Td("class", "af-wraptext").T(html.EscapeString(c.CustomerEmail.String)),
								b.Td("class", "af-num").T(dollars(c.AmountPaid.Int64)),
								b.Td().T(chargeStatus(c)),
								b.Td("class", "af-wraptext").T(html.EscapeString(c.Description.String)),
								b.Td("class", "af-wraptext").T(html.EscapeString(c.Comment.String)),
								b.Td().R(
									b.Wrap(func() { renderReceiptLink(b, c) }),
								),
							)
						}
					}),
				),
			),
		),
	)
}

// renderReceiptLink links Stripe's hosted receipt. The URL goes into an href,
// so only a plain https URL is linked; anything else (a javascript: URL, or a
// value with quotes or brackets) is shown as nothing rather than trusted.
func renderReceiptLink(b *element.Builder, c *models.Charge) {
	u := c.ReceiptURL.String
	if !c.ReceiptURL.Valid || !strings.HasPrefix(u, "https://") || strings.ContainsAny(u, "\"'<> ") {
		return
	}
	b.A("href", u, "target", "_blank", "rel", "noopener noreferrer").T("View")
}

// dollars formats cents with thousands separators ("123456" → "$1,234.56").
// Amounts are stored in cents (see ChargePresenter). The CSV export uses
// decimalCents instead, which spreadsheets read as a number.
func dollars(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	var sb strings.Builder
	for i, d := range whole {
		// A comma before every group of three digits, counted from the right
		if i > 0 && (len(whole)-i)%3 == 0 {
			sb.WriteByte(',')
		}
		sb.WriteRune(d)
	}
	return fmt.Sprintf("%s$%s.%02d", sign, sb.String(), cents%100)
}

// plural renders "1 gift" / "3 gifts".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

func isAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

// chargeStatus summarizes the settlement flags for the list.
func chargeStatus(c *models.Charge) string {
	switch {
	case c.Refunded.Bool && c.AmountRefunded.Int64 >= c.AmountPaid.Int64:
		return "refunded"
	case c.Refunded.Bool || c.AmountRefunded.Int64 > 0:
		return "partly refunded (" + dollars(c.AmountRefunded.Int64) + ")"
	case c.Paid.Bool:
		return "paid"
	default:
		return "pending"
	}
}
