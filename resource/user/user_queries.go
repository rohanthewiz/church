package user

import (
	"fmt"
	"strconv"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	. "github.com/vattle/sqlboiler/queries/qm"
)

func (p Presenter) UpsertUser(exec db.Executor) error {
	usr, create, err := modelFromPresenter(exec, p)
	if err != nil {
		LogErr(err, "Error in user from presenter")
		return err
	}
	if create {
		err = usr.Insert(exec)
		if err != nil {
			LogErr(err, "Error inserting user into DB")
			return err
		} else {
			LogAsync("Info", "Successfully created user")
		}
	} else {
		err = usr.Update(exec)
		if err != nil {
			LogErr(err, "Error updating user in DB")
		} else {
			LogAsync("Info", "Successfully updated user")
		}
	}
	return err
}

// Returns a user model for id `id` or a new user model
func findByIdOrCreate(exec db.Executor, id string) (model *models.User) {
	if id != "" {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			LogErr(err, "Unable to convert User id to integer", "Id", id)
			return new(models.User)
		}
		model, err = findUserById(exec, intId)
		if err != nil {
			return new(models.User)
		}
	}
	if model == nil {
		model = new(models.User)
	}
	return
}

func DeleteUserById(exec db.Executor, id string) error {
	const when = "When deleting user by id"
	if id == "" {
		return serr.NewSErr("Id to delete is empty string", "when", when)
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert User id to integer", "Id", id, "when", when)
	}
	err = models.Users(exec, Where("id=?", intId)).DeleteAll()
	if err != nil {
		return serr.Wrap(err, "Error when deleting user by id", "id", id, "when", when)
	}
	return nil
}

// Returns a user model for id `id` or error
func findUserById(exec db.Executor, id int64) (*models.User, error) {
	usr, err := models.Users(exec, Where("id = ?", id)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving user by id", "id", fmt.Sprintf("%d", id))
	}
	return usr, err
}

// Returns a user model username or error
func findUserByUsername(exec db.Executor, username string) (*models.User, error) {
	usr, err := models.Users(exec, Where("user = ?", username)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving user by username", "username", username)
	}
	return usr, err
}

func QueryUsers(exec db.Executor, condition, order string, limit, offset int64) (presenters []Presenter, err error) {
	users, err := models.Users(exec, Where(condition), OrderBy(order), Limit(int(limit)),
		Offset(int(offset))).All()
	if err != nil {
		return nil, serr.Wrap(err, "Error querying users")
	}
	for _, usr := range users {
		presenters = append(presenters, presenterFromModel(usr))
	}
	return
}
