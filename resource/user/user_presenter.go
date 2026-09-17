package user

import (
	"fmt"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/church/util/inputerr"
	"github.com/rohanthewiz/serr"
	"gopkg.in/nullbio/null.v6"
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

func modelFromPresenter(exec db.Executor, pres Presenter) (usrmod *models.User, createOp bool, err error) {
	usrmod = findByIdOrCreate(exec, pres.Id)
	if usrmod.ID < 1 {
		createOp = true
	}
	if pres.Password != "" {  // we are setting or changing a password
		if pres.Password != pres.PasswordConfirmation {
			// An InputError, so the controller shows this on the form. Refused
			// before any write.
			return usrmod, createOp, serr.Wrap(inputerr.New("Password and password confirmation do not match", nil))
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

func presenterFromUsername(exec db.Executor, username string) (pres Presenter, err error) {
	model, err := findUserByUsername(exec, username)
	if err != nil {
		return pres, serr.Wrap(err, "Error finding user by username")
	}
	pres = presenterFromModel(model)
	return
}

// FormDraft is what a refused user save hands back to the user form (see
// core/formdraft). The presenter never carries a password: the controller
// blanks both password fields before saving the draft, so a typed password
// is never kept in the store.
type FormDraft struct {
	Presenter
	// RolesPosted is false when the roles couldn't be listed at refusal time;
	// the form then shows the stored assignments. When true, RoleIDs are the
	// ticked boxes, which the form applies to the roles the viewer may grant
	// (locked roles always show their stored state).
	RolesPosted bool
	RoleIDs     []int64
}
