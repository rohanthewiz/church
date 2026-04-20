package user

import (
	"database/sql"
	"strings"

	"github.com/rohanthewiz/church/model"
	. "github.com/rohanthewiz/logger"
)

// AllUsers returns every row in the users table. Read-only and used from
// admin-only code paths, so the lack of pagination is intentional.
func AllUsers() ([]*model.User, error) {
	return model.AllUsers()
}

// SaveUser creates a user with only the minimal fields a bootstrap /
// registration path supplies. Password hash + salt are taken as plain
// strings now — the old null.String type in the signature served no
// purpose because callers always passed Valid=true.
func SaveUser(username, passHash, salt string, role int) error {
	u := &model.User{
		Username:          username,
		EncryptedPassword: sql.NullString{String: passHash, Valid: true},
		EncryptedSalt:     sql.NullString{String: salt, Valid: true},
		Role:              role,
		Enabled:           true,
		EmailAddress:      "superadmin@thisSite.com",
		FirstName:         "Super",
	}
	if err := model.InsertUser(u); err != nil {
		LogErr(err, "Failed to insert user into db")
		return err
	}
	Log("Info", "User Added", "Username", username, "Password hash", passHash, "Salt", salt)
	return nil
}

// UserCreds returns the stored password hash + salt for the named user,
// but only if the user is enabled. Disabled accounts look identical to
// "no such user" to the caller — intentional, so login paths can't
// fingerprint enabled vs disabled state.
func UserCreds(username string) (string, string, error) {
	return model.UserCredsByUsername(username)
}

func FullName(usr *model.User) string {
	out := strings.TrimSpace(usr.FirstName)
	if usr.LastName.Valid {
		out += " " + strings.TrimSpace(usr.LastName.String)
	}
	return out
}

func SuperAdminsExist() (bool, error) {
	return model.ExistsUserWithRole(Roles.SuperAdmin)
}
