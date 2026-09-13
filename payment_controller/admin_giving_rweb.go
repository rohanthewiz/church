package payment_controller

import (
	"bytes"
	"time"

	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/payment"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

// AdminListGivingRWeb renders the read-only giving records page
// (GET /admin/giving[?year=2025], charges.read — see router_rweb.go).
// It shows one calendar year grouped by month; no year means year to date.
func AdminListGivingRWeb(ctx rweb.Context) error {
	pg, err := page.GivingList()
	if err != nil {
		return err
	}
	// The module reads the year from its own params (modules have no request
	// context). The value is validated and clamped by payment.ResolveGivingYear.
	return ctx.WriteHTML(string(base.RenderPageListWithOptsRWeb(pg, ctx,
		map[string]string{"year": ctx.Request().QueryParam("year")})))
}

// AdminGivingCSVRWeb downloads the same year the page shows as CSV
// (GET /admin/giving/csv[?year=2025], charges.read).
//
// It builds the report through payment.LoadGivingYear, the same call the page
// makes, so the page and the export can't disagree about a year's gifts.
func AdminGivingCSVRWeb(ctx rweb.Context) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err, "Could not obtain DB handle")
	}
	gy, err := payment.LoadGivingYear(dbH, ctx.Request().QueryParam("year"), time.Now())
	if err != nil {
		logger.LogErr(err, "Error loading giving year for CSV")
		return err
	}

	// Buffer the whole file: the rweb response is buffered anyway, and a
	// mid-write error then can't leave a truncated download that looks complete.
	var buf bytes.Buffer
	if err = payment.WriteGivingCSV(&buf, gy); err != nil {
		logger.LogErr(err, "Error writing giving CSV")
		return err
	}

	ctx.Response().SetHeader("Content-Type", "text/csv; charset=utf-8")
	ctx.Response().SetHeader("Content-Disposition", `attachment; filename="`+gy.CSVFilename()+`"`)
	// Donor names, emails and amounts: keep them out of browser and proxy caches.
	ctx.Response().SetHeader("Cache-Control", "no-store")
	return ctx.Bytes(buf.Bytes())
}
