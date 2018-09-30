package sermon

import (
	"github.com/rohanthewiz/church/resource/content"
	"github.com/rohanthewiz/church/models"
	"fmt"
	"time"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/logger"
	"strings"
	"gopkg.in/nullbio/null.v6"
	"github.com/rohanthewiz/serr"
	"errors"
)

type Presenter struct {
	content.Content
	AudioLink string
	DateTaught string
	DateTaughtShort string
	PlaceTaught string
	Teacher string
	ScriptureRefs []string
}

func PresenterFromSlug(slug string) (pres Presenter, err error) {
	model, err := findSermonBySlug(slug)
	if err != nil {
		return pres, serr.Wrap(err, "Error finding sermon by slug")
	}

	fmt.Println("Sermon model (from slug):", model.ID, model.Title, model.AudioLink)

	pres = presenterFromModel(model)
	return
}

func presenterFromId(id int64) (pres Presenter, err error) {
	model, err := findSermonById(id)
	if err != nil {
		return pres, serr.Wrap(err, "Unable to obtain sermon", "when", "finding sermon by Id")
	}
	return presenterFromModel(model), nil
}

func presenterFromModel(ser *models.Sermon) (pres Presenter) {
	if ser.CreatedAt.Valid {
		pres.CreatedAt = ser.CreatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	if ser.UpdatedAt.Valid {
		pres.UpdatedAt = ser.UpdatedAt.Time.Format(config.DisplayDateTimeFormat)
	}

	// Generic Content
	pres.Title = ser.Title
	pres.Slug = ser.Slug.String
	// Categories
	cats := []string{}
	for _, cat := range ser.Categories {
		cats = append(cats, cat)
	}
	if len(cats) < 1 {
		cats = []string{""}
	}
	pres.Categories = cats
	// Scripture references
	pres.Id = fmt.Sprintf("%d", ser.ID)
	pres.Summary = ser.Summary.String
	pres.Body = ser.Body.String
	pres.Published = ser.Published
	pres.UpdatedBy = ser.UpdatedBy

	// Sermon specific content
	pres.AudioLink = ser.AudioLink.String

	local, err := time.LoadLocation("Local")
	if err != nil {
		logger.LogErr(err, "Failed to load time location 'Local'")
	} else {
		localEventDate := ser.DateTaught.In(local)
		pres.DateTaught = localEventDate.Format(config.DisplayDateFormat)
		pres.DateTaughtShort = localEventDate.Format(config.DisplayShortDateFormat)
	}

	pres.PlaceTaught = ser.PlaceTaught.String
	pres.Teacher = ser.Teacher

	srefs := []string{}
	for _, sref := range ser.ScriptureRefs {
		srefs = append(srefs, sref)
	}
	if len(srefs) < 1 {
		srefs = []string{""}
	}
	pres.ScriptureRefs = srefs
	return
}

func modelFromPresenter(ser Presenter) (sermod *models.Sermon, create_op bool, err error) {
	sermod = findByIdOrCreate(ser.Id)
	if sermod.ID < 1 {
		create_op = true
	}

	if title := strings.TrimSpace(ser.Title); title != "" {
		sermod.Title = title
		if create_op {  // Allow slug update only on create to maintain external references
			ser.CreateSlug() // could check for uniqueness
			sermod.Slug = null.NewString(ser.Slug, true)
		}
	} else {
		msg := "Sermon title is a required field when creating sermons"
		return sermod, create_op, serr.Wrap(errors.New(msg))
	}
	zone, _ := time.Now().Zone()  // server timezone should be good enough? I hope!
	datetimez := ser.DateTaught + " 11:00 " + zone
	fmt.Println("[Debug] datetimez:", datetimez)  // debug
	dte, err := time.Parse(config.IncomingDateTimeFormat, datetimez)
	if err != nil {
		return sermod, create_op, serr.Wrap(err, "Error parsing sermon date")
	}
	sermod.DateTaught = dte
	sermod.PlaceTaught = null.NewString(strings.TrimSpace(ser.PlaceTaught), true)
	serAudioLink := strings.TrimSpace(ser.AudioLink)
	if serAudioLink != "" {
		sermod.AudioLink = null.NewString(serAudioLink, true)
	}
	sermod.Teacher = strings.TrimSpace(ser.Teacher)
	fmt.Println("[Debug] sermod.DateTaught:", sermod.DateTaught)  // debug
	sermod.Published = ser.Published
	sermod.Summary = null.NewString(strings.TrimSpace(ser.Summary), true)
	sermod.Body = null.NewString(strings.TrimSpace(ser.Body), true)
	sermod.UpdatedBy = strings.TrimSpace(ser.UpdatedBy)
	if len(ser.Categories) > 0 {
		// Do not add categories individually, build a slice of strings, then set categories equal to that
		// Otherwise the db field becomes a non-volatile accumulation of categories
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
