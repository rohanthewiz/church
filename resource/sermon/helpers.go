package sermon

import (
	"path/filepath"

	"github.com/rohanthewiz/church/config"
)

// GetRelAndLocalFileSpecs returns some filespecs to be used as cloud key and caching respectively.
// relFileSpec is the file spec of the sermon from a given root
// it is used as the key for cloud storage and joined with a local root to form localFileSpec for caching sermons
func GetRelAndLocalFileSpecs(year, fName string) (relativeFileSpec, localFileSpec string) {
	relativeFileSpec = filepath.Join(year, fName)
	localFileSpec = filepath.Join(config.Options.IDrive.LocalSermonsDir, relativeFileSpec)
	return
}
