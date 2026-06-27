package idrive

// sermon_cleanup_service.go backs the admin "Sermon Cleanup" tool. It answers two
// questions for locally-cached sermon files:
//
//  1. Which local copies are safe to delete because a good copy already lives on
//     IDrive e2? (ScanEligibleForDeletion)
//  2. Delete a selected batch — but re-verify each against IDrive e2 first, so a
//     stale browser selection can never cause us to delete the only copy.
//     (DeleteVerifiedLocalCopies)
//
// "Already on IDrive e2" means: an object exists at the exact key year/filename
// (case-sensitive, with the year being the file's parent directory) AND its size is
// non-zero. Because the IDrive object key is constructed as year/filename, a
// successful HeadObject on that key inherently confirms the case-exact name and the
// correct year; ObjectInfo additionally returns the size.
//
// This service lives in core/idrive (not resource/sermon) because core/idrive
// already depends on resource/sermon — importing the other way would form a cycle.

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/core/s3ops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// cloudCheckConcurrency bounds how many HeadObject calls run in parallel while
// building the eligibility listing. A church may have hundreds of cached files;
// doing the checks sequentially would make the admin page sluggish, while an
// unbounded fan-out could overwhelm the endpoint. 8 is a comfortable middle ground.
const cloudCheckConcurrency = 8

// LocalSermonInfo describes one locally-cached sermon file and the result of
// comparing it against IDrive e2. The admin UI consumes these directly.
type LocalSermonInfo struct {
	Year          string     // parent directory name, e.g. "2024"
	FileName      string     // base file name, e.g. "2024_0107-John1.mp3"
	RelFileSpec   string     // IDrive object key: "year/filename"
	LocalFileSpec string     // absolute/relative path of the local cached copy
	LocalSize     int64      // size of the local copy in bytes
	CloudExists   bool       // object present on IDrive e2 at RelFileSpec
	CloudSize     int64      // size of the IDrive object in bytes (when present)
	CloudPath     string     // human-friendly "bucket/year/filename" for display
	LastAccessed  *time.Time // from sermon_cache_access; nil if never tracked
}

// Eligible reports whether the local copy can be safely deleted: the cloud copy
// exists and is non-empty.
func (s LocalSermonInfo) Eligible() bool {
	return s.CloudExists && s.CloudSize > 0
}

// ScanEligibleForDeletion walks the local sermons directory and returns the files
// whose IDrive e2 copy is confirmed present and non-zero — i.e. those eligible for
// local deletion. Results are sorted by year (newest first) then file name so the
// admin UI can group them by year. Files that error out on the cloud check (network
// or auth problems) are conservatively omitted (not eligible) and logged.
func ScanEligibleForDeletion() (eligible []LocalSermonInfo, err error) {
	root := config.Options.IDrive.LocalSermonsDir
	if strings.TrimSpace(root) == "" {
		return nil, serr.New("LocalSermonsDir is not configured")
	}

	candidates, err := walkLocalSermons(root)
	if err != nil {
		return nil, serr.Wrap(err)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// Pull all last-accessed timestamps once rather than per file.
	lastAccessed, laErr := LastAccessedByRelSpec()
	if laErr != nil {
		// Non-fatal: the listing is still useful without the "last accessed" column.
		logger.LogErr(laErr, "sermon-cleanup: could not load last-accessed times; continuing without them")
		lastAccessed = map[string]time.Time{}
	}

	bucket := s3ops.BucketName()

	// Fan out the cloud existence/size checks across a bounded worker pool. Each
	// worker writes only to its own index in `candidates`, so no locking is needed
	// on the slice itself.
	var wg sync.WaitGroup
	sem := make(chan struct{}, cloudCheckConcurrency)
	for i := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			c := &candidates[idx]
			exists, size, cErr := s3ops.ObjectInfo(c.RelFileSpec)
			if cErr != nil {
				// "Unknown" — leave CloudExists false so it is treated as ineligible.
				logger.LogErr(cErr, "sermon-cleanup: cloud check failed; treating as ineligible",
					"relFileSpec", c.RelFileSpec)
				return
			}
			c.CloudExists = exists
			c.CloudSize = size
			if exists {
				c.CloudPath = strings.TrimSuffix(bucket, "/") + "/" + c.RelFileSpec
			}
		}(i)
	}
	wg.Wait()

	for i := range candidates {
		c := candidates[i]
		if at, ok := lastAccessed[c.RelFileSpec]; ok {
			t := at
			c.LastAccessed = &t
		}
		if c.Eligible() {
			eligible = append(eligible, c)
		}
	}

	// Sort: newest year first, then file name ascending.
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].Year != eligible[j].Year {
			return eligible[i].Year > eligible[j].Year
		}
		return eligible[i].FileName < eligible[j].FileName
	})

	return eligible, nil
}

// walkLocalSermons returns the cached sermon files laid out as <root>/<year>/<file>.
// Only regular files exactly one directory below the root are considered a sermon
// (the intervening directory is the year). Hidden files/dirs are skipped.
func walkLocalSermons(root string) (infos []LocalSermonInfo, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			// Skip hidden directories (e.g. .DS_Store containers) but keep walking root.
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") { // skip hidden files
			return nil
		}

		// Determine the path of this file relative to the root and require exactly
		// year/filename (one separator). Anything shallower or deeper is not a sermon.
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil // unexpected; just skip this entry
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		if len(parts) != 2 {
			return nil
		}
		year, fileName := parts[0], parts[1]

		var size int64
		if fi, statErr := d.Info(); statErr == nil {
			size = fi.Size()
		}

		infos = append(infos, LocalSermonInfo{
			Year:          year,
			FileName:      fileName,
			RelFileSpec:   year + "/" + fileName, // IDrive key uses forward slashes
			LocalFileSpec: path,
			LocalSize:     size,
		})
		return nil
	})
	if err != nil {
		return nil, serr.Wrap(err, "error walking local sermons directory", "root", root)
	}
	return infos, nil
}

// DeleteResult reports the outcome of a batch deletion request.
type DeleteResult struct {
	Deleted []string          // rel_file_specs whose local copies were deleted
	Skipped map[string]string // rel_file_spec -> reason it was not deleted
}

// DeleteVerifiedLocalCopies deletes the local copy of each requested sermon, but
// only after independently re-confirming that a non-zero copy exists on IDrive e2.
// The rel_file_specs come from a user-submitted form, so each is validated and
// re-checked here; we never trust the prior listing as authorization to delete.
//
// For each spec the steps are: validate shape -> confirm it resolves inside the
// local sermons dir (no path traversal) -> ObjectInfo must report exists && size>0
// -> os.Remove the local file -> drop its sermon_cache_access row.
func DeleteVerifiedLocalCopies(relFileSpecs []string) DeleteResult {
	res := DeleteResult{Skipped: map[string]string{}}
	root := config.Options.IDrive.LocalSermonsDir

	for _, raw := range relFileSpecs {
		spec := strings.TrimSpace(raw)
		if spec == "" {
			continue
		}

		localFileSpec, vErr := safeLocalFileSpec(root, spec)
		if vErr != nil {
			res.Skipped[spec] = vErr.Error()
			logger.LogErr(vErr, "sermon-cleanup: rejected unsafe spec", "spec", spec)
			continue
		}

		// Re-verify against IDrive e2 immediately before deleting.
		exists, size, cErr := s3ops.ObjectInfo(spec)
		if cErr != nil {
			res.Skipped[spec] = "could not verify IDrive copy; kept local file"
			logger.LogErr(cErr, "sermon-cleanup: verify failed; keeping local", "spec", spec)
			continue
		}
		if !exists {
			res.Skipped[spec] = "not found on IDrive e2; kept local file"
			continue
		}
		if size == 0 {
			res.Skipped[spec] = "IDrive copy is zero bytes; kept local file"
			continue
		}

		if rmErr := os.Remove(localFileSpec); rmErr != nil {
			if os.IsNotExist(rmErr) {
				// Local file already gone — still clear any tracking row, report deleted.
				_ = DeleteCacheRowByRelSpec(spec)
				res.Deleted = append(res.Deleted, spec)
				continue
			}
			res.Skipped[spec] = "failed to delete local file: " + rmErr.Error()
			logger.LogErr(serr.Wrap(rmErr, "sermon-cleanup: delete failed"), "localFileSpec", localFileSpec)
			continue
		}

		if rowErr := DeleteCacheRowByRelSpec(spec); rowErr != nil {
			// The file is gone; a leftover tracking row is harmless (the hourly sweep
			// tolerates a missing local file). Log but still count as deleted.
			logger.LogErr(rowErr, "sermon-cleanup: deleted file but failed to clear tracking row", "spec", spec)
		}
		res.Deleted = append(res.Deleted, spec)
		logger.Info("sermon-cleanup: deleted local copy", "relFileSpec", spec, "localFileSpec", localFileSpec)
	}

	return res
}

// safeLocalFileSpec validates a client-supplied rel_file_spec and resolves it to an
// absolute local path, guarding against path traversal. It enforces the
// year/filename shape and that the result stays within root.
func safeLocalFileSpec(root, spec string) (string, error) {
	if root == "" {
		return "", serr.New("LocalSermonsDir is not configured")
	}
	cleaned := filepath.ToSlash(filepath.Clean(spec))
	if strings.HasPrefix(cleaned, "/") || strings.Contains(cleaned, "..") {
		return "", serr.New("invalid sermon path", "spec", spec)
	}
	if len(strings.Split(cleaned, "/")) != 2 { // must be exactly year/filename
		return "", serr.New("sermon path must be year/filename", "spec", spec)
	}

	localFileSpec := filepath.Join(root, filepath.FromSlash(cleaned))

	// Defense in depth: confirm the joined path is genuinely under root.
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", serr.Wrap(err)
	}
	absFile, err := filepath.Abs(localFileSpec)
	if err != nil {
		return "", serr.Wrap(err)
	}
	if absFile != absRoot && !strings.HasPrefix(absFile, absRoot+string(os.PathSeparator)) {
		return "", serr.New("resolved path escapes sermons directory", "spec", spec)
	}
	return localFileSpec, nil
}
