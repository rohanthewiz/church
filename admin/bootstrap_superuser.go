package admin

import (
	"errors"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/logger"
	"gopkg.in/nullbio/null.v6"
	"io/ioutil"
	"os"
)

var SuperToken string
const tokenFile = "token.txt"

func AuthBootstrap() {
	// If no superadmins exists then we are likely starting the app for the first time
	exists, err := user.SuperAdminsExist()
	if err != nil { logger.Log("Fatal", "Error querying for superadmin") }
	if !exists {
		SuperToken = auth.RandomKey()
		ioutil.WriteFile("token.txt", []byte(SuperToken), os.ModePerm)
		logger.Log("info", "superadmin token created in <project root>/" + tokenFile)
	}
}

func CreateSuperUser(username, password string) (err error) {
	salt := auth.GenSalt("j$&@randomness!!$$$")
	pass_hash := auth.PasswordHash(password, salt)
	err = user.SaveUser(username, null.NewString(pass_hash, true), null.NewString(salt, true), user.Roles.SuperAdmin)
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
