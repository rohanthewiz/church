// images_to_e2 copies a site's existing editor-uploaded article images
// (dist/img/) to IDrive e2, under the images/ prefix that resource/chimage
// now writes new uploads to and serves from when a local copy is missing.
//
// Run it from the SITE directory, the one holding cfg/options.yml and
// dist/img/, with APP_ENV selecting the options section whose idrive block
// names the bucket:
//
//	cd cema
//	APP_ENV=production go run github.com/rohanthewiz/church/test_scripts/images_to_e2          # dry run
//	APP_ENV=production go run github.com/rohanthewiz/church/test_scripts/images_to_e2 -apply   # upload
//
// Once a run reports nothing left to copy, the site no longer needs dist/img/
// to persist: a fresh container fetches each image from e2 on first request.
// That is what lets the Kubernetes manifests drop the uploads volume mount.
//
// Design choices:
//   - Dry run by default. The tool writes to a shared bucket, so it says what
//     it would do unless -apply is given.
//   - Objects already present are skipped, never overwritten. Names are
//     content-addressed (<name>.<xxhash>.<ext>), so a present key already
//     holds the same bytes, and a re-run after a partial failure only copies
//     what is missing.
//   - An existence check that errors (network, credentials) counts as a
//     failure rather than "absent": uploading blind could mask a
//     misconfigured bucket as a successful run.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/core/idrive"
	"github.com/rohanthewiz/church/core/s3ops"
	"github.com/rohanthewiz/church/resource/chimage"
)

func main() {
	apply := flag.Bool("apply", false, "upload missing images (default is a dry run)")
	dir := flag.String("dir", chimage.LocalDir, "local image directory")
	flag.Parse()

	config.InitConfig("", "", "")
	if !config.Options.IDrive.Enabled {
		fmt.Println("idrive.enabled is false in cfg/options.yml for APP_ENV=" + config.AppEnv + "; nothing to do")
		os.Exit(1)
	}
	idrive.InitClient()
	fmt.Printf("bucket %q, prefix %q, source %s, %s\n", s3ops.BucketName(), chimage.ObjectPrefix, *dir,
		map[bool]string{true: "APPLY", false: "dry run"}[*apply])

	entries, err := os.ReadDir(*dir)
	if err != nil {
		fmt.Println("FATAL reading", *dir+":", err)
		os.Exit(1)
	}

	var present, copied, wouldCopy, failed int
	for _, e := range entries {
		name := e.Name()
		// Hidden files include .keep and any temp file from an interrupted write
		if e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		key := chimage.ObjectKey(name)
		exists, err := s3ops.ObjectExists(key)
		if err != nil {
			fmt.Printf("FAIL  %s: existence check: %v\n", key, err)
			failed++
			continue
		}
		if exists {
			present++
			continue
		}
		if !*apply {
			fmt.Printf("would copy  %s\n", key)
			wouldCopy++
			continue
		}
		if err := s3ops.PutFileToS3(chimage.ObjectPrefix, filepath.Join(*dir, name)); err != nil {
			fmt.Printf("FAIL  %s: upload: %v\n", key, err)
			failed++
			continue
		}
		fmt.Printf("copied  %s\n", key)
		copied++
	}

	fmt.Printf("\nalready on e2: %d, copied: %d, would copy: %d, failed: %d\n", present, copied, wouldCopy, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
