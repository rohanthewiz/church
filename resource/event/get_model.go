package event

import (
	"github.com/rohanthewiz/church/models"
	. "github.com/rohanthewiz/logger"
	"gopkg.in/nullbio/null.v6"
	"strings"
	"time"
	"errors"
	"fmt"
	"github.com/rohanthewiz/church/chweb/config"
)

// Fixup Received data for Presenter
func modelFromPresenter(pres Presenter) (*models.Event, bool, error) {
	var create_op bool  // inits to false
	model := findModelByIdOrCreate(pres.Id)
	if model.ID < 1 {
		create_op = true
	}

	if title := strings.TrimSpace(pres.Title); title != "" {
		model.Title = title
	} else {
		msg := "Presenter title is a required field when creating events"
		Log("Error", msg)
		return model, create_op, errors.New(msg)
	}
	// Todo - do this for other resources
	if create_op {  // Allow slug update only on create to maintain external references
		pres.CreateSlug() // could check ahead for uniqueness in Javascript, but good randomness should get us by
		model.Slug = pres.Slug  // we update slug only on create - slug has unique constraint
	}
	zone, _ := time.Now().Zone()  // server timezone should be good enough? I hope!
	datetimez := pres.EventDate + " " + pres.EventTime + " " + zone
	fmt.Println("[Debug] datetimez:", datetimez)  // debug
	dte, err := time.Parse(config.IncomingDateTimeFormat, datetimez)
	if err != nil {
		Log("Error", "Error parsing event date", "error", err.Error())
		return model, create_op, err
	}
	model.EventDate = dte
	model.EventTime = strings.TrimSpace(pres.EventTime) // todo - could eliminate this db attrib
	model.Published = pres.Published
	model.Summary = null.NewString(strings.TrimSpace(pres.Summary), true)
	model.Body = null.NewString(strings.TrimSpace(pres.Body), true)
	model.EventLocation = null.NewString(strings.TrimSpace(pres.Location), true)
	model.ContactPerson = null.NewString(strings.TrimSpace(pres.ContactPerson), true)
	model.ContactPhone = null.NewString(strings.TrimSpace(pres.ContactPhone), true)
	model.ContactEmail = null.NewString(strings.TrimSpace(pres.ContactEmail), true)
	model.ContactURL = null.NewString(strings.TrimSpace(pres.ContactURL), true)
	model.UpdatedBy = strings.TrimSpace(pres.UpdatedBy)
	if len(pres.Categories) > 0 {
		// Do not add categories individually, build a slice of strings, then set categories equal to that
		// Otherwise the db field becomes a non-volatile accumulation of categories
		categories := []string{}
		for _, cat := range pres.Categories {
			if trimmed := strings.TrimSpace(cat); trimmed != "" {
				categories = append(categories, trimmed)
			}
		}
		model.Categories = categories
	} else {
		model.Categories = []string{"general"}
	}
	return model, create_op, err
}
