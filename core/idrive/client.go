package idrive

import (
	"os"
	"path/filepath"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/core/s3ops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

func InitClient() {
	s3ops.InitS3Config(config.Options.IDrive.Region, config.Options.IDrive.EndPoint,
		config.Options.IDrive.Bucket, config.Options.IDrive.AccessKey,
		config.Options.IDrive.SecretKey)
}

// GetSermon gets the sermon by year and filename
// Return it as bytes so the caller can simply push the contents to the user
func GetSermon(year, fName string) (byts []byte, err error) {
	key := filepath.Join(year, fName) // key is essentially the file spec from the cloud storage perspective
	localFileSpec := filepath.Join(config.Options.IDrive.LocalSermonsDir, key)

	// If the file does not exist locally, then get the file from IDriveE2
	// and cache it to the local sermons directory
	if _, err = os.Stat(localFileSpec); err != nil {
		// Then get the file from Idrive, caching it to the sermons folder
		// (TODO - we will have a LRU cleanup process - track usages in some structure (redis?))
		err = s3ops.GetFileFromS3(key, filepath.Join(config.Options.IDrive.LocalSermonsDir, key))
		if err != nil {
			return byts, serr.Wrap(err, "error obtaining file from IDriveE2")
		}
	} else {
		logger.Info("Serving cached sermon", "sermon", key)
	}

	byts, err = os.ReadFile(localFileSpec)
	if err != nil {
		return byts, serr.Wrap(err, "could not read cached sermon file from server")
	}
	return
}

/* May use later */
// if keys, err := ListKeys("2008"); err != nil {
// 	log.Println(serr.StringFromErr(serr.Wrap(err)))
// 	return
// } else {
// 	fmt.Println(strings.Join(keys, ", "))
// }
//
// err := GetFileFromS3("2008/0928-Heb1.mp3", "/Users/ro/xfr/2008_0928-Heb1.mp3")
// if err != nil {
// 	log.Println(serr.StringFromErr(serr.Wrap(err)))
// 	return
// }
//
// err = PutFileToS3("2007", "/Users/ro/xfr/2008_0928-Heb1.mp3")
// if err != nil {
// 	log.Println(serr.StringFromErr(serr.Wrap(err)))
// 	return
// }
//
// err = RenameFileInS3("2007", "2008_0928-Heb1.mp3", "Renamed_2008_0928-Heb1.mp3")
// if err != nil {
// 	log.Println(serr.StringFromErr(serr.Wrap(err)))
// 	return
// }
