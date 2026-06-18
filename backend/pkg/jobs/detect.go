package jobs

import (
	"path/filepath"
	"strings"
	"time"
)

// MediaType is the detected content category, surfaced to the UI as a badge
// and used by smart routing to override format + output directory.
type MediaType string

const (
	MediaUnknown MediaType = ""
	MediaMusic   MediaType = "music"
	MediaAudio   MediaType = "audio"
	MediaVideo   MediaType = "video"
)

// audioDurationThreshold defines the cutoff: items shorter than this are
// classified as audio/music; longer items as video/movie.
const audioDurationThreshold = 12 * time.Minute

// audioExts and videoExts are used as a URL-extension fallback signal.
var audioExts = map[string]bool{
	".mp3": true, ".m4a": true, ".flac": true, ".wav": true,
	".opus": true, ".aac": true, ".ogg": true, ".vorbis": true,
}
var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".webm": true,
	".avi": true, ".mov": true, ".flv": true,
}

// DetectMediaType classifies an item from available signals.
// Priority order:
//
//  1. Extractor/source — yt-music / soundcloud → music; strong video extractors → video
//  2. Duration        — < 12 min → audio, ≥ 12 min → video
//  3. URL extension   — known audio/video file extension
//  4. Fallback        — MediaUnknown (no override applied)
func DetectMediaType(extractor, url string, duration float64) MediaType {
	lower := strings.ToLower(extractor)
	lowerURL := strings.ToLower(url)

	// ── Signal 1: extractor / source ──
	if strings.Contains(lower, "music") || strings.Contains(lower, "soundcloud") {
		return MediaMusic
	}
	// URL-based source hint for SoundCloud short URLs (no extractor metadata).
	if strings.Contains(lowerURL, "soundcloud.com") {
		return MediaMusic
	}

	// ── Signal 2: duration ──
	if duration > 0 {
		d := time.Duration(duration * float64(time.Second))
		if d < audioDurationThreshold {
			return MediaAudio
		}
		return MediaVideo
	}

	// ── Signal 3: URL extension ──
	if ext := strings.ToLower(filepath.Ext(lowerURL)); ext != "" {
		if audioExts[ext] {
			return MediaAudio
		}
		if videoExts[ext] {
			return MediaVideo
		}
	}

	return MediaUnknown
}

// WithMediaType returns a copy of Options with format and directory adjusted
// according to the detected media type. When mt is MediaUnknown the original
// options are returned unchanged.
func (o Options) WithMediaType(mt MediaType) Options {
	if mt == MediaUnknown {
		return o
	}
	switch mt {
	case MediaMusic, MediaAudio:
		o.Format = "mp3"
		o.MediaType = string(mt)
	case MediaVideo:
		o.Format = "best"
		o.MediaType = string(mt)
	}
	return o
}
