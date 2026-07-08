package event

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func assertDates(t *testing.T, got []time.Time, want ...time.Time) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d occurrences, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Errorf("occurrence %d: expected %v, got %v", i, want[i], got[i])
		}
	}
}

func TestWeeklyOccurrences(t *testing.T) {
	// Every Sunday, anchored Tue Jul 7 2026. Sundays in window: Jul 12, 19, 26
	r := Recurrence{Freq: RecurWeekly, Weekday: time.Sunday}
	got := r.Occurrences(date(2026, 7, 7), date(2026, 7, 1), date(2026, 7, 31))
	assertDates(t, got,
		date(2026, 7, 12), date(2026, 7, 19), date(2026, 7, 26))
}

func TestWeeklyAnchorDateExcluded(t *testing.T) {
	// Anchor Sun Jul 12 matches the rule but is the base event's own date —
	// the base row already represents it, so expansion must skip it
	r := Recurrence{Freq: RecurWeekly, Weekday: time.Sunday}
	got := r.Occurrences(date(2026, 7, 12), date(2026, 7, 1), date(2026, 7, 31))
	assertDates(t, got, date(2026, 7, 19), date(2026, 7, 26))
}

func TestWeeklyUntilBoundsSeries(t *testing.T) {
	r := Recurrence{Freq: RecurWeekly, Weekday: time.Sunday, Until: date(2026, 7, 19)}
	got := r.Occurrences(date(2026, 7, 7), date(2026, 7, 1), date(2026, 8, 31))
	assertDates(t, got, date(2026, 7, 12), date(2026, 7, 19))
}

func TestMonthlySecondSaturday(t *testing.T) {
	// "Every second Saturday": Aug 8, Sep 12, Oct 10 (2026)
	r := Recurrence{Freq: RecurMonthly, Weekday: time.Saturday, Week: 2}
	got := r.Occurrences(date(2026, 7, 20), date(2026, 8, 1), date(2026, 10, 31))
	assertDates(t, got,
		date(2026, 8, 8), date(2026, 9, 12), date(2026, 10, 10))
}

func TestMonthlyLastSunday(t *testing.T) {
	// "The last Sunday of the month": Aug 30, Sep 27, Oct 25 (2026)
	r := Recurrence{Freq: RecurMonthly, Weekday: time.Sunday, Week: RecurWeekLast}
	got := r.Occurrences(date(2026, 7, 20), date(2026, 8, 1), date(2026, 10, 31))
	assertDates(t, got,
		date(2026, 8, 30), date(2026, 9, 27), date(2026, 10, 25))
}

func TestMonthlyAnchorBoundsStart(t *testing.T) {
	// Window opens Aug 1 but the series doesn't start until the anchor Sep 1
	r := Recurrence{Freq: RecurMonthly, Weekday: time.Saturday, Week: 2}
	got := r.Occurrences(date(2026, 9, 1), date(2026, 8, 1), date(2026, 10, 31))
	assertDates(t, got, date(2026, 9, 12), date(2026, 10, 10))
}

func TestMonthlyYearWrap(t *testing.T) {
	// Dec 2026 -> Jan 2027: last Sundays are Dec 27 and Jan 31
	r := Recurrence{Freq: RecurMonthly, Weekday: time.Sunday, Week: RecurWeekLast}
	got := r.Occurrences(date(2026, 12, 1), date(2026, 12, 5), date(2027, 1, 31))
	assertDates(t, got, date(2026, 12, 27), date(2027, 1, 31))
}

func TestOccurrencesCarryAnchorClockTime(t *testing.T) {
	anchor := time.Date(2026, 7, 7, 10, 30, 0, 0, time.UTC)
	r := Recurrence{Freq: RecurWeekly, Weekday: time.Sunday}
	got := r.Occurrences(anchor, date(2026, 7, 1), date(2026, 7, 15))
	assertDates(t, got, time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC))
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		rec     Recurrence
		wantErr bool
	}{
		{"weekly ok", Recurrence{Freq: RecurWeekly, Weekday: time.Sunday}, false},
		{"monthly second saturday", Recurrence{Freq: RecurMonthly, Weekday: time.Saturday, Week: 2}, false},
		{"monthly last sunday", Recurrence{Freq: RecurMonthly, Weekday: time.Sunday, Week: RecurWeekLast}, false},
		{"bad freq", Recurrence{Freq: "daily", Weekday: time.Monday}, true},
		{"weekly with week set", Recurrence{Freq: RecurWeekly, Weekday: time.Sunday, Week: 2}, true},
		{"monthly week zero", Recurrence{Freq: RecurMonthly, Weekday: time.Sunday}, true},
		{"monthly week five", Recurrence{Freq: RecurMonthly, Weekday: time.Sunday, Week: 5}, true},
		{"weekday out of range", Recurrence{Freq: RecurWeekly, Weekday: 7}, true},
	}
	for _, c := range cases {
		if err := c.rec.Validate(); (err != nil) != c.wantErr {
			t.Errorf("%s: wantErr=%v, got %v", c.name, c.wantErr, err)
		}
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		rec  Recurrence
		want string
	}{
		{Recurrence{Freq: RecurWeekly, Weekday: time.Sunday}, "Every Sunday"},
		{Recurrence{Freq: RecurMonthly, Weekday: time.Saturday, Week: 2}, "Second Saturday of each month"},
		{Recurrence{Freq: RecurMonthly, Weekday: time.Sunday, Week: RecurWeekLast}, "Last Sunday of each month"},
		{Recurrence{Freq: RecurNone}, ""},
	}
	for _, c := range cases {
		if got := c.rec.Describe(); got != c.want {
			t.Errorf("expected %q, got %q", c.want, got)
		}
	}
}
