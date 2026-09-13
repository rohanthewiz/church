// Package inputerr separates "the admin typed something we refuse" from "the
// server failed", so admin controllers can put the first on screen and log the
// second.
//
// It started life in resource/event; it lives here so article, sermon and user
// saves classify their failures the same way without importing each other.
package inputerr

import "errors"

// InputError is a refusal of what the admin typed, as opposed to a failure of
// the server. A controller distinguishes the two with UserMessage: an
// InputError's message is written for the admin and goes straight into the
// flash on the form, while any other error is logged and replaced with a
// generic message (a pq error has no business on screen).
//
// A type rather than serr's SetUserMsg because serr.UserMsgFromErr only looks
// at the outermost error, and these travel up through serr.Wrap. errors.As
// walks the Unwrap chain (SErr implements Unwrap), so the classification
// survives wrapping.
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

// New builds an InputError. err may be nil when there is no underlying cause
// (a blank required field, say).
func New(msg string, err error) error { return &InputError{Msg: msg, Err: err} }

// UserMessage returns the admin-facing text when err is (or wraps) an
// InputError; ok=false means the error is a server fault.
func UserMessage(err error) (msg string, ok bool) {
	var ie *InputError
	if errors.As(err, &ie) {
		return ie.Msg, true
	}
	return "", false
}
