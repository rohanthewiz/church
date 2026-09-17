package chimage

// Durable storage for editor-uploaded article images.
//
// Images used to live only in the CWD-relative dist/img/, served by the
// /assets/ static handler. On Kubernetes that meant a dedicated volume mount,
// and the files were the one piece of site data not shipped to object storage.
// They now follow the sermon media pattern: IDrive e2 is the durable copy and
// the local directory is a cache.
//
//	editor save ─► ProcessInlineImages ─► storeImage
//	                                        ├─ write dist/img/<name>  (temp + rename)
//	                                        └─ IDrive enabled: PUT images/<name> (async)
//
//	GET /assets/img/<name> ─► ServeImageRWeb
//	                            ├─ local file present ─► serve it
//	                            ├─ IDrive enabled ─► GET images/<name> ─► cache locally ─► serve
//	                            └─ otherwise 404
//
// URLs are unchanged (/assets/img/<name>), so stored article HTML needs no
// rewrite. The /assets/img/:filename route is more specific than the static
// /assets/*path route, so the router picks it regardless of order.
//
// Names are content-addressed (<original>.<xxhash>.<ext>), so an object never
// changes once written: uploads can be fire-and-forget, a stale cache is
// impossible, and responses can be cached as immutable.
//
// With IDrive disabled everything behaves as before: local files only.
// test_scripts/images_to_e2 copies an existing dist/img/ up to e2.

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/core/s3ops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

// ObjectPrefix is the bucket prefix for images, beside the sermon year
// prefixes. Sermon keys start with a four-digit year, so they can't collide.
const ObjectPrefix = "images"

// LocalDir is where images are cached (and, with IDrive off, stored).
const LocalDir = localImagesFolder

// ObjectKey is the bucket key for an image name.
func ObjectKey(name string) string {
	return path.Join(ObjectPrefix, name)
}

// validName reports whether name is a plain image file name: no directory
// part, no leading dot. Request paths and editor-supplied filenames both pass
// through here before touching the filesystem or the bucket.
func validName(name string) bool {
	return name != "" && name == filepath.Base(name) && !strings.ContainsAny(name, `/\`) &&
		!strings.HasPrefix(name, ".")
}

// storeImage writes an image to the local directory and, when IDrive is
// enabled, copies it to e2 in the background. The local write is what the
// page renders from immediately, so it is the only step that can fail the
// image; an e2 failure is logged, and the local copy still serves.
func storeImage(name string, data []byte) error {
	if !validName(name) {
		return serr.New("invalid image name", "name", name)
	}
	localSpec := filepath.Join(LocalDir, name)
	if err := writeFileAtomic(localSpec, data); err != nil {
		return serr.Wrap(err, "could not write image locally", "file", localSpec)
	}
	if config.Options != nil && config.Options.IDrive.Enabled {
		go func() {
			if err := s3ops.PutFileToS3(ObjectPrefix, localSpec); err != nil {
				logger.LogErr(err, "Could not copy image to IDrive e2; it is served locally only", "file", localSpec)
			}
		}()
	}
	return nil
}

// writeFileAtomic writes data beside dest under a hidden temp name, then
// renames it into place, so a reader (or the e2 upload) never sees a partial
// file.
func writeFileAtomic(dest string, data []byte) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(dest)+".tmp-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmp.Name())
		if werr != nil {
			return werr
		}
		return cerr
	}
	// CreateTemp makes the file 0600; images are served by this process, but
	// match what os.WriteFile(…, 0644) used to produce for anyone inspecting
	// the directory
	_ = os.Chmod(tmp.Name(), 0644)
	if err := os.Rename(tmp.Name(), dest); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// loadImage returns an image's bytes: the local copy, or else the e2 object
// (cached locally for next time). found is false when neither has it.
func loadImage(name string) (data []byte, found bool, err error) {
	localSpec := filepath.Join(LocalDir, name)
	data, err = os.ReadFile(localSpec)
	if err == nil {
		return data, true, nil
	}
	if !os.IsNotExist(err) {
		return nil, false, serr.Wrap(err, "could not read local image", "file", localSpec)
	}
	if config.Options == nil || !config.Options.IDrive.Enabled {
		return nil, false, nil
	}

	exists, err := s3ops.ObjectExists(ObjectKey(name))
	if err != nil {
		return nil, false, serr.Wrap(err, "could not check image on IDrive e2", "key", ObjectKey(name))
	}
	if !exists {
		return nil, false, nil
	}
	data, err = s3ops.GetFileFromS3(ObjectKey(name))
	if err != nil {
		return nil, false, serr.Wrap(err, "could not fetch image from IDrive e2", "key", ObjectKey(name))
	}
	// Cache for the next request. A failed cache write costs a refetch, not
	// the response.
	if err := writeFileAtomic(localSpec, data); err != nil {
		logger.LogErr(err, "Could not cache image fetched from IDrive e2", "file", localSpec)
	}
	return data, true, nil
}

// imageTypes are the extensions served inline, by content type.
var imageTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".ico":  "image/x-icon",
}

// ServeImageRWeb serves GET /assets/img/:filename.
func ServeImageRWeb(ctx rweb.Context) error {
	// Path params arrive still percent-encoded, and file names keep the
	// editor's spelling (spaces included)
	name, err := url.PathUnescape(ctx.Request().PathParam("filename"))
	if err != nil || !validName(name) {
		return ctx.Status(http.StatusNotFound).WriteText("not found")
	}
	data, found, err := loadImage(name)
	if err != nil {
		logger.LogErr(err, "Error loading image", "name", name)
		// e2 unreachable is a server-side problem, not a missing image; 503
		// also keeps browsers and proxies from caching the failure as a 404
		return ctx.Status(http.StatusServiceUnavailable).WriteText("image temporarily unavailable")
	}
	if !found {
		return ctx.Status(http.StatusNotFound).WriteText("not found")
	}

	// Only raster image types are served as what their extension claims. The
	// extension is editor-supplied, and an .html or .svg name served with its
	// own type would be a page running on this site's origin.
	contentType, ok := imageTypes[strings.ToLower(filepath.Ext(name))]
	if !ok {
		contentType = "application/octet-stream"
		ctx.Response().SetHeader("Content-Disposition", "attachment")
	}
	ctx.Response().SetHeader("Content-Type", contentType)
	ctx.Response().SetHeader("Content-Length", strconv.Itoa(len(data)))
	// Content-addressed names never change, so the response can be cached for
	// good. nosniff because the type comes from an editor-supplied extension.
	ctx.Response().SetHeader("Cache-Control", "public, max-age=31536000, immutable")
	ctx.Response().SetHeader("X-Content-Type-Options", "nosniff")
	return ctx.Status(http.StatusOK).Bytes(data)
}
