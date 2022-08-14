package sermon_controller

import (
	"bytes"
	"errors"
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
)

func NewSermon(c echo.Context) error {
	pg, err := page.SermonForm()
	if err != nil {
		c.Error(err)
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNew(c), map[string]map[string]string{}, app.IsLoggedIn(c))
	_ = c.HTMLBlob(200, buf.Bytes())
	return nil
}

func Import(c echo.Context) error {
	_ = c.JSONBlob(200, sermon.Import())
	return nil
}

// Show a particular sermon - for given by id
func ShowSermon(c echo.Context) error {
	pg, err := page.SermonShow()
	if err != nil {
		c.Error(err)
		return err
	}
	_ = c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return nil
}

func ListSermons(c echo.Context) error {
	pg, err := page.SermonsList()
	if err != nil {
		c.Error(err)
		return err
	}
	_ = c.HTMLBlob(200, base.RenderPageList(pg, c))
	return nil
}

func AdminListSermons(c echo.Context) error {
	pg, err := page.AdminSermonsList()
	if err != nil {
		c.Error(err)
		return err
	}
	_ = c.HTMLBlob(200, base.RenderPageList(pg, c))
	return nil
}

func EditSermon(c echo.Context) error {
	pg, err := page.SermonForm()
	if err != nil {
		c.Error(err)
		return err
	}
	_ = ctx.SetFormReferrer(c) // save the referrer calling for edit
	_ = c.HTMLBlob(200, base.RenderPageSingle(pg, c))
	return nil
}

func UpsertSermon(c echo.Context) error {
	const sermonsLocalFilePrefix = "sermons"
	const sermonsLocalURLPrefix = "media"
	const ftpUploadDelay = time.Second * 30

	var fileUploaded bool
	var localFilePath string
	csrf := c.FormValue("csrf")
	// Check that this token is present and valid in Redis
	if !app.VerifyFormToken(csrf) {
		err := errors.New("your form is expired. Go back to the form, refresh the page and try again")
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
	sermonFilename := sermonAudio.Filename // get filename by value, not ptr to form data stuff

	// fmt.Printf("|** %#v\n", sermonAudio)
	if err == nil && sermonAudio != nil && sermonFilename != "" {
		sermonTmp, err := sermonAudio.Open() // Todo: move to sermon model
		if err != nil {
			logger.LogErrAsync(err, "when", "opening sermon from FormFile", "filename", sermonFilename)
			c.Error(err)
			return err
		}
		defer func() { _ = sermonTmp.Close() }()
		localFilePath = path.Join(sermonsLocalFilePrefix, sermonFilename)
		initialUrlPath := path.Join(sermonsLocalURLPrefix, sermonFilename)
		dest, err := os.Create(localFilePath)
		if err != nil {
			logger.LogErrAsync(err, "when", "creating destination file for sermon", "filename", sermonFilename)
			c.Error(err)
			return err
		}
		defer func() { _ = dest.Close() }()
		fileUploaded = true

		// Copy to server
		if _, err := io.Copy(dest, sermonTmp); err != nil {
			logger.LogErrAsync(err, "when", "copying sermon from FormFile to dest", "filename", sermonFilename)
			c.Error(err)
			return err
		}
		serPres.AudioLink = fmt.Sprintf("/" + initialUrlPath) // todo URL encode on store
		logger.Log("info", "New sermon file uploaded", "upload_path", serPres.AudioLink)
	} else {
		if c.FormValue("audio-link-ovrd") == "on" {
			serPres.AudioLink = c.FormValue("audio_link")
			logger.Log("Info", "Audio link manually overidden to: "+serPres.AudioLink)
		} else {
			logger.Log("Debug", "Sermon updated, but audio file not updated")
		}
	}

	serPres.Categories = strings.Split(c.FormValue("categories"), ",")
	serPres.ScriptureRefs = strings.Split(c.FormValue("scripture_refs"), ",")
	serPres.UpdatedBy = c.(*ctx.CustomContext).Session.Username
	if c.FormValue("published") == "on" {
		serPres.Published = true
	}

	slug, err := serPres.Upsert()
	if err != nil {
		c.Error(err)
		return err
	}
	msg := "Created"
	if serPres.Id != "0" && serPres.Id != "" {
		msg = "Updated"
	}

	// TODO - use rerr and rlog
	if config.Options.FTP.Main.Enabled && fileUploaded { // Transfer to main sermon archive
		go func() {
			time.Sleep(ftpUploadDelay)

			logger.Log("Info", "Attempting xfer to FTP Server",
				"localFilePath", localFilePath,
				"sermonFilename", sermonFilename, "date taught", serPres.DateTaught,
			)

			upl := chftp.NewCemaUploader(localFilePath, sermonFilename, serPres.DateTaught)
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
		logger.LogErrAsync(err, msg, "when", "deleting sermon")
	}
	app.Redirect(c, "/admin/sermons", msg)
	return nil
}
