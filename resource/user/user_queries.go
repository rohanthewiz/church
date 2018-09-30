package user

import (
	"github.com/rohanthewiz/church/models"
	"strconv"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
	"fmt"
	. "github.com/vattle/sqlboiler/queries/qm"
)

func (p Presenter) UpsertUser() error {
	db, err := db.Db()
	if err != nil {
		return  err
	}
	usr, create, err := modelFromPresenter(p)
	if err != nil {
		LogErr(err, "Error in user from presenter")
		return err
	}
	if create {
		err = usr.Insert(db)
		if err != nil {
			LogErr(err, "Error inserting user into DB")
			return err
		} else {
			LogAsync("Info", "Successfully created user")
		}
	} else {
		err = usr.Update(db)
		if err != nil {
			LogErr(err, "Error updating user in DB")
		} else {
			LogAsync("Info", "Successfully updated user")
		}
	}
	return err
}

// Returns a user model for id `id` or a new user model
func findByIdOrCreate(id string) (model *models.User) {
	if id != "" {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			LogErr(err, "Unable to convert User id to integer", "Id", id)
			return new(models.User)
		}
		model, err = findUserById(intId)
		if err != nil {
			return new(models.User)
		}
	}
	if model == nil {
		model = new(models.User)
	}
	return
}


func DeleteUserById(id string) error {
	const when = "When deleting user by id"
	dbH, err := db.Db()
	if err != nil {
		return  err
	}
	if id == "" { return serr.NewSErr("Id to delete is empty string", "when", when) }
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert User id to integer", "Id", id, "when", when)
	}
	err = models.Users(dbH, Where("id=?", intId)).DeleteAll()
	if err != nil {
		return serr.Wrap(err, "Error when deleting user by id", "id", id, "when", when)
	}
	return nil
}

// Returns a user model for id `id` or error
func findUserById(id int64) (*models.User, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, err
	}
	usr, err := models.Users(dbH, Where("id = ?", id)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving user by id", "id", fmt.Sprintf("%d", id))
	}
	return usr, err
}

// Returns a user model username or error
func findUserByUsername(username string) (*models.User, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Error obtaining DB handle")
	}
	usr, err := models.Users(dbH, Where("user = ?", username)).One()
	if  err != nil {
		return nil, serr.Wrap(err, "Error retrieving user by username", "username", username)
	}
	return usr, err
}

func QueryUsers(condition, order string, limit, offset int64) (presenters []Presenter, err error) {
	Log("Debug", "User query", "condition:", condition, " order:", order,
		" limit:", fmt.Sprintf("%d", limit), " offset:", fmt.Sprintf("%d", offset))
	dbH, err := db.Db()
	if err != nil {
		return
	}
	users, err := models.Users(dbH, Where(condition), OrderBy(order), Limit(int(limit)),
		Offset(int(offset))).All()
	if err != nil {
		return nil, serr.Wrap(err, "Error querying users")
	}
	for _, usr := range users {
		presenters = append(presenters, presenterFromModel(usr))
	}
	return
}
