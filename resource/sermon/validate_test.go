package sermon

import (
	"strings"
	"testing"

	"github.com/rohanthewiz/church/util/inputerr"
)

func TestPresenterValidate(t *testing.T) {
	ok := Presenter{DateTaught: "2026-09-13"}
	ok.Title = "Grace"
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid sermon refused: %v", err)
	}

	cases := []struct {
		name, title, date, wantMsg string
	}{
		{"blank title", "  ", "2026-09-13", "title"},
		{"blank date", "Grace", "", "date"},
		{"bad date", "Grace", "09/13/2026", "YYYY-MM-DD"},
	}
	for _, c := range cases {
		p := Presenter{DateTaught: c.date}
		p.Title = c.title
		err := p.Validate()
		msg, isInput := inputerr.UserMessage(err)
		if !isInput {
			t.Errorf("%s: expected an InputError, got %v", c.name, err)
			continue
		}
		if !strings.Contains(msg, c.wantMsg) {
			t.Errorf("%s: message %q does not mention %q", c.name, msg, c.wantMsg)
		}
	}
}

// modelFromPresenter keeps its own checks for callers that skip Validate; they
// must classify the same way. Id "" never touches the executor.
func TestModelFromPresenterInputErrors(t *testing.T) {
	p := Presenter{DateTaught: "2026-09-13"}
	if _, _, err := modelFromPresenter(nil, p); err == nil {
		t.Fatal("blank title accepted")
	} else if _, isInput := inputerr.UserMessage(err); !isInput {
		t.Errorf("blank title not an InputError: %v", err)
	}
	p.Title = "Grace"
	p.DateTaught = "nope"
	if _, _, err := modelFromPresenter(nil, p); err == nil {
		t.Fatal("bad date accepted")
	} else if _, isInput := inputerr.UserMessage(err); !isInput {
		t.Errorf("bad date not an InputError: %v", err)
	}
}
