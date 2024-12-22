package sermon_controller

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/chftp"
	"github.com/rohanthewiz/church/config"
	ctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/sermon"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
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
	return nil
}

func Import(c echo.Context) error {
	c.JSONBlob(200, sermon.Import())
	return nil
}

// Show a particular sermon - for given by id
func ShowSermon(c echo.Context) error {
	pg, err := page.SermonShow()
	if err != nil {
		c.Error(err)
		return err
	}
	c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return nil
}

func ListSermons(c echo.Context) error {
	pg, err := page.SermonsList()
	if err != nil {
		c.Error(err)
		return err
	}
	c.HTMLBlob(200, base.RenderPageList(pg, c))
	return nil
}

func AdminListSermons(c echo.Context) error {
	pg, err := page.AdminSermonsList()
	if err != nil {
		c.Error(err)
		return err
	}
	c.HTMLBlob(200, base.RenderPageList(pg, c))
	return nil
}

func EditSermon(c echo.Context) error {
	pg, err := page.SermonForm()
	if err != nil {
		c.Error(err)
		return err
	}
	ctx.SetFormReferrer(c) // save the referrer calling for edit
	c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return nil
}

func UpsertSermon(c echo.Context) error {
	const sermonsLocalFilePrefix = "sermons"
	const sermonsLocalURLPrefix = "media"
	const ftpUploadDelay = time.Second * 40
	var fileUploaded bool
	var localFileSpec string

	csrf := c.FormValue("csrf")
	// Check that this token is present and valid in Redis
	if !app.VerifyFormToken(csrf) {
		err := serr.New("Your form is expired. Go back to the form, refresh the page and try again")
		c.Error(err)
		return err
	}
	// apparently embedded fields cannot be set immediately in  a literal struct
	// we'll set those after the object is created
	serPres := sermon.Presenter{}
	serPres.Id = c.FormValue("sermon_id")
	serPres.Title = c.FormValue("sermon_title")
	serPres.Summary = c.FormValue("sermon_summary")
	serPres.Body = c.FormValue("sermon_body")
	serPres.DateTaught = c.FormValue("sermon_date")
	serPres.PlaceTaught = c.FormValue("sermon_place")
	serPres.Teacher = c.FormValue("pastor-teacher")
	serPres.Categories = strings.Split(c.FormValue("categories"), ",")
	serPres.ScriptureRefs = strings.Split(c.FormValue("scripture_refs"), ",")
	serPres.UpdatedBy = c.(*ctx.CustomContext).Session.Username
	if c.FormValue("published") == "on" {
		serPres.Published = true
	}

	// Here we don't want to always err if form file is just not set
	sermonAudio, err := c.FormFile("sermon_audio")
	if err == nil && sermonAudio != nil && sermonAudio.Filename != "" { // If all conditions are good upload the sermon contents
		sermonTmp, err := sermonAudio.Open()
		if err != nil {
			logger.LogErr(err, "when", "opening sermon from FormFile", "filename", sermonAudio.Filename)
			c.Error(err)
			return err
		}
		defer sermonTmp.Close()

		serYear := serPres.GetYear()

		localFileSpec = path.Join(config.Options.IDrive.LocalSermonsDir, serYear, sermonAudio.Filename)
		initialUrlPath := path.Join(sermonsLocalURLPrefix, serYear, sermonAudio.Filename)

		// Create empty local file
		dest, err := os.Create(localFileSpec)
		if err != nil {
			logger.LogErr(err, "when", "creating local destination file for sermon", "fileSpec", localFileSpec)
			c.Error(err)
			return err
		}
		defer dest.Close()

		// Copy to server
		if _, err := io.Copy(dest, sermonTmp); err != nil {
			logger.LogErr(err, "when", "copying sermon from FormFile to dest", "filename", sermonAudio.Filename)
			c.Error(err)
			return err
		}

		fileUploaded = true

		serPres.AudioLink = fmt.Sprintf("/" + initialUrlPath) // todo URL encode on store
		logger.Log("info", "New sermon file uploaded", "upload_path", serPres.AudioLink)

	} else { // We are not uploading a sermon, what else can we do?
		if c.FormValue("audio-link-ovrd") == "on" {
			serPres.AudioLink = c.FormValue("audio_link")
			logger.Log("Info", "Audio link manually overidden to: "+serPres.AudioLink)
		} else {
			logger.Log("Debug", "Sermon updated, but audio file not updated")
		}
	}
	// fmt.Printf("*|* serPres --> %#v\n", serPres)
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
			time.Sleep(ftpUploadDelay)

			upl := chftp.NewCemaUploader(localFileSpec, sermonAudio.Filename, serPres.DateTaught)
			println("Transferring", localFileSpec, "to Main FTP server")
			err := upl.Run()
			if err != nil {
				logger.LogErr(err, "Error transferring to Church FTP", "sermon", localFileSpec)
			} else {
				pres, err := sermon.PresenterFromSlug(slug)
				if err != nil {
					logger.LogErr(err, "Error finding sermon by slug", "slug", slug)
				}
				pres.AudioLink = upl.DestWebPath()

				_, err = pres.Upsert()
				if err != nil {
					logger.LogErr(err, "Error updating Sermon audio link to Church FTP server")
				}
				logger.Log("Info", "Sermon transferred to Church FTP server", "sermon_link", pres.AudioLink)
			}
		}()
	}

	// Backup will be similar
	redirectTo := "/admin/sermons"
	if cc, ok := c.(*ctx.CustomContext); ok && cc.Session.FormReferrer != "" {
		redirectTo = cc.Session.FormReferrer // return to the form caller
	}
	app.Redirect(c, redirectTo, "Sermon "+msg)
	return nil
}

func DeleteSermon(c echo.Context) error {
	err := sermon.DeleteSermonById(c.Param("id"))
	msg := "Sermon with id: " + c.Param("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete sermon with id: " + c.Param("id")
		logger.LogErr(err, msg, "when", "deleting sermon")
	}
	app.Redirect(c, "/admin/sermons", msg)
	return nil
}
