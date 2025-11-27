package html

import (
	"path/filepath"
	"strings"
)

// audioContentTypeMapIOS maps file extensions to their possible MIME types for iOS devices.
// iOS has specific codec support and won't play 3gp files by default.
// Multiple types are provided for better browser compatibility, ordered by preference.
var audioContentTypeMapIOS = map[string][]string{
	".mp3":  {"audio/mpeg", "audio/mp3"},
	".3gp":  {"audio/3gpp"},  // iOS requires correct MIME type
	".3g2":  {"audio/3gpp2"}, // Same for 3g2
	".m4a":  {"audio/mp4", "audio/x-m4a"},
	".aac":  {"audio/aac", "audio/aacp"},
	".ogg":  {"audio/ogg", "audio/vorbis"},
	".opus": {"audio/opus", "audio/ogg"},
	".wav":  {"audio/wav", "audio/wave", "audio/x-wav"},
	".flac": {"audio/flac", "audio/x-flac"},
	".webm": {"audio/webm"},
}

// audioContentTypeMapNonIOS maps file extensions to their possible MIME types for non-iOS devices.
// Android and other platforms have broader codec support including native 3gp playback.
// Multiple types are provided for better browser compatibility, ordered by preference.
var audioContentTypeMapNonIOS = map[string][]string{
	".mp3":  {"audio/mpeg", "audio/mp3"},
	".3gp":  {"audio/mpeg", "audio/3gpp"}, // Android/PC more lenient, this worked for you
	".3g2":  {"audio/mpeg", "audio/3gpp"}, // Same approach
	".m4a":  {"audio/mp4", "audio/x-m4a"},
	".aac":  {"audio/aac", "audio/aacp"},
	".ogg":  {"audio/ogg", "audio/vorbis"},
	".opus": {"audio/opus", "audio/ogg"},
	".wav":  {"audio/wav", "audio/wave", "audio/x-wav"},
	".flac": {"audio/flac", "audio/x-flac"},
	".webm": {"audio/webm"},
}

// isIOSUserAgent checks if the user agent string indicates an iOS device (iPhone, iPad, iPod).
func isIOSUserAgent(userAgent string) bool {
	ua := strings.ToLower(userAgent)
	return strings.Contains(ua, "iphone") ||
		strings.Contains(ua, "ipad") ||
		strings.Contains(ua, "ipod")
}

// GetAudioContentType determines possible MIME types based on file extension and user agent.
// Returns an array of possible content types, ordered by preference.
// The userAgent parameter is used to optimize media type selection for different platforms.
func GetAudioContentType(filename string, userAgent string) []string {
	ext := strings.ToLower(filepath.Ext(filename))

	// Select the appropriate content type map based on user agent
	var contentTypeMap map[string][]string
	if isIOSUserAgent(userAgent) {
		contentTypeMap = audioContentTypeMapIOS
	} else {
		contentTypeMap = audioContentTypeMapNonIOS
	}

	if types, ok := contentTypeMap[ext]; ok {
		return types
	}
	return []string{"audio/mpeg"} // Default fallback
}
