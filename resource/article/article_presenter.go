package article

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/church/resource/content"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

type Presenter struct {
	content.Content
	Page     string // slug of the page it should appear on
	Position int
}

func presenterFromSlug(slug string) (pres Presenter, err error) {
	m, err := findArticleBySlug(slug)
	if err != nil {
		return pres, serr.Wrap(err, "Error finding article by slug")
	}
	pres = presenterFromModel(m)
	return
}

func presenterFromId(id int64) (pres Presenter, err error) {
	m, err := findArticleById(id)
	if err != nil {
		return pres, serr.Wrap(err, "Unable to obtain article", "when", "finding Article by Id")
	}
	return presenterFromModel(m), nil
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

// presenterFromModel converts the DB-shaped struct into the view-shaped
// Presenter. Any nullable/array unwrapping belongs here so the rest of the
// article package sees plain Go types.
func presenterFromModel(m *model.Article) (pres Presenter) {
	if m.CreatedAt.Valid {
		pres.CreatedAt = m.CreatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	if m.UpdatedAt.Valid {
		pres.UpdatedAt = m.UpdatedAt.Time.Format(config.DisplayDateTimeFormat)
	}

	pres.Title = m.Title
	pres.Slug = m.Slug

	// Copy out of pq.StringArray into a plain []string so downstream code
	// (templates, JSON marshal in the grid renderer) never sees the driver type.
	cats := make([]string, 0, len(m.Categories))
	for _, cat := range m.Categories {
		cats = append(cats, cat)
	}
	pres.Categories = cats

	pres.Id = fmt.Sprintf("%d", m.ID)
	pres.Summary = m.Summary
	pres.Body = m.Body.String
	pres.Published = m.Published
	pres.UpdatedBy = m.UpdatedBy
	return
}

// modelFromPresenter builds (or updates in place) the row we'll send to the
// DB. Returns create_op=true when the presenter has no id yet, so the caller
// can pick INSERT vs UPDATE without re-parsing the id.
func modelFromPresenter(pres Presenter) (m *model.Article, create_op bool, err error) {
	m = findModelByIdOrCreate(pres.Id)
	if m.ID < 1 {
		create_op = true
	}

	if title := strings.TrimSpace(pres.Title); title != "" {
		m.Title = title
		// Slugs are only generated on create to preserve any external
		// references/bookmarks built against the original slug.
		if create_op {
			pres.CreateSlug() // could check ahead for uniqueness in Javascript
			m.Slug = pres.Slug
		}
	} else {
		return m, create_op, serr.New("Article title is a required field when creating articles")
	}
	m.Published = pres.Published
	m.Summary = strings.TrimSpace(pres.Summary)
	m.Body = sql.NullString{String: strings.TrimSpace(pres.Body), Valid: true}
	m.UpdatedBy = strings.TrimSpace(pres.UpdatedBy)

	if len(pres.Categories) > 0 {
		// Rebuild the slice rather than appending to the existing column value
		// — otherwise an update would accumulate the previous categories
		// (the db column is non-volatile across the scan/update round-trip).
		categories := make([]string, 0, len(pres.Categories))
		for _, cat := range pres.Categories {
			if trimmed := strings.TrimSpace(cat); trimmed != "" {
				categories = append(categories, trimmed)
			}
		}
		m.Categories = categories
	} else {
		m.Categories = []string{""}
	}
	return
}