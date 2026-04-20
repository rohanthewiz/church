package sermon

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/church/resource/content"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

type Presenter struct {
	content.Content
	AudioLink       string
	DateTaught      string
	DateTaughtShort string
	PlaceTaught     string
	Teacher         string
	ScriptureRefs   []string
}

func PresenterFromSlug(slug string) (pres Presenter, err error) {
	m, err := findSermonBySlug(slug)
	if err != nil {
		return pres, serr.Wrap(err, "Error finding sermon by slug")
	}
	return presenterFromModel(m), nil
}

func presenterFromId(id int64) (pres Presenter, err error) {
	m, err := findSermonById(id)
	if err != nil {
		return pres, serr.Wrap(err, "Unable to obtain sermon", "when", "finding sermon by Id")
	}
	return presenterFromModel(m), nil
}

// presenterFromModel flattens the DB struct (with sql.Null* wrappers and
// StringSlice values) into plain strings and []string for the view layer.
// Empty category / scripture-ref slices become [""] so the editor form
// always shows at least one input row.
func presenterFromModel(ser *model.Sermon) (pres Presenter) {
	if ser.CreatedAt.Valid {
		pres.CreatedAt = ser.CreatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	if ser.UpdatedAt.Valid {
		pres.UpdatedAt = ser.UpdatedAt.Time.Format(config.DisplayDateTimeFormat)
	}

	pres.Id = fmt.Sprintf("%d", ser.ID)
	pres.Title = ser.Title
	pres.Slug = ser.Slug.String
	pres.Summary = ser.Summary.String
	pres.Body = ser.Body.String
	pres.Published = ser.Published
	pres.UpdatedBy = ser.UpdatedBy

	cats := []string(ser.Categories)
	if len(cats) < 1 {
		cats = []string{""}
	}
	pres.Categories = cats

	pres.AudioLink = ser.AudioLink.String

	// Coerce the stored (tz-less) timestamp into the server's local zone so
	// the admin form and public display show a reasonable calendar date.
	if local, err := time.LoadLocation("Local"); err != nil {
		logger.LogErr(err, "Failed to load time location 'Local'")
	} else {
		localEventDate := ser.DateTaught.In(local)
		pres.DateTaught = localEventDate.Format(config.DisplayDateFormat)
		pres.DateTaughtShort = localEventDate.Format(config.DisplayShortDateFormat)
	}

	pres.PlaceTaught = ser.PlaceTaught.String
	pres.Teacher = ser.Teacher

	srefs := []string(ser.ScriptureRefs)
	if len(srefs) < 1 {
		srefs = []string{""}
	}
	pres.ScriptureRefs = srefs
	return
}

// modelFromPresenter builds the DB struct. On create we generate a slug from
// the title; on update slug is left untouched to preserve external links.
// Nullable text columns are wrapped in sql.NullString with Valid=true (we
// always store a concrete string — empty-string is a legitimate value for
// place_taught / summary / body / audio_link).
func modelFromPresenter(ser Presenter) (sermod *model.Sermon, create_op bool, err error) {
	sermod = findByIdOrCreate(ser.Id)
	if sermod.ID < 1 {
		create_op = true
	}

	title := strings.TrimSpace(ser.Title)
	if title == "" {
		return sermod, create_op, serr.Wrap(errors.New("Sermon title is a required field when creating sermons"))
	}
	sermod.Title = title
	if create_op {
		ser.CreateSlug()
		sermod.Slug = sql.NullString{String: ser.Slug, Valid: true}
	}

	// Incoming date is a bare YYYY-MM-DD string from the form; splice in a
	// fixed 11:00 time plus server timezone so the parser has enough context.
	zone, _ := time.Now().Zone()
	datetimez := ser.DateTaught + " 11:00 " + zone
	dte, err := time.Parse(config.IncomingDateTimeFormat, datetimez)
	if err != nil {
		return sermod, create_op, serr.Wrap(err, "Error parsing sermon date")
	}
	sermod.DateTaught = dte

	sermod.PlaceTaught = sql.NullString{String: strings.TrimSpace(ser.PlaceTaught), Valid: true}
	if link := strings.TrimSpace(ser.AudioLink); link != "" {
		sermod.AudioLink = sql.NullString{String: link, Valid: true}
	}
	sermod.Teacher = strings.TrimSpace(ser.Teacher)
	sermod.Published = ser.Published
	sermod.Summary = sql.NullString{String: strings.TrimSpace(ser.Summary), Valid: true}
	sermod.Body = sql.NullString{String: strings.TrimSpace(ser.Body), Valid: true}
	sermod.UpdatedBy = strings.TrimSpace(ser.UpdatedBy)

	// Rebuild the arrays rather than appending — otherwise the DB column would
	// grow on every edit with stale duplicates. Trim and drop empty entries
	// produced by empty form rows.
	if len(ser.Categories) > 0 {
		categories := []string{}
		for _, cat := range ser.Categories {
			if trimmed := strings.TrimSpace(cat); trimmed != "" {
				categories = append(categories, trimmed)
			}
		}
		sermod.Categories = categories
	}
	if len(ser.ScriptureRefs) > 0 {
		srefs := []string{}
		for _, sref := range ser.ScriptureRefs {
			if trimmed := strings.TrimSpace(sref); trimmed != "" {
				srefs = append(srefs, trimmed)
			}
		}
		sermod.ScriptureRefs = srefs
	}
	return
}
