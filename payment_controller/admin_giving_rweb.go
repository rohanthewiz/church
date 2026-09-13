package payment_controller

import (
	"bytes"
	"io"
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
	return writeGivingCSV(ctx, payment.WriteGivingCSV, payment.GivingYear.CSVFilename)
}

// AdminGivingSummaryCSVRWeb downloads the year's month totals only
// (GET /admin/giving/csv/summary[?year=2025], charges.read). It is the page's
// "By month" table as a file, for a treasurer's report, beside the per-gift
// export.
func AdminGivingSummaryCSVRWeb(ctx rweb.Context) error {
	return writeGivingCSV(ctx, payment.WriteGivingSummaryCSV, payment.GivingYear.SummaryCSVFilename)
}

// writeGivingCSV is shared by both exports: they differ only in the writer
// and the filename, and must agree on everything else (year resolution,
// headers, caching).
func writeGivingCSV(ctx rweb.Context, write func(io.Writer, payment.GivingYear) error,
	filename func(payment.GivingYear) string) error {
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
	if err = write(&buf, gy); err != nil {
		logger.LogErr(err, "Error writing giving CSV")
		return err
	}

	ctx.Response().SetHeader("Content-Type", "text/csv; charset=utf-8")
	ctx.Response().SetHeader("Content-Disposition", `attachment; filename="`+filename(gy)+`"`)
	// Donor names, emails and amounts: keep them out of browser and proxy caches.
	ctx.Response().SetHeader("Cache-Control", "no-store")
	return ctx.Bytes(buf.Bytes())
}
