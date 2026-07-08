package event

import (
	"fmt"
	"time"

	"github.com/rohanthewiz/serr"
)

// Recurrence is an event's repeat rule (see the event_recurrences migration).
// Two frequencies cover the church scheduling patterns in practice:
//
//	weekly  — every <Weekday>                        ("every Sunday")
//	monthly — the <Week>th <Weekday> of each month   ("second Saturday",
//	          Week -1 = last                          "last Sunday")
//
// The base event's event_date anchors the series: occurrences are only
// generated strictly after it, because the base row itself already appears in
// event listings for its own date. Until (optional) is the last date an
// occurrence may fall on.
type Recurrence struct {
	EventID int64
	Freq    string
	Weekday time.Weekday
	Week    int
	Until   time.Time // zero value = open-ended; date precision only
}

const (
	RecurNone    = "" // no rule / one-time event
	RecurWeekly  = "weekly"
	RecurMonthly = "monthly"

	RecurWeekLast = -1 // Week value meaning "last <weekday> of the month"
)

// Expansion windows are demand-bounded (callers pass from/to), but guard
// against a pathological window generating unbounded output.
const maxOccurrences = 1000

func (r Recurrence) Validate() error {
	if r.Weekday < time.Sunday || r.Weekday > time.Saturday {
		return serr.New("recurrence weekday must be 0 (Sunday) through 6 (Saturday)")
	}
	switch r.Freq {
	case RecurWeekly:
		if r.Week != 0 {
			return serr.New("weekly recurrence does not use the week field")
		}
	case RecurMonthly:
		if r.Week != RecurWeekLast && (r.Week < 1 || r.Week > 4) {
			return serr.New("monthly recurrence week must be 1-4 (first-fourth) or -1 (last)")
		}
	default:
		return serr.New("recurrence freq must be 'weekly' or 'monthly'", "freq", r.Freq)
	}
	return nil
}

// Describe renders the rule for humans: "Every Sunday",
// "Second Saturday of each month", "Last Sunday of each month".
func (r Recurrence) Describe() string {
	switch r.Freq {
	case RecurWeekly:
		return "Every " + r.Weekday.String()
	case RecurMonthly:
		ordinals := map[int]string{1: "First", 2: "Second", 3: "Third", 4: "Fourth", RecurWeekLast: "Last"}
		ord, ok := ordinals[r.Week]
		if !ok {
			return ""
		}
		return fmt.Sprintf("%s %s of each month", ord, r.Weekday.String())
	}
	return ""
}

// Occurrences returns the rule's dates within [from, to], excluding the
// anchor's own date (the base event row represents that one). Each returned
// time carries the anchor's clock time and location so occurrence rows are
// interchangeable with real event_date values downstream.
//
// All range logic runs at date granularity via yyyymmdd ints — this sidesteps
// timezone drift between the anchor (server-local timestamptz) and from/to
// (often parsed from YYYY-MM-DD as UTC), where instant-based comparison could
// shift a date across midnight.
func (r Recurrence) Occurrences(anchor, from, to time.Time) (occurrences []time.Time) {
	if r.Validate() != nil {
		return nil
	}

	lo := maxDateInt(dateInt(from), dateInt(anchor))
	hi := dateInt(to)
	if until := dateInt(r.Until); !r.Until.IsZero() && until < hi {
		hi = until
	}
	if lo > hi {
		return nil
	}
	anchorDate := dateInt(anchor)

	appendIfInRange := func(t time.Time) {
		d := dateInt(t)
		if d >= lo && d <= hi && d != anchorDate && len(occurrences) < maxOccurrences {
			occurrences = append(occurrences, t)
		}
	}

	switch r.Freq {
	case RecurWeekly:
		// Advance from the window start to the first matching weekday, then step by 7s
		t := fromDateInt(lo, anchor)
		t = t.AddDate(0, 0, int((r.Weekday-t.Weekday()+7)%7))
		for ; dateInt(t) <= hi && len(occurrences) < maxOccurrences; t = t.AddDate(0, 0, 7) {
			appendIfInRange(t)
		}

	case RecurMonthly:
		// Walk month by month; each month contributes at most one date
		y, m, _ := fromDateInt(lo, anchor).Date()
		endY, endM, _ := fromDateInt(hi, anchor).Date()
		for !(y > endY || (y == endY && m > endM)) {
			if t, ok := r.dateInMonth(y, m, anchor); ok {
				appendIfInRange(t)
			}
			m++
			if m > time.December {
				m = time.January
				y++
			}
		}
	}
	return occurrences
}

// dateInMonth resolves the rule to its single date within year/month.
// ok is false only for rules that can't land (never happens for week 1-4:
// the fourth weekday is at latest day 28, which every month has).
func (r Recurrence) dateInMonth(year int, month time.Month, anchor time.Time) (time.Time, bool) {
	first := time.Date(year, month, 1, anchor.Hour(), anchor.Minute(), 0, 0, anchor.Location())
	if r.Week == RecurWeekLast {
		// Last day of month, stepped back to the wanted weekday
		last := first.AddDate(0, 1, -1)
		return last.AddDate(0, 0, -int((last.Weekday()-r.Weekday+7)%7)), true
	}
	day := 1 + int((r.Weekday-first.Weekday()+7)%7) + (r.Week-1)*7
	t := time.Date(year, month, day, anchor.Hour(), anchor.Minute(), 0, 0, anchor.Location())
	return t, t.Month() == month
}

// dateInt encodes a time as yyyymmdd for timezone-proof date comparison
func dateInt(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

func maxDateInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// fromDateInt reconstructs a time on the encoded date, carrying the anchor's
// clock time and location (see Occurrences doc comment)
func fromDateInt(d int, anchor time.Time) time.Time {
	return time.Date(d/10000, time.Month(d/100%100), d%100,
		anchor.Hour(), anchor.Minute(), 0, 0, anchor.Location())
}
