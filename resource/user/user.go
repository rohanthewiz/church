package user

import (
	"strings"
	"gopkg.in/nullbio/null.v6"
	. "github.com/rohanthewiz/logger"
	. "github.com/vattle/sqlboiler/queries/qm"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/chweb/db"
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
