package event

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/rohanthewiz/church/config"
)

// InputError is a refusal of what the admin typed, as opposed to a failure of
// the server. The controller distinguishes the two with errors.As: an
// InputError's message is written for the admin and goes straight into the
// flash on the form, while any other error is logged and replaced with a
// generic message (a pq error has no business on screen).
//
// A type rather than serr's SetUserMsg because serr.UserMsgFromErr only looks
// at the outermost error, and these travel up through serr.Wrap. errors.As
// walks the Unwrap chain, so the classification survives wrapping.
type InputError struct {
	Msg string // shown to the admin verbatim
	Err error  // underlying parse/validation error, kept for the log
}

func (e *InputError) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

func (e *InputError) Unwrap() error { return e.Err }

func inputErr(msg string, err error) error { return &InputError{Msg: msg, Err: err} }

// UserMessage returns the admin-facing text when err is (or wraps) an
// InputError; ok=false means the error is a server fault.
func UserMessage(err error) (msg string, ok bool) {
	var ie *InputError
	if errors.As(err, &ie) {
		return ie.Msg, true
	}
	return "", false
}

// Validate checks everything in the presenter that can be refused without the
// database, and must run before any write.
//
// Ordering is the point. UpsertEvent writes the events row first and the
// recurrence/location rows after (they need the event ID), with no
// transaction. Validation used to live in those later steps, so a bad latitude
// was reported only after the event row had been inserted, leaving a
// "failed" create that had in fact created an event, minus its location, and
// a retry that made a second one. Refusing up front makes a failed save a
// no-op, which is also what makes sending the admin back to the form safe.
func (p Presenter) Validate() error {
	if strings.TrimSpace(p.Title) == "" {
		return inputErr("An event needs a title", nil)
	}
	if _, err := p.eventDateTime(); err != nil {
		return err
	}
	if _, _, err := p.parseRecurrence(); err != nil {
		return err
	}
	if _, _, err := p.parsePoint(); err != nil {
		return err
	}
	return nil
}

// eventDateTime parses the form's date and time in the server's zone, the same
// interpretation modelFromPresenter has always used.
func (p Presenter) eventDateTime() (time.Time, error) {
	date, tm := strings.TrimSpace(p.EventDate), strings.TrimSpace(p.EventTime)
	if date == "" || tm == "" {
		return time.Time{}, inputErr("An event needs both a date and a time", nil)
	}
	zone, _ := time.Now().Zone()
	dte, err := time.Parse(config.IncomingDateTimeFormat, date+" "+tm+" "+zone)
	if err != nil {
		return time.Time{}, inputErr("Event date must be YYYY-MM-DD and time HH:MM", err)
	}
	return dte, nil
}

// parseRecurrence turns the form strings into a validated rule. set=false
// means "None" was chosen, i.e. a one-time event. EventID is left zero for the
// caller to fill once the event row exists.
func (p Presenter) parseRecurrence() (rec Recurrence, set bool, err error) {
	if p.RecurFreq == RecurNone {
		return rec, false, nil
	}
	weekday, err := strconv.Atoi(p.RecurWeekday)
	if err != nil {
		return rec, true, inputErr("Recurrence weekday must be a number", err)
	}
	rec = Recurrence{Freq: p.RecurFreq, Weekday: time.Weekday(weekday)}
	if p.RecurFreq == RecurMonthly {
		// The form always submits a week value; it is only meaningful for monthly
		if rec.Week, err = strconv.Atoi(p.RecurWeek); err != nil {
			return rec, true, inputErr("Recurrence week must be a number", err)
		}
	}
	if p.RecurUntil != "" {
		if rec.Until, err = time.Parse("2006-01-02", p.RecurUntil); err != nil {
			return rec, true, inputErr("Recurrence end date must be YYYY-MM-DD", err)
		}
	}
	if err = rec.Validate(); err != nil {
		return rec, true, inputErr("Invalid recurrence: "+err.Error(), err)
	}
	return rec, true, nil
}

// parsePoint turns the two coordinate strings into a validated point.
// set=false means both were blank (the event is at the church). Both or
// neither: a half-filled pair is somebody who was interrupted, and saving it
// would put the map at the prime meridian while looking configured.
func (p Presenter) parsePoint() (pt Point, set bool, err error) {
	lat := strings.TrimSpace(p.Latitude)
	lng := strings.TrimSpace(p.Longitude)
	if lat == "" && lng == "" {
		return pt, false, nil
	}
	if lat == "" || lng == "" {
		return pt, true, inputErr("An event location needs both a latitude and a longitude", nil)
	}
	if pt.Latitude, err = strconv.ParseFloat(lat, 64); err != nil {
		return pt, true, inputErr("Latitude must be a number", err)
	}
	if pt.Longitude, err = strconv.ParseFloat(lng, 64); err != nil {
		return pt, true, inputErr("Longitude must be a number", err)
	}
	// SErr.Error() is the bare message (fields and location stay out of it), and
	// the Validate messages are already phrased for people, so they pass through.
	if err = pt.Validate(); err != nil {
		return pt, true, inputErr("Invalid location: "+err.Error(), err)
	}
	return pt, true, nil
}
