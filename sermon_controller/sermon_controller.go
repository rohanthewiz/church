package sermon_controller

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/config"
	ctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/core/idrive"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/sermon"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/church/util/fileops"
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
	const sermonsURLPrefix = "sermon-audio"
	const cloudUploadDelay = time.Second * 45
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
	serYear := serPres.GetYear()

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

		// Apparently sermonAudio.Filename is coming in url encoded
		filenameDecoded, err := url.QueryUnescape(sermonAudio.Filename)
		if err != nil {
			logger.LogErr(err, "when", "un-escaping filename", "filename", sermonAudio.Filename)
			c.Error(err)
			return err
		}

		localFileSpec = path.Join(config.Options.IDrive.LocalSermonsDir, serYear, filenameDecoded)

		sermonDir := filepath.Dir(localFileSpec)
		err = fileops.EnsureDir(sermonDir)
		if err != nil {
			logger.LogErr(err, "error ensuring local directory exists for sermon", "localFileSpec", localFileSpec)
			c.Error(err)
			return err
		}

		sermonAudioURL := path.Join(sermonsURLPrefix, serYear, sermonAudio.Filename)

		// Create empty local file
		dest, err := os.Create(localFileSpec)
		if err != nil {
			logger.LogErr(err, "when", "creating local destination file for sermon", "fileSpec", localFileSpec)
			c.Error(err)
			return err
		}
		defer dest.Close()

		// Copy file contents
		if _, err := io.Copy(dest, sermonTmp); err != nil {
			logger.LogErr(err, "when", "copying sermon from FormFile to dest", "filename", sermonAudio.Filename)
			c.Error(err)
			return err
		}

		fileUploaded = true

		serPres.AudioLink = "/" + sermonAudioURL
		logger.Info("New sermon file uploaded", "upload_path", serPres.AudioLink)

	} else { // We are not uploading a sermon, what else can we do?
		if c.FormValue("audio-link-ovrd") == "on" {
			serPres.AudioLink = c.FormValue("audio_link")
			logger.Warn("Audio link manually overridden to: " + serPres.AudioLink)
		} else {
			logger.Info("Sermon updated, but audio file not updated")
		}
	}

	// Save it
	slug, err := serPres.Upsert()
	// fmt.Printf("*|* serPres --> %#v\n", serPres)
	if err != nil {
		c.Error(err)
		return err
	}

	msg := "Created"
	if serPres.Id != "0" && serPres.Id != "" {
		msg = "Updated"
	}

	if config.Options.IDrive.Enabled && fileUploaded { // Transfer to sermon archive
		go func() {
			time.Sleep(cloudUploadDelay)

			logger.Info("Transferring", localFileSpec, "to IDriveE2")
			err = idrive.PutSermonToIDrive(serYear, localFileSpec)
			if err != nil {
				logger.LogErr(err, "Error transferring sermon to IDriveE2", "sermon", localFileSpec)
				return
			}
			logger.Info("Sermon transferred to IDriveE2", "sermonFile", localFileSpec, "slug", slug)
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
