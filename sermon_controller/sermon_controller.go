package sermon_controller

import (
	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/flash"
	"bytes"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/church/app"
	ctx "github.com/rohanthewiz/church/context"
	"strings"
	"errors"
	"github.com/rohanthewiz/church/resource/sermon"
	"os"
	"io"
	"path"
	"fmt"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/church/config"
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/chftp"
	"time"
	"github.com/rohanthewiz/church/auth_controller"
)

func NewSermon(c echo.Context) error {
	pg, err := page.SermonForm()
	if err != nil {
		c.Error(err)
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{}, app.IsLoggedIn(c))
	c.HTMLBlob(200, buf.Bytes())
	return  nil
}

func Import(c echo.Context) error {
	c.JSONBlob(200, sermon.Import())
	return nil
}

// Show a particular sermon - for given by id
func ShowSermon(c echo.Context) error {
	pg, err := page.SermonShow()
	if err != nil {	c.Error(err); return err }
	c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return  nil
}

func ListSermons(c echo.Context) error {
	pg, err := page.SermonsList()
	if err != nil {	c.Error(err); return err }
	c.HTMLBlob(200, base.RenderPageList(pg, c))
	return  nil
}

func AdminListSermons(c echo.Context) error {
	pg, err := page.AdminSermonsList()
	if err != nil { c.Error(err); return err }
	c.HTMLBlob(200, base.RenderPageList(pg, c))
	return  nil
}

func EditSermon(c echo.Context) error {
	pg, err := page.SermonForm()
	if err != nil { c.Error(err); return err }
	auth_controller.SetFormReferrer(c) // save the referrer calling for edit
	c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return  nil
}

func UpsertSermon(c echo.Context) error {
	const sermonsLocalFilePrefix = "sermons"
	const sermonsLocalURLPrefix = "media"
	var fileUploaded bool
	var localFilePath string
	csrf := c.FormValue("csrf")
	// Check that this token is present and valid in Redis
	if !app.VerifyFormToken(csrf) {
		err := errors.New("Your form is expired. Go back to the form, refresh the page and try again")
		c.Error(err)
		return err
	}
	// apparently embedded fields cannot be set immediately in  a literal struct
	// we'll set those after efs is created
	serPres := sermon.Presenter{}
	// set embedded fields etc
	serPres.Id = c.FormValue("sermon_id")
	serPres.Title = c.FormValue("sermon_title")
	serPres.Summary = c.FormValue("sermon_summary")
	serPres.Body = c.FormValue("sermon_body")
	serPres.DateTaught = c.FormValue("sermon_date")
	serPres.PlaceTaught = c.FormValue("sermon_place")
	serPres.Teacher = c.FormValue("pastor-teacher")

	sermonAudio, err := c.FormFile("sermon_audio")
	//fmt.Printf("|** %#v\n", sermonAudio)
	if err == nil && sermonAudio != nil && sermonAudio.Filename != "" {
		sermonTmp, err := sermonAudio.Open()  // Todo: move to sermon model
		if err != nil {
			logger.LogErrAsync(err, "when", "opening sermon from FormFile", "filename", sermonAudio.Filename)
			c.Error(err)
			return err
		}
		defer sermonTmp.Close()
		localFilePath = path.Join(sermonsLocalFilePrefix, sermonAudio.Filename)
		initialUrlPath := path.Join(sermonsLocalURLPrefix, sermonAudio.Filename)
		dest, err := os.Create(localFilePath)
		if err != nil {
			logger.LogErrAsync(err, "when", "creating destination file for sermon", "filename", sermonAudio.Filename)
			c.Error(err)
			return err
		}
		defer dest.Close()
		fileUploaded = true

		// Copy to server
		if _, err := io.Copy(dest, sermonTmp); err != nil {
			logger.LogErrAsync(err, "when", "copying sermon from FormFile to dest", "filename", sermonAudio.Filename)
			c.Error(err)
			return err
		}
		serPres.AudioLink = fmt.Sprintf("/" + initialUrlPath) // todo URL encode on store
		logger.Log("info", "New sermon file uploaded", "upload_path", serPres.AudioLink)
	} else {
		fmt.Println("*|* sermon file not updated")
	}

	serPres.Categories = strings.Split(c.FormValue("categories"), ",")
	serPres.ScriptureRefs = strings.Split(c.FormValue("scripture_refs"), ",")
	serPres.UpdatedBy = c.(*ctx.CustomContext).Username
	if c.FormValue("published") == "on" {
		serPres.Published = true
	}
	//fmt.Printf("*|* serPres --> %#v\n", serPres)
	slug, err := serPres.Upsert()
	if err != nil {
		c.Error(err)
		return err
	}
	msg := "Created"
	if serPres.Id != "0" && serPres.Id != "" {
		msg = "Updated"
	}

	if config.Options.FTP.Main.Enabled && fileUploaded { // Transfer to main sermon archive
		go func() {
			time.Sleep(time.Second * 8) // todo adj
			upl := chftp.NewCemaUploader(localFilePath, sermonAudio.Filename)
			println("Transferring", localFilePath, "to Main FTP server")
			err := upl.Run()
			if err != nil {
				logger.LogErrAsync(err, "Error transferring to Church FTP", "sermon", localFilePath)
			} else {
				// Get Sermon Presenter by slug
				pres, err := sermon.PresenterFromSlug(slug)
				if err != nil {
					logger.LogErrAsync(err, "Error finding sermon by slug", "slug", slug)
				}
				pres.AudioLink = upl.DestWebPath()
				_, err = pres.Upsert()
				if err != nil {
					logger.LogErrAsync(err, "Error updating Sermon audio link to Church FTP server")
				}
				logger.Log("Info", "Sermon transferred to Church FTP server", "sermon_link", pres.AudioLink)
			}
		}()
	}
		// Backup will be similar
	redirectTo := "/admin/sermons"
	if cc, ok := c.(*ctx.CustomContext); ok && cc.FormReferrer != "" {
		redirectTo = cc.FormReferrer // return to the form caller
	}
	app.Redirect(c, redirectTo, "Sermon " + msg)
	return nil
}

func DeleteSermon(c echo.Context) error {
	err := sermon.DeleteSermonById(c.Param("id"))
	msg := "Sermon with id: " + c.Param("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete sermon with id: " + c.Param("id")
		logger.LogErrAsync(err, msg, "when", "deleting sermon")
	}
	app.Redirect(c, "/admin/sermons", msg)
	return nil
}
