package idrive

// image_cache.go plugs editor-uploaded article images into the media cache
// eviction in sermon_cache.go.
//
// resource/chimage keeps IDrive e2 as the durable copy of each image (key
// images/<name>) and dist/img/ as a local cache it fills on demand. Without
// eviction that cache only grows: on Kubernetes it lives in the container's
// writable layer, which counts against the node's ephemeral storage until the
// pod is replaced. Tracking image access here lets the same hourly sweep that
// trims sermons reclaim images nobody has requested within the idle window.
//
//	chimage store (upload reached e2) ─┐
//	chimage serve (fetched from e2)  ──┼─► TrackImageAccess ─► image_cache_access upsert
//	chimage serve (local hit)        ──┘
//
//	StartCacheCleanup tick ─► imageCache.runCleanupPass
//	                            idle row ─► HEAD images/<name> on e2
//	                                          ├─ present ─► rm dist/img/<name>, drop row
//	                                          └─ missing/error ─► keep the file
//
// Evicting an image is cheap to undo: its name is content-addressed and its
// response is cached by browsers as immutable, so a later request is one e2
// fetch, after which the image is cached and tracked again.

var imageCache = cacheIndex{table: "image_cache_access", label: "image"}

// TrackImageAccess records (or refreshes) the access time of a locally-cached
// image. relFileSpec is the e2 key (images/<name>), which eviction HEADs before
// deleting; localFileSpec is the cached file.
//
// Best-effort like TrackSermonAccess: errors are logged, never returned, and
// callers run it in a goroutine so tracking never delays serving the image.
func TrackImageAccess(relFileSpec, localFileSpec string) {
	imageCache.trackAccess(relFileSpec, localFileSpec)
}
