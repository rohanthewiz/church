package payment

import (
	"bytes"
	"encoding/csv"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"gopkg.in/nullbio/null.v6"
)

func charge(id int64, at time.Time, cents int64, paid bool) *models.Charge {
	return &models.Charge{
		ID: id, CreatedAt: null.TimeFrom(at), CustomerName: "Giver",
		AmountPaid: null.Int64From(cents), Paid: null.BoolFrom(paid),
	}
}

func TestResolveGivingYear(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"", 2026}, {"abc", 2026}, {"2030", 2026}, // default / invalid / future → current
		{"2024", 2024}, {" 2025 ", 2025},
		{"2019", 2022}, // before the first data → earliest
	}
	for _, c := range cases {
		if got := ResolveGivingYear(c.raw, 2022, 2026); got != c.want {
			t.Errorf("ResolveGivingYear(%q) = %d, want %d", c.raw, got, c.want)
		}
	}
}

func TestGroupGivingYearYTD(t *testing.T) {
	// A zone west of UTC makes the month cut observable: 20:00 on Jan 31
	// local is Feb 1 in UTC and must stay in January.
	loc := time.FixedZone("CST", -6*3600)
	now := time.Date(2026, time.March, 15, 12, 0, 0, 0, loc)

	jan31Evening := time.Date(2026, time.January, 31, 20, 0, 0, 0, loc).UTC()
	refunded := charge(3, time.Date(2026, time.March, 2, 9, 0, 0, 0, loc), 5000, true)
	refunded.AmountRefunded = null.Int64From(2000)
	charges := models.ChargeSlice{
		refunded,
		charge(1, jan31Evening, 10000, true),
		charge(2, time.Date(2026, time.January, 5, 9, 0, 0, 0, loc), 2500, true),
		charge(4, time.Date(2026, time.March, 3, 9, 0, 0, 0, loc), 9999, false), // pending
	}

	g := GroupGivingYear(2026, 2024, charges, now)

	if !g.YTD() || !g.HasPrev() || g.HasNext() {
		t.Errorf("YTD=%v HasPrev=%v HasNext=%v; want true true false", g.YTD(), g.HasPrev(), g.HasNext())
	}
	if len(g.Months) != 3 {
		t.Fatalf("YTD through March has %d months, want 3", len(g.Months))
	}
	jan := g.Months[0]
	if len(jan.Charges) != 2 || jan.Charges[0].ID != 2 || jan.Charges[1].ID != 1 {
		t.Errorf("January should hold charges 2 then 1 (oldest first), got %d charges", len(jan.Charges))
	}
	if jan.Totals.Gross != 12500 || jan.Totals.Gifts != 2 {
		t.Errorf("January totals = %+v", jan.Totals)
	}
	if feb := g.Months[1]; len(feb.Charges) != 0 || feb.Key() != "2026-02" {
		t.Errorf("February should be empty with key 2026-02, got %d charges key %s", len(feb.Charges), feb.Key())
	}
	mar := g.Months[2].Totals
	if mar.Gifts != 1 || mar.Pending != 1 || mar.Gross != 5000 || mar.Net() != 3000 {
		t.Errorf("March totals = %+v net %d; the pending charge must not count toward money", mar, mar.Net())
	}
	if g.Totals.Gross != 17500 || g.Totals.Refunded != 2000 || g.Totals.Net() != 15500 || g.Totals.Pending != 1 {
		t.Errorf("year totals = %+v", g.Totals)
	}
}

func TestGroupGivingYearPastYearHasTwelveMonths(t *testing.T) {
	now := time.Date(2026, time.March, 15, 12, 0, 0, 0, time.UTC)
	g := GroupGivingYear(2025, 2025, nil, now)
	if len(g.Months) != 12 || g.YTD() || g.HasPrev() || !g.HasNext() {
		t.Errorf("past year: months=%d YTD=%v HasPrev=%v HasNext=%v", len(g.Months), g.YTD(), g.HasPrev(), g.HasNext())
	}
	if g.CSVFilename() != "giving-2025.csv" {
		t.Errorf("filename = %s", g.CSVFilename())
	}
}

func TestWriteGivingCSV(t *testing.T) {
	now := time.Date(2026, time.February, 10, 12, 0, 0, 0, time.UTC)
	evil := charge(7, time.Date(2026, time.February, 1, 9, 30, 0, 0, time.UTC), 123456, true)
	evil.CustomerName = `=HYPERLINK("http://x","y")`
	evil.Comment = null.StringFrom("thanks, \"pastor\"")
	pending := charge(8, time.Date(2026, time.February, 2, 9, 30, 0, 0, time.UTC), 500, false)
	g := GroupGivingYear(2026, 2026, models.ChargeSlice{evil, pending}, now)

	var buf bytes.Buffer
	if err := WriteGivingCSV(&buf, g); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "\uFEFF") {
		t.Error("CSV must start with a UTF-8 BOM for Excel")
	}
	if !strings.Contains(out, "\r\n") {
		t.Error("CSV should use CRLF line endings")
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(out, "\uFEFF"))).ReadAll()
	if err != nil {
		t.Fatalf("CSV does not parse: %v", err)
	}
	if len(rows) != 3 || strings.Join(rows[0], ",") != strings.Join(GivingCSVHeadings, ",") {
		t.Fatalf("want headings + 2 rows, got %d rows; first %v", len(rows), rows[0])
	}
	r := rows[1]
	if r[0] != "2026-02-01 09:30" || r[1] != "2026-02" || r[4] != "1234.56" || r[6] != "1234.56" || r[7] != "paid" {
		t.Errorf("paid row = %v", r)
	}
	if !strings.HasPrefix(r[2], "'=") {
		t.Errorf("formula in name must be defused, got %q", r[2])
	}
	if r[9] != `thanks, "pastor"` {
		t.Errorf("comment should round-trip through quoting, got %q", r[9])
	}
	if rows[2][6] != "" || rows[2][7] != "pending" {
		t.Errorf("pending row should have blank Net and status pending, got %v", rows[2])
	}
	if g.CSVFilename() != "giving-2026-ytd.csv" {
		t.Errorf("filename = %s", g.CSVFilename())
	}
}

func TestDollarsGroupsThousands(t *testing.T) {
	for cents, want := range map[int64]string{
		0: "$0.00", 5: "$0.05", 99999: "$999.99", 100000: "$1,000.00",
		123456789: "$1,234,567.89", -250000: "-$2,500.00",
	} {
		if got := dollars(cents); got != want {
			t.Errorf("dollars(%d) = %s, want %s", cents, got, want)
		}
	}
}

// TestLoadGivingYearAgainstBytDB runs the report's SQL (range filter,
// IS NOT NULL, ORDER BY ... LIMIT 1) on an embedded bytdb over the wire, the
// backend with the narrower SQL subset.
func TestLoadGivingYearAgainstBytDB(t *testing.T) {
	if err := db.InitDB(db.DBOpts{DBType: db.DBTypes.BytDB, File: filepath.Join(t.TempDir(), "giving.db")}); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(db.CloseDB)
	exec, err := db.Db()
	if err != nil {
		t.Fatalf("Db: %v", err)
	}

	loc := time.Local
	now := time.Now()
	seed := []struct {
		at    time.Time
		cents int64
	}{
		{time.Date(now.Year()-2, time.June, 10, 10, 0, 0, 0, loc), 1000},
		{time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, loc), 2000},        // first instant of the year
		{time.Date(now.Year()-1, time.December, 31, 23, 59, 59, 0, loc), 4000}, // last second of last year
		{time.Date(now.Year(), now.Month(), 1, 12, 0, 0, 0, loc), 8000},
	}
	for i, s := range seed {
		_, err := exec.Exec(`INSERT INTO charges (created_at, customer_name, payment_token, paid, amount_paid)
			VALUES ($1, $2, $3, $4, $5)`, s.at, "Giver", "tok_"+string(rune('a'+i)), true, s.cents)
		if err != nil {
			t.Fatalf("seed charge %d: %v", i, err)
		}
	}
	// A row without created_at must not break the earliest-year lookup.
	if _, err := exec.Exec(`INSERT INTO charges (customer_name, payment_token, paid, amount_paid) VALUES ($1, $2, $3, $4)`,
		"Nodate", "tok_z", true, 1); err != nil {
		t.Fatalf("seed undated charge: %v", err)
	}

	g, err := LoadGivingYear(exec, "", now)
	if err != nil {
		t.Fatalf("LoadGivingYear: %v", err)
	}
	if g.Year != now.Year() || g.EarliestYear != now.Year()-2 {
		t.Errorf("year %d earliest %d; want %d and %d", g.Year, g.EarliestYear, now.Year(), now.Year()-2)
	}
	const wantGross = int64(2000 + 8000)
	if g.Totals.Gross != wantGross || g.Totals.Gifts != 2 {
		t.Errorf("current year totals = %+v; want gross %d from 2 gifts (boundary rows must not leak across years)", g.Totals, wantGross)
	}

	prev, err := LoadGivingYear(exec, "1990", now) // clamps to the earliest year
	if err != nil {
		t.Fatalf("LoadGivingYear 1990: %v", err)
	}
	if prev.Year != now.Year()-2 || prev.HasPrev() || prev.Totals.Gross != 1000 {
		t.Errorf("earliest year: year %d HasPrev %v totals %+v", prev.Year, prev.HasPrev(), prev.Totals)
	}
	lastYear, err := LoadGivingYear(exec, strconv.Itoa(now.Year()-1), now)
	if err != nil {
		t.Fatalf("LoadGivingYear last year: %v", err)
	}
	if lastYear.Totals.Gross != 4000 || len(lastYear.Months) != 12 || !lastYear.HasPrev() || !lastYear.HasNext() {
		t.Errorf("last year totals %+v months %d prev %v next %v; want gross 4000, 12 months, both links",
			lastYear.Totals, len(lastYear.Months), lastYear.HasPrev(), lastYear.HasNext())
	}
}
