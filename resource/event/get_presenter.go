package event

import (
	"fmt"
	"github.com/rohanthewiz/church/models"
	"time"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/chweb/config"
	"github.com/rohanthewiz/church/chweb/resource/content"
)

type Presenter struct {
	content.Content
	EventDate string
	EventTime string
	Location string
	ContactPerson string
	ContactPhone string
	ContactEmail string
	ContactURL string
}

// Fix up Presenter for Web
func presenterFromModel(evt *models.Event) Presenter {
	pres := Presenter{}
	if evt.CreatedAt.Valid {
		pres.CreatedAt = evt.CreatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	if evt.UpdatedAt.Valid {
		pres.UpdatedAt = evt.UpdatedAt.Time.Format(config.DisplayDateTimeFormat)
	}

	// Generic Content
	pres.Title = evt.Title
	pres.Slug = evt.Slug

	cats := []string{}
	for _, cat := range evt.Categories {
		cats = append(cats, cat)
	}
	pres.Categories = cats
	pres.Id = fmt.Sprintf("%d", evt.ID)
	pres.Summary = evt.Summary.String
	pres.Body = evt.Body.String
	pres.Published = evt.Published
	pres.UpdatedBy = evt.UpdatedBy

	// Presenter specific content
	earliest_date, err := time.Parse("2006-01-02", "2010-01-01")
	if err == nil && evt.EventDate.After(earliest_date) {
		loc, err := time.LoadLocation("Local")
		if err != nil {
			Log("Error", "Failed to load location 'Local'")
		} else {
			localEventDate := evt.EventDate.In(loc)
			fmt.Println("[Debug] localEventDate.Format(config.IncomingDateTimeFormat):",
					localEventDate.Format(config.IncomingDateTimeFormat))  // debug
			pres.EventDate = localEventDate.Format(config.DisplayDateFormat)
			pres.EventTime = localEventDate.Format(config.DisplayTimeFormat)
		}
	}
	pres.Location = evt.EventLocation.String
	pres.ContactPerson = evt.ContactPerson.String
	pres.ContactPhone = evt.ContactPhone.String
	pres.ContactEmail = evt.ContactEmail.String
	pres.ContactURL = evt.ContactURL.String
	return pres
}
