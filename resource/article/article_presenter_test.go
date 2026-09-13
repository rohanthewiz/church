package article

import (
	"testing"

	"github.com/rohanthewiz/church/util/inputerr"
)

// A blank title is the admin's mistake and must reach the form as such.
// Id "" never touches the executor.
func TestModelFromPresenterBlankTitle(t *testing.T) {
	p := Presenter{}
	p.Title = "   "
	_, _, err := modelFromPresenter(nil, p)
	if err == nil {
		t.Fatal("blank title accepted")
	}
	if _, isInput := inputerr.UserMessage(err); !isInput {
		t.Errorf("blank title not an InputError: %v", err)
	}
}
