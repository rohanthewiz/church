package inputerr

import (
	"errors"
	"testing"

	"github.com/rohanthewiz/serr"
)

func TestUserMessage(t *testing.T) {
	cause := errors.New("parsing time")
	ie := New("Date must be YYYY-MM-DD", cause)

	if msg, ok := UserMessage(ie); !ok || msg != "Date must be YYYY-MM-DD" {
		t.Errorf("bare InputError: ok=%v msg=%q", ok, msg)
	}
	// The error text keeps the cause for the log; the flash text does not.
	if got := ie.Error(); got != "Date must be YYYY-MM-DD: parsing time" {
		t.Errorf("Error() = %q", got)
	}
	if !errors.Is(ie, cause) {
		t.Error("cause not reachable through Unwrap")
	}

	// Controllers receive these through one or more serr.Wrap layers.
	wrapped := serr.Wrap(serr.Wrap(New("A title is required", nil), "in presenter"), "in upsert")
	if msg, ok := UserMessage(wrapped); !ok || msg != "A title is required" {
		t.Errorf("wrapped InputError: ok=%v msg=%q", ok, msg)
	}

	// A server fault must never be shown as if it were the admin's mistake.
	if _, ok := UserMessage(serr.Wrap(errors.New("pq: connection refused"))); ok {
		t.Error("server error misclassified as input")
	}
	if _, ok := UserMessage(nil); ok {
		t.Error("nil misclassified as input")
	}
}
