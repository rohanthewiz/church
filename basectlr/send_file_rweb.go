package basectlr

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/core/html"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

func SendFileRWeb(ctx rweb.Context, filename string, body []byte) error {
	return rweb.File(ctx, filename, body)
}

// parseRange parses HTTP Range header (e.g., "bytes=0-1023")
// Returns start, end (inclusive), and error
func parseRange(rangeHeader string, fileSize int64) (start, end int64, err error) {
	if rangeHeader == "" {
		return 0, fileSize - 1, nil
	}

	// Remove "bytes=" prefix
	rangeHeader = strings.TrimPrefix(rangeHeader, "bytes=")

	// Handle "bytes=start-" or "bytes=start-end"
	parts := strings.Split(rangeHeader, "-")
	if len(parts) != 2 {
		return 0, 0, serr.New("invalid range format")
	}

	// Parse start position
	if parts[0] != "" {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 || start >= fileSize {
			return 0, 0, serr.New("invalid start range")
		}
	}

	// Parse end position
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start || end >= fileSize {
			return 0, 0, serr.New("invalid end range")
		}
	} else {
		// If no end specified, serve to end of file
		end = fileSize - 1
	}

	return start, end, nil
}

// SendAudioFileRWeb streams audio files with support for:
// - Content-Type detection based on file extension and user agent
// - HTTP Range requests for seek capability (206 Partial Content)
// - CORS headers for cross-origin access
// - Proper caching headers
func SendAudioFileRWeb(ctx rweb.Context, filename string, body []byte) error {
	fileSize := int64(len(body))

	// Get user agent for platform-specific MIME type optimization
	userAgent := ctx.UserAgent()

	// Set Content-Type based on file extension and user agent (use the first/preferred type)
	contentTypes := html.GetAudioContentType(filename, userAgent)
	ctx.Response().SetHeader("Content-Type", contentTypes[0]) // above will always return a non-empty array

	// Set basic headers
	ctx.Response().SetHeader("Content-Disposition", "inline; filename="+url.QueryEscape(filename))
	ctx.Response().SetHeader("x-filename", url.QueryEscape(filename))
	ctx.Response().SetHeader("Content-Description", "File Transfer")

	// Enable range requests for seeking
	ctx.Response().SetHeader("Accept-Ranges", "bytes")

	// Cache headers - long-lived immutable content
	ctx.Response().SetHeader("Cache-Control", "public, max-age=31536000, immutable")

	// CORS headers - allow cross-origin access
	ctx.Response().SetHeader("Access-Control-Allow-Origin", "*")
	ctx.Response().SetHeader("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	ctx.Response().SetHeader("Access-Control-Allow-Headers", "Range")
	ctx.Response().SetHeader("Access-Control-Expose-Headers", "Content-Length, Content-Range, x-filename")

	// Handle OPTIONS preflight request
	if ctx.Request().Method() == "OPTIONS" {
		return ctx.WriteString("") // Status defaults to 200
	}

	// Check for Range header
	rangeHeader := ctx.Request().Header("Range")

	// If no range requested, send full file
	if rangeHeader == "" {
		ctx.Response().SetHeader("Content-Length", strconv.FormatInt(fileSize, 10))
		return ctx.Status(http.StatusOK).Bytes(body)
	}

	// Parse range request
	start, end, err := parseRange(rangeHeader, fileSize)
	if err != nil {
		// Invalid range - send 416 Range Not Satisfiable
		ctx.Response().SetHeader("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		return ctx.Status(http.StatusRequestedRangeNotSatisfiable).WriteString("Invalid range")
	}

	// Calculate content length for this range
	contentLength := end - start + 1

	// Set 206 Partial Content headers
	ctx.Response().SetHeader("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	ctx.Response().SetHeader("Content-Length", strconv.FormatInt(contentLength, 10))

	// Send partial content
	return ctx.Status(http.StatusPartialContent).Bytes(body[start : end+1])
}
