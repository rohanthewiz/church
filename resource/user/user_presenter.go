package user

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/serr"
)

type Presenter struct {
	Id                   string
	CreatedAt            string
	UpdatedAt            string
	UpdatedBy            string
	Enabled              bool
	Role                 int
	Username             string
	Firstname            string
	Lastname             string
	EmailAddress         string
	Summary              string
	Password             string
	PasswordConfirmation string
	EncryptedPassword    string
	EncryptedSalt        string
	ResetPasswordToken   string
	PasswordResetAt      time.Time
	ConfirmationToken    string
	ConfirmedAt          time.Time
	//Prefs
}

type role struct {
	SuperAdmin, Admin, Publisher, Author, RegisteredUser int
}

var Roles = role{99, 1, 5, 7, 9}

var RoleToString = map[int]string{99: "SuperAdmin", 1: "Admin", 5: "Publisher", 7: "Editor", 9: "RegisteredUser"}

func presenterFromModel(usr *model.User) (pres Presenter) {
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

// modelFromPresenter builds a user row from a form-submitted Presenter.
// Password handling: when a non-empty Password is present we generate a
// fresh salt and rehash — we never re-use an old salt on update, so a
// password change invalidates any cached hash.
func modelFromPresenter(pres Presenter) (usrmod *model.User, createOp bool, err error) {
	usrmod = findByIdOrCreate(pres.Id)
	if usrmod.ID < 1 {
		createOp = true
	}
	if pres.Password != "" {
		if pres.Password != pres.PasswordConfirmation {
			return usrmod, createOp, serr.Wrap(errors.New("Password and password confirmation do not match"))
		}
		salt := auth.GenSalt("MyRandomString$%@!@") // todo rand source
		usrmod.EncryptedSalt = sql.NullString{String: salt, Valid: true}
		usrmod.EncryptedPassword = sql.NullString{String: auth.PasswordHash(pres.Password, salt), Valid: true}
	}
	if createOp {
		// Username is write-once: uniqueness is enforced by DB constraint,
		// and external references (bookmarks, sessions) depend on stability.
		usrmod.Username = pres.Username
	}
	usrmod.UpdatedBy = pres.UpdatedBy
	usrmod.Enabled = pres.Enabled
	usrmod.Role = pres.Role
	usrmod.FirstName = pres.Firstname
	usrmod.LastName = sql.NullString{String: pres.Lastname, Valid: true}
	usrmod.EmailAddress = pres.EmailAddress
	usrmod.Summary = sql.NullString{String: pres.Summary, Valid: true}
	usrmod.PasswordResetAt = sql.NullTime{Time: pres.PasswordResetAt, Valid: !pres.PasswordResetAt.IsZero()}
	usrmod.ConfirmedAt = sql.NullTime{Time: pres.ConfirmedAt, Valid: !pres.ConfirmedAt.IsZero()}
	return
}

func presenterFromUsername(username string) (pres Presenter, err error) {
	m, err := findUserByUsername(username)
	if err != nil {
		return pres, serr.Wrap(err, "Error finding user by username")
	}
	pres = presenterFromModel(m)
	return
}
