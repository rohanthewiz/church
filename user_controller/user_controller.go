package user_controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	ctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/logger"
)

func NewUser(c echo.Context) error {
	pg, err := page.UserForm()
	if err != nil {
		c.Error(err)
		return err
	}
	c.HTMLBlob(200, base.RenderPageNew(pg, c))
	return nil
}

func ListUsers(c echo.Context) error {
	pg, err := page.UsersList()
	if err != nil {
		c.Error(err)
		return err
	}
	c.HTMLBlob(200, base.RenderPageList(pg, c))
	return nil
}

func EditUser(c echo.Context) error {
	pg, err := page.UserForm()
	if err != nil {
		c.Error(err)
		return err
	}
	c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return nil
}

func UpsertUser(c echo.Context) error {
	csrf := c.FormValue("csrf")
	// Check token valid against Redis
	if !app.VerifyFormToken(csrf) {
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		c.Error(err)
		return err
	}
	efs := user.Presenter{}
	efs.Id = c.FormValue("user_id")
	efs.Username = strings.TrimSpace(c.FormValue("username"))
	efs.EmailAddress = strings.TrimSpace(c.FormValue("email_address"))
	efs.Firstname = strings.TrimSpace(c.FormValue("firstname"))
	efs.Lastname = strings.TrimSpace(c.FormValue("lastname"))
	efs.Summary = c.FormValue("user_summary")
	efs.Password = c.FormValue("password")                     // do not trim space!
	efs.PasswordConfirmation = c.FormValue("password_confirm") // do not trim space!
	efs.UpdatedBy = c.(*ctx.CustomContext).Session.Username
	role, err := strconv.ParseInt(c.FormValue("role"), 10, 64)
	if err != nil {
		logger.LogErr(err, "Error converting role")
		return err
	}
	efs.Role = int(role)
	if c.FormValue("enabled") == "on" {
		efs.Enabled = true
	}

	err = efs.UpsertUser()
	if err != nil {
		logger.LogErr(err, "Error in user upsert", "user_presenter", fmt.Sprintf("%#v", efs))
		c.Error(err)
		return err
	}
	msg := "Created"
	if efs.Id != "0" && efs.Id != "" {
		msg = "Updated"
	}
	app.Redirect(c, "/admin/users", "User "+msg)
	return nil
}

func DeleteUser(c echo.Context) error {
	err := user.DeleteUserById(c.Param("id"))
	msg := "User with id: " + c.Param("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete user with id: " + c.Param("id")
		logger.LogErr(err, "when", "deleting user")
	}
	app.Redirect(c, "/admin/users", msg)
	return nil
}
