package sermon

import (
	"net/url"
	"path/filepath"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/logger"
)

// GetRelAndLocalFileSpecs returns some filespecs to be used as cloud key and caching respectively.
// relativeFileSpec is the file spec of the sermon from a given root
// it is used as 1) the key for cloud storage and 2) joined with a local root to form localFileSpec for caching sermons
func GetRelAndLocalFileSpecs(year, fName string) (relativeFileSpec, localFileSpec string) {
	fDecoded, err := url.QueryUnescape(fName) // Unescape file name that were saved URL encoded
	if err != nil {
		logger.LogErr(err, "when", "un-escaping filename", "filename", fName)
		return
	}

	relativeFileSpec = filepath.Join(year, fDecoded)
	localFileSpec = filepath.Join(config.Options.IDrive.LocalSermonsDir, relativeFileSpec)
	return
}
