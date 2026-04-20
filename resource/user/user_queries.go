package user

import (
	"strconv"

	"github.com/rohanthewiz/church/model"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

func (p Presenter) UpsertUser() error {
	usr, create, err := modelFromPresenter(p)
	if err != nil {
		LogErr(err, "Error in user from presenter")
		return err
	}
	if create {
		if err := model.InsertUser(usr); err != nil {
			LogErr(err, "Error inserting user into DB")
			return err
		}
		LogAsync("Info", "Successfully created user")
	} else {
		if err := model.UpdateUser(usr); err != nil {
			LogErr(err, "Error updating user in DB")
			return err
		}
		LogAsync("Info", "Successfully updated user")
	}
	return nil
}

// Returns a user model for id `id` or a new blank user model. The silent
// fallback-on-error matches the prior SQLBoiler path.
func findByIdOrCreate(id string) *model.User {
	if id == "" {
		return &model.User{}
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		LogErr(err, "Unable to convert User id to integer", "Id", id)
		return &model.User{}
	}
	m, err := findUserById(intId)
	if err != nil || m == nil {
		return &model.User{}
	}
	return m
}

func DeleteUserById(id string) error {
	const when = "When deleting user by id"
	if id == "" {
		return serr.NewSErr("Id to delete is empty string", "when", when)
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return serr.Wrap(err, "unable to convert User id to integer", "Id", id, "when", when)
	}
	if err := model.DeleteUser(intId); err != nil {
		return serr.Wrap(err, "Error when deleting user by id", "id", id, "when", when)
	}
	return nil
}

func findUserById(id int64) (*model.User, error) {
	return model.UserByID(id)
}

// NOTE: the legacy code queried column `user` here, which was almost
// certainly a typo for `username` (and would have failed since `user` is
// a Postgres reserved word with no column of that name). The DAO uses
// `username`; preserving the typo would just carry a latent bug forward.
func findUserByUsername(username string) (*model.User, error) {
	return model.UserByUsername(username)
}

func QueryUsers(condition, order string, limit, offset int64) (presenters []Presenter, err error) {
	users, err := model.QueryUsers(condition, order, limit, offset)
	if err != nil {
		return nil, serr.Wrap(err, "Error querying users")
	}
	for _, usr := range users {
		presenters = append(presenters, presenterFromModel(usr))
	}
	return
}
