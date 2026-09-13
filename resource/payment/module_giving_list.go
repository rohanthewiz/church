package payment

import (
	"fmt"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/grid"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	"github.com/vattle/sqlboiler/queries/qm"
)

// ModuleTypeGivingList is the read-only admin list of giving records
// (charges), hardwired into /admin/giving and gated by charges.read.
//
// Read-only on purpose: charges are written by the Stripe receipt redirect
// and webhook (payment_controller), and a local edit would drift from what
// Stripe actually settled. Corrections and refunds happen in Stripe.
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

// GetData loads one server page of charges, newest first.
func (m ModuleGivingList) GetData() (models.ChargeSlice, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Could not obtain DB handle")
	}
	limit := m.Opts.Limit
	if limit <= 0 {
		limit = 50
	}
	charges, err := models.Charges(dbH, qm.OrderBy("created_at DESC"),
		qm.Limit(int(limit)), qm.Offset(int(m.Opts.Offset))).All()
	if err != nil {
		return nil, serr.Wrap(err, "Error querying charges")
	}
	return charges, nil
}

func (m *ModuleGivingList) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok {
		m.SetLimitAndOffset(opts)
	}
	charges, err := m.GetData()
	if err != nil {
		logger.LogErr(err, "Error obtaining giving records")
		return ""
	}

	g := grid.Grid{
		Class:        "giving-list-grid",
		EmptyMessage: "No giving records yet",
		Limit:        m.Opts.Limit,
		Offset:       m.Opts.Offset,
		Columns: []grid.Column{
			{Header: "Date", Type: grid.ColDate, GroupBy: true},
			{Header: "Name"},
			{Header: "Email", Popup: true},
			{Header: "Amount", Type: grid.ColNum, Shrink: true},
			{Header: "Status", Shrink: true},
			{Header: "Description", Popup: true},
			{Header: "Comment", Popup: true},
			{Header: "Receipt", NoSort: true, NoFilter: true, Shrink: true},
		},
	}

	for _, c := range charges {
		date := ""
		if c.CreatedAt.Valid {
			date = c.CreatedAt.Time.Format(config.DisplayDateTimeFormat)
		}
		receipt := grid.Text("")
		if c.ReceiptURL.Valid && c.ReceiptURL.String != "" {
			receipt = grid.Link("View", c.ReceiptURL.String)
		}
		g.Rows = append(g.Rows, []grid.Cell{
			grid.Text(date),
			grid.Text(c.CustomerName),
			grid.Text(c.CustomerEmail.String),
			{Text: dollars(c.AmountPaid.Int64), SortVal: fmt.Sprintf("%d", c.AmountPaid.Int64)},
			grid.Text(chargeStatus(c)),
			grid.Text(c.Description.String),
			grid.Text(c.Comment.String),
			receipt,
		})
	}

	b := element.NewBuilder()
	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading").T(m.Opts.Title),
		b.PClass("af-help").T("Read-only. Giving records come from Stripe; refunds and corrections are made there."),
		b.DivClass("list-wrapper").R(
			g.Render(b),
		),
	)
	return b.String()
}

// dollars formats cents ("12345" → "$123.45"). Amounts are stored in cents
// (see ChargePresenter).
func dollars(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
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
