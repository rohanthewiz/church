package sermon

import (
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// Upsert inserts or updates a sermon row derived from the presenter, and
// returns the slug of the persisted row. Slug is write-once on create (see
// modelFromPresenter), so on update we just echo back whatever is already
// stored.
func (p Presenter) Upsert() (slug string, err error) {
	ser, create, err := modelFromPresenter(p)
	if err != nil {
		logger.LogErr(err, "Error in sermon model from presenter")
		return slug, err
	}
	if create {
		if err = model.InsertSermon(ser); err != nil {
			logger.LogErr(err, "Error inserting sermon into DB")
			return slug, err
		}
		logger.Log("Info", "Successfully created sermon")
	} else {
		if err = model.UpdateSermon(ser); err != nil {
			logger.LogErr(err, "Error updating sermon in DB")
			return slug, err
		}
		logger.Log("Info", "Successfully updated sermon")
	}
	return ser.Slug.String, nil
}

func (p Presenter) GetYear() (year string) {
	year = timeutil.CurrentYear()
	if arr := strings.SplitN(p.DateTaught, "-", 2); len(arr) == 2 {
		year = arr[0]
	}
	return
}

// findByIdOrCreate returns a sermon model for `id` or a zero-valued model
// if the id is missing/invalid. Matching the silent-fallback contract used
// by the other resources — the caller inspects m.ID < 1 to decide create vs
// update.
func findByIdOrCreate(id string) *model.Sermon {
	if id == "" {
		return &model.Sermon{}
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		logger.LogErr(err, "Unable to convert Sermon id to integer", "Id", id)
		return &model.Sermon{}
	}
	m, err := findSermonById(intId)
	if err != nil || m == nil {
		return &model.Sermon{}
	}
	return m
}

func DeleteSermonById(id string) error {
	const when = "When deleting sermon by id"
	if id == "" {
		return serr.New("Id to delete is empty string", "when", when)
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Sermon id to integer", "Id", id, "when", when)
	}
	if err := model.DeleteSermon(intId); err != nil {
		return serr.Wrap(err, "Error when deleting sermon by id", "id", id, "when", when)
	}
	return nil
}

func findSermonById(id int64) (*model.Sermon, error) {
	return model.SermonByID(id)
}

func findSermonBySlug(slug string) (*model.Sermon, error) {
	return model.SermonBySlug(slug)
}

func QuerySermons(condition, order string, limit, offset int64) (presenters []Presenter, err error) {
	sermons, err := model.QuerySermons(condition, order, limit, offset)
	if err != nil {
		return nil, serr.Wrap(err, "Error querying sermons")
	}
	for _, ser := range sermons {
		presenters = append(presenters, presenterFromModel(ser))
	}
	return
}

func RecentSermons(limit int64) (presenters []Presenter, err error) {
	return QuerySermons("1 = 1", "created_at DESC", limit, 0)
}
