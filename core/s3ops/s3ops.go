package s3ops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
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

func PutFileToS3(bucketPrefix string, fileSpec string) (err error) {
	if s3Client == nil {
		_ = initS3Client()
	}
	if s3Client == nil {
		return serr.New("Could not initialize S3 client")
	}

	log.Println("Uploading file: " + fileSpec)
	fileContent, err := os.ReadFile(fileSpec)
	if err != nil {
		return serr.New(fmt.Sprintf("Unable to read file %q, %v", fileSpec, err))
	}

	filename := filepath.Base(fileSpec)
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
		fileSpec, s3Cfg.Bucket, filepath.Join(bucketPrefix, filename))
	return
}

// BucketName returns the configured bucket name (mainly for display/diagnostics,
// e.g. showing the full IDrive e2 path of an object). Empty if not yet configured.
func BucketName() string {
	if s3Cfg == nil {
		return ""
	}
	return s3Cfg.Bucket
}

// ObjectInfo returns whether an object with the given key is present in the
// configured bucket on IDrive e2 and, when present, its size in bytes. The key is
// matched exactly and case-sensitively (S3 object keys are case-sensitive), so a
// local file whose name differs only in case will register as absent.
//
// The return contract is deliberately conservative for callers that gate
// destructive actions on it:
//   - (true, size, nil)  the object is present (size may be 0 for an empty object).
//   - (false, 0, nil)    a definitive "not found" (404 / NoSuchKey / NotFound).
//   - (false, 0, err)    existence could NOT be determined (network, auth, etc.);
//     callers must treat this as "unknown" and must NOT delete any local copy.
func ObjectInfo(key string) (exists bool, size int64, err error) {
	if s3Client == nil {
		_ = initS3Client()
	}
	if s3Client == nil {
		return false, 0, serr.New("Could not initialize S3 client")
	}

	out, err := s3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(s3Cfg.Bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, aws.ToInt64(out.ContentLength), nil
	}

	// Distinguish a genuine "object is absent" from a transient/operational error.
	// HeadObject surfaces a missing object as types.NotFound, but some
	// S3-compatible providers (IDrive e2 included) may instead return a generic
	// API error carrying a NotFound/NoSuchKey/404 code, so we check both.
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return false, 0, nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return false, 0, nil
		}
	}

	// Anything else is "unknown" — bubble it up so the caller stays safe.
	return false, 0, serr.Wrap(err, "Error checking object existence in S3", "key", key)
}

// ObjectExists reports whether an object with the given key is present in the
// configured bucket on IDrive e2. It is a thin wrapper over ObjectInfo for callers
// that do not care about size; the same conservative error contract applies
// (a (false, err) result means "unknown" — keep the local copy).
func ObjectExists(key string) (exists bool, err error) {
	exists, _, err = ObjectInfo(key)
	return exists, err
}

func GetFileFromS3(key string) (fileBytes []byte, err error) {
	if s3Client == nil {
		_ = initS3Client()
	}
	if s3Client == nil {
		return fileBytes, serr.New("Could not initialize S3 client")
	}

	output, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fileBytes, serr.Wrap(err, "Error obtaining file via s3 client")
	}
	defer output.Body.Close()

	fileBytes, err = io.ReadAll(output.Body)
	if err != nil {
		return fileBytes, serr.Wrap(err, "Error reading file content from s3 object")
	}

	return fileBytes, nil
}
