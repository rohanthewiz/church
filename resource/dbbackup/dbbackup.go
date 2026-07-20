// Package dbbackup implements consistent database snapshots to S3-compatible
// object storage — the interim replication story for bytdb sites until WAL
// shipping (bytdb/replicate) exists: RPO equals the trigger cadence (the k8s
// CronJob runs hourly).
//
// Key layout in the bucket, per site (config Backup.Prefix):
//
//	<prefix>/<UTC timestamp>/church.db   immutable history, pruned to Retain
//	<prefix>/latest/church.db            rolling pointer; what the Deployment's
//	                                     restore-if-empty initContainer pulls
//
// The snapshot is uploaded once to its timestamped key, then server-side
// copied over latest/ — S3 PUT and CopyObject are atomic per key, so latest/
// is always a complete database, never a partial write.
//
// This package owns its own S3 client rather than reusing core/s3ops: that
// client is bound to the IDrive media bucket, and backups deliberately use
// separate credentials (media creds must not read database contents).
package dbbackup

import (
	"bytes"
	"context"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const (
	dbFileName = "church.db"
	latestDir  = "latest"
	// tsFormat sorts lexicographically == chronologically, which is what
	// lets pruning sort keys as plain strings. Always UTC — sites in
	// different timezones share buckets, and DST would reorder local times.
	tsFormat      = "20060102-150405Z"
	defaultRetain = 72 // three days of hourly snapshots
)

// Result is what a successful run reports back through the API — enough for
// the CronJob log line to be a meaningful audit record on its own.
type Result struct {
	Key       string `json:"key"`        // timestamped object written
	LatestKey string `json:"latest_key"` // rolling pointer updated
	Bytes     int64  `json:"bytes"`      // snapshot size
	Pruned    int    `json:"pruned"`     // old snapshots deleted this run
	DurMillis int64  `json:"dur_millis"`
}

// Configured reports whether the destination is fully specified. The token
// is checked separately by the API layer (it gates the endpoint, not the
// storage destination).
func Configured() bool {
	if config.Options == nil { // config not loaded (early boot, tests)
		return false
	}
	b := config.Options.Backup
	return b.Endpoint != "" && b.Bucket != "" && b.AccessKey != "" && b.SecretKey != ""
}

// client is built per run rather than cached: backups run hourly, so setup
// cost is irrelevant, and a fresh client picks up rotated credentials
// without a pod restart.
func client() (*s3.Client, error) {
	b := config.Options.Backup
	endpoint := b.Endpoint
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	region := b.Region
	if region == "" {
		region = "us-east-1" // S3-compatibles generally accept any non-empty region
	}
	conf, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			b.AccessKey, b.SecretKey, "")),
		awsconfig.WithRegion(region),
		awsconfig.WithBaseEndpoint(endpoint),
	)
	if err != nil {
		return nil, serr.Wrap(err, "error building S3 config for backup")
	}
	return s3.NewFromConfig(conf), nil
}

// Run takes one snapshot: stream from the engine, upload, roll latest/,
// prune history. Returns a Result for the API response.
func Run() (res Result, err error) {
	started := time.Now()
	b := config.Options.Backup

	// The whole database rides through memory — church DBs are MBs, and an
	// in-memory buffer keeps the pod filesystem out of the picture (the only
	// writable volume is the live DB's own). If a site's DB ever grows to
	// where this matters, switch to a spooled temp file + multipart upload.
	var buf bytes.Buffer
	n, err := db.BytDBBackupTo(&buf)
	if err != nil {
		return res, serr.Wrap(err, "error snapshotting database")
	}

	cl, err := client()
	if err != nil {
		return res, err
	}

	// path (not filepath): S3 keys always use forward slashes.
	res.Key = path.Join(b.Prefix, time.Now().UTC().Format(tsFormat), dbFileName)
	res.LatestKey = path.Join(b.Prefix, latestDir, dbFileName)
	res.Bytes = n

	_, err = cl.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(b.Bucket),
		Key:    aws.String(res.Key),
		Body:   bytes.NewReader(buf.Bytes()),
	})
	if err != nil {
		return res, serr.Wrap(err, "error uploading backup", "key", res.Key)
	}

	// Server-side copy — no second upload of the payload. CopySource is
	// "bucket/key", URL-escaped per the S3 API contract.
	_, err = cl.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     aws.String(b.Bucket),
		CopySource: aws.String(url.PathEscape(b.Bucket + "/" + res.Key)),
		Key:        aws.String(res.LatestKey),
	})
	if err != nil {
		return res, serr.Wrap(err, "error updating latest backup pointer", "key", res.LatestKey)
	}

	// Prune failures are logged, not returned: the snapshot itself succeeded,
	// and failing the run would make the CronJob retry a full backup just to
	// re-attempt deletes. Over-retention is the safe failure direction.
	pruned, pruneErr := prune(cl)
	if pruneErr != nil {
		logger.LogErr(pruneErr, "backup succeeded but pruning old snapshots failed")
	}
	res.Pruned = pruned
	res.DurMillis = time.Since(started).Milliseconds()
	return res, nil
}

// prune deletes timestamped snapshots beyond the retention count, oldest
// first. latest/ never qualifies (it doesn't parse as a timestamp).
func prune(cl *s3.Client) (deleted int, err error) {
	b := config.Options.Backup
	retain := b.Retain
	if retain <= 0 {
		retain = defaultRetain
	}

	listPrefix := b.Prefix
	if listPrefix != "" && !strings.HasSuffix(listPrefix, "/") {
		listPrefix += "/"
	}

	// Collect every timestamped snapshot key. Paginated: at hourly cadence
	// the listing exceeds one page (1000 keys) only after a long prune
	// outage, which is exactly when correctness matters most.
	var tsKeys []string
	var contToken *string
	for {
		out, listErr := cl.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket:            aws.String(b.Bucket),
			Prefix:            aws.String(listPrefix),
			ContinuationToken: contToken,
		})
		if listErr != nil {
			return 0, serr.Wrap(listErr, "error listing backups for pruning")
		}
		for _, obj := range out.Contents {
			key := aws.ToString(obj.Key)
			// Expect <prefix>/<timestamp>/church.db; skip anything else
			// (latest/, foreign objects sharing the prefix).
			rel := strings.TrimPrefix(key, listPrefix)
			parts := strings.Split(rel, "/")
			if len(parts) != 2 || parts[1] != dbFileName {
				continue
			}
			if _, tErr := time.Parse(tsFormat, parts[0]); tErr != nil {
				continue
			}
			tsKeys = append(tsKeys, key)
		}
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		contToken = out.NextContinuationToken
	}

	if len(tsKeys) <= retain {
		return 0, nil
	}
	sort.Strings(tsKeys) // timestamp format sorts chronologically
	for _, key := range tsKeys[:len(tsKeys)-retain] {
		if _, delErr := cl.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: aws.String(b.Bucket),
			Key:    aws.String(key),
		}); delErr != nil {
			// Report partial progress; the next run retries the remainder.
			return deleted, serr.Wrap(delErr, "error deleting old backup", "key", key)
		}
		deleted++
	}
	return deleted, nil
}
