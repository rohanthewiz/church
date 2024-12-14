package s3ops

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rohanthewiz/church/util/fileops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

var s3Client *s3.Client

type s3Config struct {
	Region string
	URL    string
	Bucket string
	Key    string
	Secret string
}

var s3Cfg *s3Config

func InitS3Config(region, url, bucket, key, secret string) {
	s3Cfg = &s3Config{Region: region, URL: url, Bucket: bucket, Key: key, Secret: secret}
}

func initS3Client() (err error) {
	if s3Client != nil {
		return nil
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Info("Recovered from panic in initS3Client - are all the configs in place?", "location",
				serr.FunctionLoc(serr.FrameLevels.FrameLevel2), "panicMsg", fmt.Sprintf("%v", r))
			// panic(r)
		}
	}()

	// Init the S3 client
	conf, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			s3Cfg.Key, s3Cfg.Secret, "")),
		config.WithRegion(s3Cfg.Region),
		config.WithBaseEndpoint(s3Cfg.URL),
	)
	if err != nil {
		fmt.Println("Error loading S3 config, ", err)
		return serr.Wrap(err)
	}
	// Populate our package level variable
	s3Client = s3.NewFromConfig(conf)
	return
}

// ListKeys gets the list of items
func ListKeys(bucketPrefix string) (keys []string, err error) {
	if s3Client == nil {
		_ = initS3Client()
	}
	if s3Client == nil {
		return keys, serr.New("Could not initialize S3 client")
	}

	resp, err := s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(s3Cfg.Bucket),
		Prefix: aws.String(bucketPrefix)})
	if err != nil {
		return keys, serr.Wrap(err)
	}

	for _, item := range resp.Contents {
		keys = append(keys, *item.Key)
	}

	// sort.SliceStable(items, func(i, j int) bool {
	// 	return (*items[i].LastModified).After(*items[j].LastModified)
	// })
	return
}

// RenameFileInS3 copies the object to a new key and deletes the original
func RenameFileInS3(bucketPrefix, srcFileName, destFileName string) (err error) {
	if s3Client == nil {
		_ = initS3Client()
	}
	if s3Client == nil {
		return serr.New("Could not initialize S3 client")
	}

	srcKey := filepath.Join(bucketPrefix, srcFileName)
	destKey := filepath.Join(bucketPrefix, destFileName)

	// Copy the object to a new key
	_, err = s3Client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     aws.String(s3Cfg.Bucket),
		CopySource: aws.String(s3Cfg.Bucket + "/" + srcKey),
		Key:        aws.String(destKey),
	})
	if err != nil {
		return serr.Wrap(err)
	}

	// Delete the old object
	_, err = s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s3Cfg.Bucket),
		Key:    aws.String(srcKey),
	})
	if err != nil {
		return serr.Wrap(err)
	}
	return
}

func PutFileToS3(bucketPrefix string, filePath string) (err error) {
	if s3Client == nil {
		_ = initS3Client()
	}
	if s3Client == nil {
		return serr.New("Could not initialize S3 client")
	}

	log.Println("Uploading file: " + filePath)
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return serr.New(fmt.Sprintf("Unable to read file %q, %v", filePath, err))
	}

	filename := filepath.Base(filePath)
	key := filepath.Join(bucketPrefix, filename)

	// log.Println("Writing file: " + key)
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s3Cfg.Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(fileContent),
	})
	if err != nil {
		return serr.New(fmt.Sprintf("Failed to upload data to %s. %s\n",
			filepath.Join(s3Cfg.Bucket, bucketPrefix, filename), err.Error()))
	}
	log.Printf("Successfully uploaded file %q to S3 Bucket %q, key: %q\n",
		filePath, s3Cfg.Bucket, filepath.Join(bucketPrefix, filename))
	return
}

func GetFileFromS3(key, targetFileSpec string) error {
	if s3Client == nil {
		_ = initS3Client()
	}
	if s3Client == nil {
		return serr.New("Could not initialize S3 client")
	}

	output, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return serr.Wrap(err)
	}

	sermonYrDir, filename := filepath.Split(targetFileSpec)

	// Setup dir
	isDir, _ := fileops.IsDir(sermonYrDir)
	if !isDir {
		err = os.Mkdir(sermonYrDir, 0750)
		if err != nil && !os.IsExist(err) {
			return serr.Wrap(err, "Error making the directory for sermon year")
		}
		logger.Info("Successfully created directory for sermon year", "year", sermonYrDir)
	}
	origDir, err := os.Getwd()
	if err != nil {
		logger.Warn("Could not obtain the curr working directory")
	}
	err = os.Chdir(sermonYrDir)
	if err != nil {
		return serr.Wrap(err, "Could not change to the sermon directory")
	}
	defer func() {
		_ = os.Chdir(origDir)
	}()

	// Create the sermon file locally
	outFile, err := os.Create(filename)
	if err != nil {
		return serr.Wrap(err, "Could not create the sermon file locally")
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, output.Body)
	if err != nil {
		return serr.Wrap(err)
	}

	log.Printf("Successfully downloaded file %q to local path %q\n", key, targetFileSpec)
	return nil
}
