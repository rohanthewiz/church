package payment

// Giving report: one calendar year of charges grouped by month, shared by the
// admin giving page (module_giving_list.go) and its CSV export
// (payment_controller.AdminGivingCSVRWeb), so the screen and the download
// always agree on which gifts a year contains and how they are totalled.
//
//	LoadGivingYear ──► EarliestGivingYear  (bounds "« previous year")
//	               ──► charges in [Jan 1 year, Jan 1 year+1)  site-local time
//	               ──► GroupGivingYear     (pure: months + totals, unit tested)
//	                        │
//	          ┌─────────────┴─────────────┐
//	   module render (HTML)        WriteGivingCSV (download)
//
// Portability: the SQL is single-table with a range filter and ORDER BY, the
// subset both Postgres and bytdb (over the wire) support. Grouping and sums
// happen in Go rather than with GROUP BY/SUM: a church's yearly charge count
// is small (hundreds to low thousands), and the page lists every gift anyway.

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/serr"
	"github.com/vattle/sqlboiler/queries/qm"
)

// GivingTotals sums a set of charges. Money is in cents.
//
// Only settled (paid) charges count toward the money columns; an unpaid
// charge is a started-but-unfinished payment, so it is counted separately as
// Pending rather than inflating what the church actually received.
type GivingTotals struct {
	Gifts    int   // paid charges
	Gross    int64 // sum of amount_paid over paid charges
	Refunded int64 // sum of amount_refunded over paid charges
	Pending  int   // unpaid charges (excluded from the sums)
}

// Net is what the church kept: gross less refunds.
func (t GivingTotals) Net() int64 { return t.Gross - t.Refunded }

func (t *GivingTotals) add(c *models.Charge) {
	if !c.Paid.Bool {
		t.Pending++
		return
	}
	t.Gifts++
	t.Gross += c.AmountPaid.Int64
	t.Refunded += refundedCents(c)
}

// refundedCents is the refunded amount capped at what was paid, so a bad
// webhook value can't turn a month's net negative on its own. A refunded flag
// with no amount stays 0, matching chargeStatus's "partly refunded" reading.
func refundedCents(c *models.Charge) int64 {
	r := c.AmountRefunded.Int64
	if r < 0 {
		return 0
	}
	if r > c.AmountPaid.Int64 {
		return c.AmountPaid.Int64
	}
	return r
}

// GivingMonth is one month of a GivingYear.
type GivingMonth struct {
	Start   time.Time          // first instant of the month, site-local
	Charges models.ChargeSlice // oldest first
	Totals  GivingTotals
}

// Key is a stable, sortable month id ("2026-03"), used for anchors and CSV.
func (m GivingMonth) Key() string { return m.Start.Format("2006-01") }

// Label is the display name ("March 2026").
func (m GivingMonth) Label() string { return m.Start.Format("January 2006") }

// GivingYear is the report for one calendar year.
type GivingYear struct {
	Year         int
	CurrentYear  int // the year "now" falls in; the report can't go past it
	EarliestYear int // first year holding any charge (CurrentYear when none)

	// Months runs January through December for a past year, and January
	// through the current month for the current year (year to date). Months
	// without gifts are included so the summary shows the gaps.
	Months []GivingMonth
	Totals GivingTotals
}

// YTD reports whether this is the current, still-running year.
func (g GivingYear) YTD() bool { return g.Year == g.CurrentYear }

// HasPrev: back navigation stops at the first year that has data.
func (g GivingYear) HasPrev() bool { return g.Year > g.EarliestYear }

// HasNext: forward navigation stops at the current year.
func (g GivingYear) HasNext() bool { return g.Year < g.CurrentYear }

// CSVFilename names the download, marking a partial (YTD) year so a treasurer
// doesn't mistake a mid-year export for the full year.
func (g GivingYear) CSVFilename() string {
	if g.YTD() {
		return fmt.Sprintf("giving-%d-ytd.csv", g.Year)
	}
	return fmt.Sprintf("giving-%d.csv", g.Year)
}

// LoadGivingYear builds the report for rawYear (a query-string value; empty
// or invalid means the current year). now fixes both "the current year" and
// the time zone months are cut in: month and year boundaries follow the
// site's local clock, not UTC, so a gift at 8pm on Dec 31 lands in December.
func LoadGivingYear(exec db.Executor, rawYear string, now time.Time) (GivingYear, error) {
	loc := now.Location()
	earliest, err := EarliestGivingYear(exec, loc, now.Year())
	if err != nil {
		return GivingYear{}, err
	}
	year := ResolveGivingYear(rawYear, earliest, now.Year())

	start := time.Date(year, time.January, 1, 0, 0, 0, 0, loc)
	end := start.AddDate(1, 0, 0)
	// Half-open range [start, end): no double-counting at the midnight boundary.
	charges, err := models.Charges(exec,
		qm.Where("created_at >= ? AND created_at < ?", start, end),
		qm.OrderBy("created_at ASC"),
	).All()
	if err != nil {
		return GivingYear{}, serr.Wrap(err, "Error querying charges for year", "year", strconv.Itoa(year))
	}
	return GroupGivingYear(year, earliest, charges, now), nil
}

// EarliestGivingYear returns the site-local year of the oldest charge, or
// fallback when there are none. A single-row ORDER BY ... LIMIT 1 instead of
// MIN() keeps to SQL that bytdb supports; created_at is indexed.
func EarliestGivingYear(exec db.Executor, loc *time.Location, fallback int) (int, error) {
	var first time.Time
	err := exec.QueryRow(`SELECT created_at FROM charges WHERE created_at IS NOT NULL ORDER BY created_at ASC LIMIT 1`).Scan(&first)
	if err == sql.ErrNoRows {
		return fallback, nil
	}
	if err != nil {
		return 0, serr.Wrap(err, "Error finding the earliest charge")
	}
	if y := first.In(loc).Year(); y < fallback {
		return y, nil
	}
	// A charge dated in the future (clock skew) must not push navigation past
	// the current year.
	return fallback, nil
}

// ResolveGivingYear turns the requested year into one the report can show:
// empty/invalid or future → current year; before the first data → earliest.
// Clamping (rather than erroring) keeps a hand-edited or stale URL useful.
func ResolveGivingYear(raw string, earliest, current int) int {
	year, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || year > current {
		return current
	}
	if year < earliest {
		return earliest
	}
	return year
}

// GroupGivingYear buckets one year's charges into months and totals them.
// Pure (no I/O), so grouping, YTD month range and time-zone handling are
// unit tested without a database.
func GroupGivingYear(year, earliest int, charges models.ChargeSlice, now time.Time) GivingYear {
	loc := now.Location()
	g := GivingYear{Year: year, CurrentYear: now.Year(), EarliestYear: earliest}

	// Order in Go as well as SQL: ties on created_at (same second) and any
	// backend difference in ORDER BY then still produce a stable listing.
	sorted := make(models.ChargeSlice, 0, len(charges))
	for _, c := range charges {
		if c != nil && c.CreatedAt.Valid {
			sorted = append(sorted, c)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		ti, tj := sorted[i].CreatedAt.Time, sorted[j].CreatedAt.Time
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return sorted[i].ID < sorted[j].ID
	})

	lastMonth := 12
	if year == now.Year() {
		lastMonth = int(now.Month())
	}
	// A charge later in the year than "now" (clock skew between the payment
	// recorder and this server) extends the range rather than being dropped.
	for _, c := range sorted {
		if t := c.CreatedAt.Time.In(loc); t.Year() == year && int(t.Month()) > lastMonth {
			lastMonth = int(t.Month())
		}
	}

	g.Months = make([]GivingMonth, lastMonth)
	for i := range g.Months {
		g.Months[i].Start = time.Date(year, time.Month(i+1), 1, 0, 0, 0, 0, loc)
	}
	for _, c := range sorted {
		t := c.CreatedAt.Time.In(loc)
		if t.Year() != year { // outside the requested year in this zone
			continue
		}
		m := &g.Months[t.Month()-1]
		m.Charges = append(m.Charges, c)
		m.Totals.add(c)
		g.Totals.add(c)
	}
	return g
}

// GivingCSVHeadings is the header row of the CSV export.
var GivingCSVHeadings = []string{
	"Date", "Month", "Name", "Email", "Amount", "Refunded", "Net",
	"Status", "Description", "Comment", "Receipt Number", "Receipt URL",
}

// WriteGivingCSV writes one row per charge, oldest first, under
// GivingCSVHeadings. Each row carries its month ("2026-03") so a spreadsheet
// can pivot or subtotal by month. Totals rows are left out on purpose: they
// break sorting and filtering, and the spreadsheet can compute them.
//
// Format choices:
//   - Amounts are plain decimals ("1234.50", no "$" or thousands separator)
//     so spreadsheets read them as numbers.
//   - Net is blank for an unpaid charge, matching the page's totals, which
//     exclude pending charges.
//   - A UTF-8 byte-order mark leads the file. Without it, Excel (what most
//     treasurers open this with) decodes as Windows-1252 and garbles accented
//     donor names; other CSV readers skip the BOM.
//   - CRLF line endings, per RFC 4180 and Excel's expectation.
func WriteGivingCSV(w io.Writer, g GivingYear) error {
	if _, err := io.WriteString(w, "\uFEFF"); err != nil {
		return serr.Wrap(err, "Error writing CSV byte-order mark")
	}
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	if err := cw.Write(GivingCSVHeadings); err != nil {
		return serr.Wrap(err, "Error writing CSV headings")
	}

	for _, m := range g.Months {
		loc := m.Start.Location()
		for _, c := range m.Charges {
			net := ""
			if c.Paid.Bool {
				net = decimalCents(c.AmountPaid.Int64 - refundedCents(c))
			}
			row := []string{
				c.CreatedAt.Time.In(loc).Format("2006-01-02 15:04"),
				m.Key(),
				csvText(c.CustomerName),
				csvText(c.CustomerEmail.String),
				decimalCents(c.AmountPaid.Int64),
				decimalCents(c.AmountRefunded.Int64),
				net,
				chargeStatus(c),
				csvText(c.Description.String),
				csvText(c.Comment.String),
				csvText(c.ReceiptNumber.String),
				csvText(c.ReceiptURL.String),
			}
			if err := cw.Write(row); err != nil {
				return serr.Wrap(err, "Error writing CSV row", "charge_id", strconv.FormatInt(c.ID, 10))
			}
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return serr.Wrap(err, "Error flushing CSV")
	}
	return nil
}

// csvText defuses spreadsheet formula injection. Name, comment and
// description come from the public giving form, so a value such as
// `=HYPERLINK("http://evil","click")` would run as a formula when the export
// is opened. A leading apostrophe makes a spreadsheet treat the cell as text
// (OWASP's recommended CSV-injection mitigation). encoding/csv handles
// quoting; this only handles the formula prefix.
func csvText(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// decimalCents formats cents as a plain decimal ("123456" → "1234.56").
func decimalCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}
