package user

import (
	"testing"

	"github.com/rohanthewiz/church/util/inputerr"
)

// A password mismatch is the admin's mistake and must reach the form as such.
// Id "" never touches the executor.
func TestModelFromPresenterPasswordMismatch(t *testing.T) {
	_, _, err := modelFromPresenter(nil, Presenter{Username: "u", Password: "a", PasswordConfirmation: "b"})
	if err == nil {
		t.Fatal("mismatched passwords accepted")
	}
	if _, isInput := inputerr.UserMessage(err); !isInput {
		t.Errorf("mismatch not an InputError: %v", err)
	}
}
