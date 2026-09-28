//go:build !cgo

package chimage

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

// Run with: CGO_ENABLED=0 go test ./resource/chimage/
// (with cgo on, the bimg implementation is compiled instead and this file is skipped)

// solid builds a w×h image with a gradient so encoders can't collapse it to
// a trivially tiny payload.
func solid(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	return img
}

func encode(t *testing.T, format string, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	var err error
	switch format {
	case "jpeg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
	case "png":
		err = png.Encode(&buf, img)
	case "gif":
		err = gif.Encode(&buf, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestResizeToHeight_DownscalesKeepingFormatAndAspect(t *testing.T) {
	for _, format := range []string{"jpeg", "png"} {
		t.Run(format, func(t *testing.T) {
			in := encode(t, format, solid(800, 600))
			out, err := resizeToHeight(in, 150)
			if err != nil {
				t.Fatal(err)
			}
			cfg, gotFormat, err := image.DecodeConfig(bytes.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			if gotFormat != format {
				t.Errorf("format = %q, want %q (caller keeps the original MIME type)", gotFormat, format)
			}
			if cfg.Width != 200 || cfg.Height != 150 {
				t.Errorf("size = %dx%d, want 200x150", cfg.Width, cfg.Height)
			}
			if len(out) >= len(in) {
				t.Errorf("resized %d bytes, not smaller than original %d", len(out), len(in))
			}
		})
	}
}

// Every case here must come back byte-identical with no error, so the caller
// falls through to storing the original rather than dropping the image.
func TestResizeToHeight_PassThrough(t *testing.T) {
	cases := map[string]struct {
		data   []byte
		height int
	}{
		"already short enough":  {encode(t, "jpeg", solid(100, 80)), 400},
		"gif (may be animated)": {encode(t, "gif", solid(300, 300)), 100},
		"unrecognised bytes":    {[]byte("not an image at all"), 100},
		"non-positive height":   {encode(t, "png", solid(300, 300)), 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := resizeToHeight(c.data, c.height)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if !bytes.Equal(out, c.data) {
				t.Error("data changed, want it returned unchanged")
			}
		})
	}
}
