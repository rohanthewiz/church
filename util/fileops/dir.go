package fileops

import (
	"os"

	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// EnsureDir ensures that the given directory exists
func EnsureDir(entry string) (err error) {
	isDir, _ := IsDir(entry)
	if !isDir {
		err = os.Mkdir(entry, 0750)
		if err != nil && !os.IsExist(err) {
			logger.LogErr(serr.Wrap(err, "Error creating directory for sermon year"))
			return
		}
		logger.Info("Successfully created directory " + entry)
	}
	return
}

func IsDir(entry string) (isDir bool, err error) {
	info, err := os.Stat(entry)
	if os.IsNotExist(err) {
		// Directory does not exist
		return false, nil
	} else if err != nil {
		// Other error occurred
		return false, serr.Wrap(err, "error occurred when ascertaining if is directory")
	} else if !info.IsDir() {
		// Path exists but is not a directory
		// fmt.Println("Path exists but is not a directory")
		return false, nil
	} else {
		// It's a directory
		return true, nil
	}
}
