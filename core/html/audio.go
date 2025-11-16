package html

import (
	"path/filepath"
	"strings"
)

// GetAudioContentType determines the MIME type based on file extension
func GetAudioContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".3gp":
		return "audio/3gpp"
	case ".3g2":
		return "audio/3gpp2"
	case ".m4a":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".ogg":
		return "audio/ogg"
	case ".opus":
		return "audio/opus"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".webm":
		return "audio/webm"
	default:
		return "audio/mpeg" // Default fallback
	}
}
