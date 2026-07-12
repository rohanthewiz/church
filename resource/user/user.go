package user

import (
	"database/sql"
	"strings"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	. "github.com/vattle/sqlboiler/queries/qm"
	"gopkg.in/nullbio/null.v6"
)

// Query functions take the executor as their first parameter (db.Executor —
// see db/executor.go) rather than reaching for the db.Db() global themselves.
// Callers at request/bootstrap boundaries fetch the handle once and pass it
// down, which is what lets these run against a transaction or a sqlmock.

func AllUsers(exec db.Executor) (models.UserSlice, error) {
	return models.Users(exec).All()
}

func SaveUser(exec db.Executor, username string, phash, salt null.String, role int) error {
	u := &models.User{
		Username:          username,
		EncryptedPassword: phash,
		EncryptedSalt:     salt,
		Role:              role,
		Enabled:           true,
		EmailAddress:      "superadmin@thisSite.com",
		FirstName:         "Super",
	}
	err := u.Insert(exec)
	if err != nil {
		LogErr(err, "Failed to insert user into db")
		return err
	}
	Log("Info", "User Added", "Username", username, "Password hash", phash.String, "Salt", salt.String)
	return nil
}

// AuthUser is the identity + credential view needed by the API login flow
// (resource/apitoken). It exists so callers never touch models.User or
// user.Presenter directly — both carry credential fields that must not leak
// into a serializer by accident; AuthUser is explicit about holding them and
// is never serialized itself (the API layer maps it to its own DTO).
type AuthUser struct {
	ID           int64
	Username     string
	FirstName    string
	LastName     string
	EmailAddress string
	Role         int
	PassHash     string // scrypt hash (see resource/auth)
	Salt         string
}

// AuthUserByUsername loads an enabled user's identity and credentials for
// login verification. found=false (no error) when the username doesn't exist
// or the account is disabled — callers answer both identically so responses
// don't become a username oracle.
func AuthUserByUsername(exec db.Executor, username string) (au AuthUser, found bool, err error) {
	usr, err := models.Users(exec, Where("username = ? and enabled = ?", username, true)).One()
	if err != nil {
		// SQLBoiler v2 wraps the sentinel, so unwrap by message. "No such
		// user" is a normal outcome; anything else is a real infra error.
		if strings.Contains(err.Error(), sql.ErrNoRows.Error()) {
			return au, false, nil
		}
		return au, false, serr.Wrap(err, "Error loading user for auth", "username", username)
	}
	return AuthUser{
		ID:           usr.ID,
		Username:     usr.Username,
		FirstName:    usr.FirstName,
		LastName:     usr.LastName.String,
		EmailAddress: usr.EmailAddress,
		Role:         usr.Role,
		PassHash:     usr.EncryptedPassword.String,
		Salt:         usr.EncryptedSalt.String,
	}, true, nil
}

// Return user's stored credentials
func UserCreds(exec db.Executor, username string) (string, string, error) {
	user, err := models.Users(exec,
		Select("encrypted_password", "encrypted_salt"),
		Where("username = ? and enabled = ?", username, true)).One()
	if err != nil {
		return "", "", err
	}
	return user.EncryptedPassword.String, user.EncryptedSalt.String, nil
}

func FullName(usr *models.User) string {
	out := strings.TrimSpace(strings.TrimSpace(usr.FirstName))
	if usr.LastName.Valid {
		out += " " + strings.TrimSpace(usr.LastName.String)
	}
	return out
}

func SuperAdminsExist(exec db.Executor) (bool, error) {
	return models.Users(exec, Where("role = ?", Roles.SuperAdmin)).Exists()
}
