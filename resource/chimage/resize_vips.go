//go:build cgo

package chimage

import "github.com/h2non/bimg"

// resizeToHeight scales an encoded image to the given height, keeping its
// aspect ratio and its original format, using libvips through bimg.
//
// This is the preferred implementation (faster, lower memory, EXIF
// auto-rotation) and is chosen automatically whenever cgo is on, which is the
// default for a native build and what deploy/docker/Dockerfile does. A build
// with CGO_ENABLED=0 (e.g. cross-compiling from macOS to linux/amd64) gets
// resize_purego.go instead, because bimg does not compile without cgo.
func resizeToHeight(data []byte, height int) ([]byte, error) {
	return bimg.Resize(data, bimg.Options{Height: height})
}
