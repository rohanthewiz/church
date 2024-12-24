package idrive

import (
	"os"
	"path/filepath"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/core/s3ops"
	"github.com/rohanthewiz/church/resource/sermon"
	"github.com/rohanthewiz/church/util/fileops"
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
// (TODO - we will have a LRU cleanup process - track usages in some structure (redis?))
func GetSermon(year, fName string) (fileBytes []byte, err error) {
	relFileSpec, localFileSpec := sermon.GetRelAndLocalFileSpecs(year, fName)

	// If the file does not exist locally, then get the file from IDriveE2
	// and cache it to the local sermons directory
	if _, err = os.Stat(localFileSpec); err != nil {
		fileBytes, err = getSermonFromIDrive(relFileSpec, localFileSpec)
		if err != nil {
			return fileBytes, serr.Wrap(err, "error obtaining file from IDriveE2", "year", year, "sermon", fName)
		}
		return fileBytes, nil
	}

	logger.Info("Serving cached sermon", "sermon", relFileSpec)
	fileBytes, err = os.ReadFile(localFileSpec)
	if err != nil {
		return fileBytes, serr.Wrap(err, "could not read cached sermon file from server")
	}
	return
}

// getSermonFromIDrive uses S3 client
func getSermonFromIDrive(relFileSpec, localFileSpec string) (fileBytes []byte, err error) {
	fileBytes, err = s3ops.GetFileFromS3(relFileSpec)
	if err != nil {
		return fileBytes, serr.Wrap(err)
	}
	logger.Info("Successfully downloaded sermon %q\n", localFileSpec)

	go func() { // Cache sermon locally - TODO some LRU process
		sermonYrDir, filename := filepath.Split(relFileSpec)

		// Ensure sermon year dir
		isDir, _ := fileops.IsDir(sermonYrDir)
		if !isDir {
			err = os.Mkdir(sermonYrDir, 0750)
			if err != nil && !os.IsExist(err) {
				logger.LogErr(serr.Wrap(err, "Error creating directory for sermon year"))
				return
			}
			logger.Info("Successfully created directory for sermon year", "year", sermonYrDir)
		}

		err = os.WriteFile(filename, fileBytes, 0644)
		if err != nil {
			logger.LogErr(serr.Wrap(err, "Could not create the sermon file locally"))
			return
		}
	}()

	return fileBytes, nil
}

func PutSermonToIDrive(sermonYear, localFileSpec string) (err error) {
	return s3ops.PutFileToS3(sermonYear, localFileSpec)
}

/* May use later */
// if keys, err := ListKeys("2008"); err != nil {
// 	log.Println(serr.StringFromErr(serr.Wrap(err)))
// 	return
// } else {
// 	fmt.Println(strings.Join(keys, ", "))
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

// origDir, err := os.Getwd()
// if err != nil {
// 	logger.Warn("Could not obtain the curr working directory")
// }
// err = os.Chdir(sermonYrDir)
// if err != nil {
// 	return fileBytes, serr.Wrap(err, "Could not change to the sermon directory")
// }
// defer func() {
// 	_ = os.Chdir(origDir)
// }()
