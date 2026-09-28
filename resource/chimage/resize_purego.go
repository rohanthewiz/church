//go:build !cgo

package chimage

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/rohanthewiz/serr"
	"golang.org/x/image/draw"
)

// jpegQuality is used when re-encoding a resized JPEG. 80 is visually close
// to the source for photos at article-inline sizes while still giving a large
// byte saving, which is the whole point of the resize.
const jpegQuality = 80

// resizeToHeight is the cgo-free counterpart of the bimg version in
// resize_vips.go. It exists so the site can be cross-compiled
// (CGO_ENABLED=0 GOOS=linux GOARCH=amd64) into a static binary that needs
// neither libvips nor a C toolchain on the server.
//
// Contract, matching how ProcessInlineImages uses the result:
//   - The output keeps the input's format, because the caller keeps the data
//     URL's original MIME type and file extension.
//   - A format this path cannot re-encode (GIF, WebP, anything unregistered)
//     is returned unchanged with a nil error. Returning an error would make the
//     caller skip the image entirely (leaving a large image inline in the
//     article HTML), whereas unchanged bytes simply fail the caller's
//     "is it smaller?" check and the original is stored as a file, as before.
//     GIF is deliberately passed through: decoding keeps only the first frame,
//     so re-encoding would silently kill animations.
//   - Images already at or below the target height are not upscaled; enlarging
//     can only grow the payload, which the caller would discard anyway.
//
// Differences from libvips worth knowing: EXIF orientation is not applied
// (JPEG decoding here ignores EXIF, and re-encoding drops it), and the
// resampling is slower. Both are acceptable for occasional editor uploads.
func resizeToHeight(data []byte, height int) ([]byte, error) {
	if height <= 0 {
		return data, nil
	}

	// DecodeConfig reads only the header, so unsupported formats and images
	// that need no resize are rejected before paying for a full decode.
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") {
		return data, nil
	}
	if cfg.Height <= height || cfg.Width <= 0 {
		return data, nil
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, serr.Wrap(err, "could not decode image for resizing", "format", format)
	}

	// Preserve aspect ratio: width scales by the same factor as height.
	// Rounded, and never allowed to collapse to zero for very tall, thin images.
	b := src.Bounds()
	width := (b.Dx()*height + b.Dy()/2) / b.Dy()
	if width < 1 {
		width = 1
	}
	dstRect := image.Rect(0, 0, width, height)

	// CatmullRom is the highest-quality kernel in x/image/draw and is the
	// right trade for downscaling photos; its extra CPU cost is irrelevant at
	// the rate editors upload images.
	var out bytes.Buffer
	switch format {
	case "jpeg":
		// JPEG has no alpha, so plain RGBA avoids an unpremultiply step.
		dst := image.NewRGBA(dstRect)
		draw.CatmullRom.Scale(dst, dstRect, src, b, draw.Src, nil)
		err = jpeg.Encode(&out, dst, &jpeg.Options{Quality: jpegQuality})
	case "png":
		// NRGBA keeps transparent edges from darkening after resampling.
		dst := image.NewNRGBA(dstRect)
		draw.CatmullRom.Scale(dst, dstRect, src, b, draw.Src, nil)
		err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&out, dst)
	}
	if err != nil {
		return nil, serr.Wrap(err, "could not encode resized image", "format", format)
	}
	return out.Bytes(), nil
}
