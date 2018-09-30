package article

import (
	"github.com/rohanthewiz/church/resource/content"
	"strings"
	"github.com/rohanthewiz/serr"
	"github.com/rohanthewiz/church/models"
	"fmt"
	"github.com/rohanthewiz/church/config"
	"gopkg.in/nullbio/null.v6"
	"errors"
	"github.com/rohanthewiz/logger"
)

type Presenter struct {
	content.Content
	Page string  // slug of the page it should appear on
	Position int
}


func presenterFromSlug(slug string) (pres Presenter, err error) {
	model, err := findArticleBySlug(slug)
	if err != nil {
		return pres, serr.Wrap(err, "Error finding article by slug")
	}
	pres = presenterFromModel(model)
	return
}

func presenterFromId(id int64) (pres Presenter, err error) {
	model, err := findArticleById(id)
	if err != nil {
		return pres, serr.Wrap(err, "Unable to obtain article", "when", "finding Article by Id")
	}
	return presenterFromModel(model), nil
}

func PresentersFromIds(ids []int64) (presenters []Presenter, err error) {
	errs := []string{}
	for _, id := range ids {
		pres, er := presenterFromId(id)
		if er != nil {
			errs = append(errs, er.Error())
			logger.LogErr(er, "Error in article presenterFromId")
			continue
		}
		presenters = append(presenters, pres)
	}
	if len(errs) > 0 {
		err = errors.New(strings.Join(errs, ", "))
	}
	return
}

func presenterFromModel(model *models.Article) (pres Presenter) {
	if model.CreatedAt.Valid {
		pres.CreatedAt = model.CreatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	if model.UpdatedAt.Valid {
		pres.UpdatedAt = model.UpdatedAt.Time.Format(config.DisplayDateTimeFormat)
	}

	// Generic Content
	pres.Title = model.Title
	pres.Slug = model.Slug

	cats := []string{}
	for _, cat := range model.Categories {
		cats = append(cats, cat)
	}
	pres.Categories = cats
	pres.Id = fmt.Sprintf("%d", model.ID)
	pres.Summary = model.Summary
	pres.Body = model.Body.String
	pres.Published = model.Published
	pres.UpdatedBy = model.UpdatedBy
	return
}

func modelFromPresenter(pres Presenter) (model *models.Article, create_op bool, err error) {
	model = findModelByIdOrCreate(pres.Id)
	if model.ID < 1 {
		create_op = true
	}

	if title := strings.TrimSpace(pres.Title); title != "" {
		model.Title = title
		if create_op {  // Allow slug update only on create to maintain external references
			pres.CreateSlug() // could check ahead for uniqueness in Javascript
			model.Slug = pres.Slug  // pass in slug only on create - slug has unique constraint
		}
	} else {
		msg := "Article title is a required field when creating articles"
		return model, create_op, serr.Wrap(errors.New(msg))
	}
	model.Published = pres.Published
	model.Summary = strings.TrimSpace(pres.Summary)
	model.Body = null.NewString(strings.TrimSpace(pres.Body), true)
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
		model.Categories = []string{""}
	}
	return
}
