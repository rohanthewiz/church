package sermon

import (
	"fmt"
	"strconv"
	"github.com/rohanthewiz/serr"
	. "github.com/rohanthewiz/logger"
	. "github.com/vattle/sqlboiler/queries/qm"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
)

func (p Presenter) Upsert() (slug string, err error) {
	dbH, err := db.Db()
	if err != nil {
		return  slug, err
	}
	ser, create, err := modelFromPresenter(p)
	if err != nil {
		LogErr(err, "Error in sermon from presenter")
		return slug, err
	}
	fmt.Printf("In Upsert: sermon model (from presenter) %#v\n", ser)
	if create {
		err = ser.Insert(dbH)
		if err != nil {
			LogErr(err, "Error inserting sermon into DB")
			return slug, err
		} else {
			Log("Info", "Successfully created sermon")
		}
	} else {
		err = ser.Update(dbH)
		if err != nil {
			LogErr(err, "Error updating sermon in DB")
		} else {
			Log("Info", "Successfully updated sermon")
		}
	}
	return ser.Slug.String, err
}

// Returns a sermon model for id `id` or a new sermon model
func findByIdOrCreate(id string) (model *models.Sermon) {
	if id != "" {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			LogErr(err, "Unable to convert Sermon id to integer", "Id", id)
			return new(models.Sermon)
		}
		model, err = findSermonById(intId)
		if err != nil {
			return new(models.Sermon)
		}
	}
	if model == nil {
		model = new(models.Sermon)
	}
	return
}

func DeleteSermonById(id string) error {
	const when = "When deleting sermon by id"
	dbH, err := db.Db()
	if err != nil {
		return  err
	}
	if id == "" { return serr.NewSErr("Id to delete is empty string", "when", when) }
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert Sermon id to integer", "Id", id, "when", when)
	}
	err = models.Sermons(dbH, Where("id=?", intId)).DeleteAll()
	if err != nil {
		return serr.Wrap(err, "Error when deleting sermon by id", "id", id, "when", when)
	}
	return nil
}

// Returns a sermon model for id `id` or error
func findSermonById(id int64) (*models.Sermon, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, err
	}
	ser, err := models.Sermons(dbH, Where("id = ?", id)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving sermon by id", "id", fmt.Sprintf("%d", id))
	}
	return ser, err
}

// Returns a sermon model slug or error
func findSermonBySlug(slug string) (*models.Sermon, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Error obtaining DB handle")
	}
	art, err := models.Sermons(dbH, Where("slug = ?", slug)).One()
	if  err != nil {
		return nil, serr.Wrap(err, "Error retrieving sermon by slug", "slug", slug)
	}
	return art, err
}

func QuerySermons(condition, order string, limit, offset int64) (presenters []Presenter, err error) {
	Log("Debug", "Sermon query", "condition:", condition, " order:", order,
		" limit:", fmt.Sprintf("%d", limit), " offset:", fmt.Sprintf("%d", offset))
	dbH, err := db.Db()
	if err != nil {
		return
	}
	sermons, err := models.Sermons(dbH, Where(condition), OrderBy(order), Limit(int(limit)),
			Offset(int(offset))).All()
	if err != nil {
		return nil, serr.Wrap(err, "Error querying sermons")
	}
	for _, ser := range sermons {
		presenters = append(presenters, presenterFromModel(ser))
	}
	return
}

func RecentSermons(limit int64) (presenters []Presenter, err error) {
	condition := "1 = 1"
	order := "created_at DESC"
	return QuerySermons(condition, order, limit, 0)
}
