package event

import (
	"errors"
	"strings"
	"testing"

	"github.com/rohanthewiz/serr"
)

// validPresenter is the smallest form submission that saves: a titled
// one-time event with no location of its own.
func validPresenter() Presenter {
	p := Presenter{EventDate: "2026-10-04", EventTime: "10:00"}
	p.Title = "Harvest Sunday"
	return p
}

func TestPresenterValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(p *Presenter)
		wantMsg string // substring of the admin-facing message; "" means valid
	}{
		{"minimal ok", func(p *Presenter) {}, ""},
		{"point and monthly rule ok", func(p *Presenter) {
			p.Latitude, p.Longitude = "38.7223", "-9.1393"
			p.RecurFreq, p.RecurWeekday, p.RecurWeek, p.RecurUntil = RecurMonthly, "0", "-1", "2026-12-31"
		}, ""},
		{"blank title", func(p *Presenter) { p.Title = "  " }, "needs a title"},
		{"blank time", func(p *Presenter) { p.EventTime = "" }, "both a date and a time"},
		{"bad date", func(p *Presenter) { p.EventDate = "10/04/2026" }, "YYYY-MM-DD"},
		{"latitude only", func(p *Presenter) { p.Latitude = "38.7" }, "both a latitude and a longitude"},
		{"latitude out of range", func(p *Presenter) { p.Latitude, p.Longitude = "300", "0" }, "between -90 and 90"},
		{"longitude not a number", func(p *Presenter) { p.Latitude, p.Longitude = "1", "west" }, "Longitude must be a number"},
		{"monthly week five", func(p *Presenter) {
			p.RecurFreq, p.RecurWeekday, p.RecurWeek = RecurMonthly, "0", "5"
		}, "Invalid recurrence"},
		{"bad until", func(p *Presenter) {
			p.RecurFreq, p.RecurWeekday, p.RecurUntil = RecurWeekly, "0", "soon"
		}, "end date"},
	}
	for _, c := range cases {
		p := validPresenter()
		c.mutate(&p)
		err := p.Validate()
		if c.wantMsg == "" {
			if err != nil {
				t.Errorf("%s: expected valid, got %v", c.name, err)
			}
			continue
		}
		msg, isInput := UserMessage(err)
		if !isInput {
			t.Errorf("%s: expected an InputError, got %v", c.name, err)
			continue
		}
		if !strings.Contains(msg, c.wantMsg) {
			t.Errorf("%s: message %q does not contain %q", c.name, msg, c.wantMsg)
		}
	}
}

// The controller classifies errors that UpsertEvent has passed through
// serr.Wrap, so the classification has to survive wrapping, and a plain server
// error must not be mistaken for the admin's fault.
func TestUserMessageThroughWrap(t *testing.T) {
	p := validPresenter()
	p.Latitude = "1"
	wrapped := serr.Wrap(p.Validate(), "when", "saving")
	if msg, ok := UserMessage(wrapped); !ok || !strings.Contains(msg, "latitude") {
		t.Errorf("wrapped InputError not recognised: ok=%v msg=%q", ok, msg)
	}
	if _, ok := UserMessage(serr.Wrap(errors.New("pq: connection refused"))); ok {
		t.Error("server error misclassified as input error")
	}
}
