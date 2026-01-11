// idrive_uploader scans the current directory for .mp3 and .3gp files
// and uploads them to IDrive E2 under a specified year path if they don't already exist.
//
// Usage: idrive_uploader <year>
//
// Environment variables required:
//   - IDRIVE_ENDPOINT: S3-compatible endpoint URL (e.g., https://xxxxxx.e2.idrivee2-XX.com)
//   - IDRIVE_REGION: Region (e.g., us-west-1)
//   - IDRIVE_BUCKET: Bucket name
//   - IDRIVE_ACCESS_KEY: Access key
//   - IDRIVE_SECRET_KEY: Secret key
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type idriveConfig struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

type uploadResult struct {
	Filename string
	Error    error
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: idrive_uploader <year>")
		fmt.Println("Example: idrive_uploader 2024")
		os.Exit(1)
	}

	year := os.Args[1]

	// Load configuration from environment variables
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize S3 client
	client, err := initS3Client(cfg)
	if err != nil {
		fmt.Printf("Failed to initialize S3 client: %v\n", err)
		os.Exit(1)
	}

	// Scan current directory for audio files
	localFiles, err := scanAudioFiles(".")
	if err != nil {
		fmt.Printf("Failed to scan directory: %v\n", err)
		os.Exit(1)
	}

	if len(localFiles) == 0 {
		fmt.Println("No .mp3 or .3gp files found in current directory.")
		os.Exit(0)
	}

	// Get list of existing files in IDrive under the year path
	existingFiles, err := listIDriveFiles(client, cfg.Bucket, year)
	if err != nil {
		fmt.Printf("Failed to list files in IDrive: %v\n", err)
		os.Exit(1)
	}

	// Build a set of existing filenames for quick lookup
	existingSet := make(map[string]bool)
	for _, key := range existingFiles {
		// Extract just the filename from the key (remove year prefix)
		filename := filepath.Base(key)
		existingSet[filename] = true
	}

	// Process files
	var alreadyPresent []string
	var successfulUploads []string
	var failedUploads []uploadResult

	for _, localFile := range localFiles {
		filename := filepath.Base(localFile)

		if existingSet[filename] {
			alreadyPresent = append(alreadyPresent, filename)
			continue
		}

		// Upload the file
		err := uploadFile(client, cfg.Bucket, year, localFile)
		if err != nil {
			failedUploads = append(failedUploads, uploadResult{
				Filename: filename,
				Error:    err,
			})
		} else {
			successfulUploads = append(successfulUploads, filename)
		}
	}

	// Print summary
	printSummary(alreadyPresent, successfulUploads, failedUploads)
}

func loadConfig() (*idriveConfig, error) {
	cfg := &idriveConfig{
		Endpoint:  os.Getenv("IDRIVE_ENDPOINT"),
		Region:    os.Getenv("IDRIVE_REGION"),
		Bucket:    os.Getenv("IDRIVE_BUCKET"),
		AccessKey: os.Getenv("IDRIVE_ACCESS_KEY"),
		SecretKey: os.Getenv("IDRIVE_SECRET_KEY"),
	}

	var missing []string
	if cfg.Endpoint == "" {
		missing = append(missing, "IDRIVE_ENDPOINT")
	}
	if cfg.Region == "" {
		missing = append(missing, "IDRIVE_REGION")
	}
	if cfg.Bucket == "" {
		missing = append(missing, "IDRIVE_BUCKET")
	}
	if cfg.AccessKey == "" {
		missing = append(missing, "IDRIVE_ACCESS_KEY")
	}
	if cfg.SecretKey == "" {
		missing = append(missing, "IDRIVE_SECRET_KEY")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func initS3Client(cfg *idriveConfig) (*s3.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "")),
		config.WithRegion(cfg.Region),
		config.WithBaseEndpoint(cfg.Endpoint),
	)
	if err != nil {
		return nil, fmt.Errorf("error loading S3 config: %w", err)
	}

	return s3.NewFromConfig(awsCfg), nil
}

func scanAudioFiles(dir string) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".3gp") {
			files = append(files, filepath.Join(dir, name))
		}
	}

	return files, nil
}

func listIDriveFiles(client *s3.Client, bucket, year string) ([]string, error) {
	var keys []string
	prefix := year + "/"

	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("error listing objects: %w", err)
		}

		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}

	return keys, nil
}

func uploadFile(client *s3.Client, bucket, year, localPath string) error {
	fileContent, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("unable to read file: %w", err)
	}

	filename := filepath.Base(localPath)
	key := year + "/" + filename

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(fileContent),
	})
	if err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}

	return nil
}

func printSummary(alreadyPresent, successfulUploads []string, failedUploads []uploadResult) {
	// Summary line
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Already in IDrive: %d | Successfully uploaded: %d | Errors: %d\n\n",
		len(alreadyPresent), len(successfulUploads), len(failedUploads))

	// Successful uploads section
	fmt.Println("=== Successful Uploads ===")
	if len(successfulUploads) > 0 {
		fmt.Println(strings.Join(successfulUploads, ", "))
	} else {
		fmt.Println("(none)")
	}
	fmt.Println()

	// Failed uploads section
	fmt.Println("=== Failed Uploads ===")
	if len(failedUploads) > 0 {
		// Find max filename length for table formatting
		maxLen := 8 // minimum "Filename" header width
		for _, f := range failedUploads {
			if len(f.Filename) > maxLen {
				maxLen = len(f.Filename)
			}
		}

		// Print table header
		fmt.Printf("%-*s | Error\n", maxLen, "Filename")
		fmt.Printf("%s-+-%s\n", strings.Repeat("-", maxLen), strings.Repeat("-", 50))

		// Print rows
		for _, f := range failedUploads {
			fmt.Printf("%-*s | %v\n", maxLen, f.Filename, f.Error)
		}
	} else {
		fmt.Println("(none)")
	}
}
