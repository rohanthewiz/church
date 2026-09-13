package sermon

import (
	"strings"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/util/inputerr"
)

const sermonTitleRequired = "A sermon needs a title"

// Validate checks everything in the presenter that can be refused without the
// database or the filesystem.
//
// Ordering is the point. The sermon controller copies the uploaded audio file
// to local disk (and later schedules its IDrive transfer) before it saves the
// row. If the title or date were refused only inside Upsert, a failed save
// would already have written the file, and for an existing sermon os.Create
// would already have truncated the old one. Calling Validate first makes a
// refused save touch nothing, which is what makes sending the admin back to
// the form safe.
//
// modelFromPresenter keeps its own title and date checks: Upsert has other
// callers (the legacy import) that do not go through Validate.
func (p Presenter) Validate() error {
	if strings.TrimSpace(p.Title) == "" {
		return inputerr.New(sermonTitleRequired, nil)
	}
	if _, err := p.dateTaught(); err != nil {
		return err
	}
	return nil
}

// dateTaught parses the form's date as 11:00 in the server's zone, the
// interpretation modelFromPresenter has always used (the form has no time
// field; 11:00 keeps the date from shifting a day when rendered in a nearby
// zone).
func (p Presenter) dateTaught() (time.Time, error) {
	date := strings.TrimSpace(p.DateTaught)
	if date == "" {
		return time.Time{}, inputerr.New("A sermon needs the date it was taught", nil)
	}
	zone, _ := time.Now().Zone() // server timezone should be good enough
	dte, err := time.Parse(config.IncomingDateTimeFormat, date+" 11:00 "+zone)
	if err != nil {
		return time.Time{}, inputerr.New("Sermon date must be YYYY-MM-DD", err)
	}
	return dte, nil
}
