package html

import (
	"path/filepath"
	"strings"
)

// audioContentTypeMap maps file extensions to their possible MIME types.
// Multiple types are provided for better browser compatibility, ordered by preference.
var audioContentTypeMap = map[string][]string{
	".mp3":  {"audio/mpeg", "audio/mp3"},
	".3gp":  {"audio/3gpp", "audio/mpeg", "audio/3gp"},
	".3g2":  {"audio/3gpp2", "audio/3gp2"},
	".m4a":  {"audio/mp4", "audio/x-m4a"},
	".aac":  {"audio/aac", "audio/aacp"},
	".ogg":  {"audio/ogg", "audio/vorbis"},
	".opus": {"audio/opus", "audio/ogg"},
	".wav":  {"audio/wav", "audio/wave", "audio/x-wav"},
	".flac": {"audio/flac", "audio/x-flac"},
	".webm": {"audio/webm"},
}

// GetAudioContentType determines possible MIME types based on file extension.
// Returns an array of possible content types, ordered by preference.
func GetAudioContentType(filename string) []string {
	ext := strings.ToLower(filepath.Ext(filename))
	if types, ok := audioContentTypeMap[ext]; ok {
		return types
	}
	return []string{"audio/mpeg"} // Default fallback
}
