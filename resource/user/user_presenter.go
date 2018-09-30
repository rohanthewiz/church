package user

import (
	"fmt"
	"github.com/rohanthewiz/serr"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/chweb/config"
	"errors"
	"github.com/rohanthewiz/church/chweb/resource/auth"
	"gopkg.in/nullbio/null.v6"
	"time"
)

type Presenter struct {
	Id string
	CreatedAt string
	UpdatedAt string
	UpdatedBy string
	Enabled bool
	Role int
	Username string
	Firstname string
	Lastname string
	EmailAddress string
	Summary string
	Password string
	PasswordConfirmation string
	EncryptedPassword string
	EncryptedSalt string
	ResetPasswordToken string
	PasswordResetAt time.Time
	ConfirmationToken string
	ConfirmedAt time.Time
	//Prefs
}

type role struct {
	SuperAdmin, Admin, Publisher, Author, RegisteredUser int
}
var Roles = role{99, 1, 5, 7, 9}

var RoleToString = map[int]string{99: "SuperAdmin", 1: "Admin", 5: "Publisher", 7: "Editor", 9: "RegisteredUser"}

func presenterFromModel(usr *models.User) (pres Presenter) {
	if usr.CreatedAt.Valid {
		pres.CreatedAt = usr.CreatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	if usr.UpdatedAt.Valid {
		pres.UpdatedAt = usr.UpdatedAt.Time.Format(config.DisplayDateTimeFormat)
	}
	pres.UpdatedBy = usr.UpdatedBy
	pres.Enabled = usr.Enabled
	pres.Username = usr.Username
	pres.Role = usr.Role
	pres.Id = fmt.Sprintf("%d", usr.ID)
	pres.Summary = usr.Summary.String
	pres.Firstname = usr.FirstName
	pres.Lastname = usr.LastName.String
	pres.EmailAddress = usr.EmailAddress
	return
}

func modelFromPresenter(pres Presenter) (usrmod *models.User, createOp bool, err error) {
	usrmod = findByIdOrCreate(pres.Id)
	if usrmod.ID < 1 {
		createOp = true
	}
	if pres.Password != "" {  // we are setting or changing a password
		if pres.Password != pres.PasswordConfirmation {
			return usrmod, createOp, serr.Wrap(errors.New("Password and password confirmation do not match"))
		}
		salt := auth.GenSalt("MyRandomString$%@!@") // todo rand source
		usrmod.EncryptedSalt = null.NewString(salt, true)
		usrmod.EncryptedPassword = null.NewString(auth.PasswordHash(pres.Password, salt), true)
	}
	if createOp {
		usrmod.Username = pres.Username  // username should be unique and can only be set once
	}
	usrmod.UpdatedBy = pres.UpdatedBy
	usrmod.Enabled = pres.Enabled
	usrmod.Role = pres.Role
	usrmod.FirstName = pres.Firstname
	usrmod.LastName = null.NewString(pres.Lastname, true)
	usrmod.EmailAddress = pres.EmailAddress
	usrmod.Summary = null.NewString(pres.Summary, true)
	usrmod.PasswordResetAt = null.NewTime(pres.PasswordResetAt, true)
	usrmod.ConfirmedAt = null.NewTime(pres.ConfirmedAt, true)
	return
}

func presenterFromUsername(username string) (pres Presenter, err error) {
	model, err := findUserByUsername(username)
	if err != nil {
		return pres, serr.Wrap(err, "Error finding user by username")
	}
	pres = presenterFromModel(model)
	return
}
