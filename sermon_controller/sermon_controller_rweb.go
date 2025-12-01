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

	"github.com/rohanthewiz/church/app"
	base "github.com/rohanthewiz/church/basectlr"
	"github.com/rohanthewiz/church/config"
	cctx "github.com/rohanthewiz/church/context"
	"github.com/rohanthewiz/church/core/idrive"
	"github.com/rohanthewiz/church/flash"
	"github.com/rohanthewiz/church/page"
	"github.com/rohanthewiz/church/resource/sermon"
	"github.com/rohanthewiz/church/template"
	"github.com/rohanthewiz/church/util/fileops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

func NewSermonRWeb(ctx rweb.Context) error {
	pg, err := page.SermonForm()
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	template.Page(buf, pg, flash.GetOrNewRWeb(ctx), map[string]map[string]string{
		"_global": {"user_agent": ctx.UserAgent()},
	}, app.IsLoggedInRWeb(ctx))
	return ctx.WriteHTML(buf.String())
}

func ImportRWeb(ctx rweb.Context) error {
	return ctx.WriteJSON(sermon.Import())
}

// Show a particular sermon - for given by id
func ShowSermonRWeb(ctx rweb.Context) error {
	pg, err := page.SermonShow()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageSingleRWeb(pg, ctx)))
}

func ListSermonsRWeb(ctx rweb.Context) error {
	pg, err := page.SermonsList()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageListRWeb(pg, ctx)))
}

func AdminListSermonsRWeb(ctx rweb.Context) error {
	pg, err := page.AdminSermonsList()
	if err != nil {
		return err
	}
	return ctx.WriteHTML(string(base.RenderPageListRWeb(pg, ctx)))
}

func EditSermonRWeb(ctx rweb.Context) error {
	pg, err := page.SermonForm()
	if err != nil {
		return err
	}
	cctx.SetFormReferrerRWeb(ctx) // save the referrer calling for edit
	return ctx.WriteHTML(string(base.RenderPageSingleRWeb(pg, ctx)))
}

func UpsertSermonRWeb(ctx rweb.Context) error {
	const sermonsURLPrefix = "sermon-audio"
	const cloudUploadDelay = time.Second * 45
	var fileUploaded bool
	var localFileSpec string

	csrf := ctx.Request().FormValue("csrf")
	// Check that this token is present and valid in Redis
	if !app.VerifyFormToken(csrf) {
		err := serr.New("Your form is expired. Go back to the form, refresh the page and try again")
		return err
	}
	// apparently embedded fields cannot be set immediately in a literal struct
	// we'll set those after the object is created
	serPres := sermon.Presenter{}
	serPres.Id = ctx.Request().FormValue("sermon_id")
	serPres.Title = ctx.Request().FormValue("sermon_title")
	serPres.Summary = ctx.Request().FormValue("sermon_summary")
	serPres.Body = ctx.Request().FormValue("sermon_body")
	serPres.DateTaught = ctx.Request().FormValue("sermon_date")
	serPres.PlaceTaught = ctx.Request().FormValue("sermon_place")
	serPres.Teacher = ctx.Request().FormValue("pastor-teacher")
	serPres.Categories = strings.Split(ctx.Request().FormValue("categories"), ",")
	serPres.ScriptureRefs = strings.Split(ctx.Request().FormValue("scripture_refs"), ",")

	// Get username from session
	sess, err := cctx.GetSessionFromRWeb(ctx)
	if err == nil && sess != nil {
		serPres.UpdatedBy = sess.Username
	}

	if ctx.Request().FormValue("published") == "on" {
		serPres.Published = true
	}
	serYear := serPres.GetYear()

	// Here we don't want to always err if form file is just not set
	sermonAudio, sermonHeader, err := ctx.Request().GetFormFile("sermon_audio")
	if err == nil && sermonAudio != nil && sermonHeader != nil && sermonHeader.Filename != "" { // If all conditions are good upload the sermon contents
		defer sermonAudio.Close()

		// Apparently sermonHeader.Filename is coming in url encoded
		filenameDecoded, err := url.QueryUnescape(sermonHeader.Filename)
		if err != nil {
			logger.LogErr(err, "when", "un-escaping filename", "filename", sermonHeader.Filename)
			return err
		}

		localFileSpec = path.Join(config.Options.IDrive.LocalSermonsDir, serYear, filenameDecoded)

		sermonDir := filepath.Dir(localFileSpec)
		err = fileops.EnsureDir(sermonDir)
		if err != nil {
			logger.LogErr(err, "error ensuring local directory exists for sermon", "localFileSpec", localFileSpec)
			return err
		}

		sermonAudioURL := path.Join(sermonsURLPrefix, serYear, sermonHeader.Filename)

		// Create empty local file
		dest, err := os.Create(localFileSpec)
		if err != nil {
			logger.LogErr(err, "when", "creating local destination file for sermon", "fileSpec", localFileSpec)
			return err
		}
		defer dest.Close()

		// Copy file contents
		if _, err := io.Copy(dest, sermonAudio); err != nil {
			logger.LogErr(err, "when", "copying sermon from FormFile to dest", "filename", sermonHeader.Filename)
			return err
		}

		fileUploaded = true

		serPres.AudioLink = "/" + sermonAudioURL
		logger.Info("New sermon file uploaded", "upload_path", serPres.AudioLink)

	} else { // We are not uploading a sermon, what else can we do?
		if ctx.Request().FormValue("audio-link-ovrd") == "on" {
			serPres.AudioLink = ctx.Request().FormValue("audio_link")
			logger.Warn("Audio link manually overridden to: " + serPres.AudioLink)
		} else {
			logger.Info("Sermon updated, but audio file not updated")
		}
	}

	// Save it
	slug, err := serPres.Upsert()
	// fmt.Printf("*|* serPres --> %#v\n", serPres)
	if err != nil {
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
	if sess != nil && sess.FormReferrer != "" {
		redirectTo = sess.FormReferrer // return to the form caller
	}
	return app.RedirectRWeb(ctx, redirectTo, "Sermon "+msg)
}

func DeleteSermonRWeb(ctx rweb.Context) error {
	err := sermon.DeleteSermonById(ctx.Request().PathParam("id"))
	msg := "Sermon with id: " + ctx.Request().PathParam("id") + " deleted"
	if err != nil {
		msg = "Error attempting to delete sermon with id: " + ctx.Request().PathParam("id")
		logger.LogErr(err, msg, "when", "deleting sermon")
	}
	return app.RedirectRWeb(ctx, "/admin/sermons", msg)
}
