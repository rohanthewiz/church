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

func AllUsers() (models.UserSlice, error) {
	db, err := db.Db()
	if err != nil {
		return models.UserSlice{}, err
	}
	return models.Users(db).All()
}

func SaveUser(username string, phash, salt null.String, role int) error {
	var err error
	u := &models.User{
		Username: username,
		EncryptedPassword: phash,
		EncryptedSalt: salt,
		Role: role,
		Enabled: true,
		EmailAddress: "superadmin@thisSite.com",
		FirstName: "Super",
	}
	db, err := db.Db()
	if err != nil {
		return err
	}
	err = u.Insert(db)
	if err != nil {
		LogErr(err, "Failed to insert user into db")
		return err
	}
	Log("Info", "User Added", "Username", username, "Password hash", phash.String, "Salt", salt.String)
	return err
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
func AuthUserByUsername(username string) (au AuthUser, found bool, err error) {
	dbH, err := db.Db()
	if err != nil {
		return au, false, serr.Wrap(err, "Error obtaining DB handle")
	}
	usr, err := models.Users(dbH, Where("username = ? and enabled = ?", username, true)).One()
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
func UserCreds(username string) (string, string, error) {
	db, err := db.Db()
	if err != nil {
		return "", "", err
	}
	user, err := models.Users(db,
		Select("encrypted_password", "encrypted_salt"),
	Where("username = ? and enabled = ?", username, true)).One()
	if err != nil {
		return "", "", err
	}
	return user.EncryptedPassword.String, user.EncryptedSalt.String, err
}

func FullName(usr *models.User) string {
	out := strings.TrimSpace(strings.TrimSpace(usr.FirstName))
	if usr.LastName.Valid {
		out += " " + strings.TrimSpace(usr.LastName.String)
	}
	return out
}

func SuperAdminsExist() (bool, error) {
	db, err := db.Db()
	if err != nil {
		Log("Error", "Error obtaining db handle", "when", "opening db")
		return false, err
	}
	return models.Users(db, Where("role = ?", Roles.SuperAdmin)).Exists()
}
