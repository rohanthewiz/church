package fileops

import (
	"os"

	"github.com/rohanthewiz/serr"
)

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
