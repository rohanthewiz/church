package admin

import (
	"errors"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/logger"
	"gopkg.in/nullbio/null.v6"
	"os"
)

var SuperToken string

const tokenFile = "token.txt"

func AuthBootstrap() {
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "Error obtaining DB handle for auth bootstrap")
		return
	}
	// If no superadmins exists then we are likely starting the app for the first time
	exists, err := user.SuperAdminsExist(dbH)
	if err != nil {
		logger.LogErr(err, "Error querying for superadmin")
	}
	if !exists {
		SuperToken = auth.RandomKey()
		// Owner read/write only: whoever holds this token can create the
		// superadmin. os.ModePerm (0777, minus umask) left it world-readable and
		// executable. The explicit Chmod is not redundant: WriteFile applies perm
		// only when it creates the file, and a token.txt from an earlier boot keeps
		// its old mode through the overwrite.
		if err := os.WriteFile(tokenFile, []byte(SuperToken), 0600); err != nil {
			logger.LogErr(err, "Error writing superadmin token file", "file", tokenFile)
			return
		}
		if err := os.Chmod(tokenFile, 0600); err != nil {
			logger.LogErr(err, "Error restricting superadmin token file permissions", "file", tokenFile)
		}
		logger.Log("info", "superadmin token created in <project root>/"+tokenFile)
	}
}

func CreateSuperUser(username, password string) (err error) {
	dbH, err := db.Db()
	if err != nil {
		return errors.New("Error obtaining DB handle for super user creation")
	}
	salt := auth.GenSalt("j$&@randomness!!$$$")
	pass_hash := auth.PasswordHash(password, salt)
	err = user.SaveUser(dbH, username, null.NewString(pass_hash, true), null.NewString(salt, true), user.Roles.SuperAdmin)
	if err != nil {
		return errors.New("Error saving super user")
	}
	SuperToken = "" // no one else can use the superadmin bootstrap
	os.Remove(tokenFile)

	return
}

// This was a consideration - we will likely not use this approach
//func BootstrapSuperUser() (err error) {
// If superuser exists return nil

// If superuser does not exist

// Write token to file (super_token.txt)

// Present form requesting token and desired password

// On token match
// Create superuser with supplied password

// Redirect to login
//	return
//}
