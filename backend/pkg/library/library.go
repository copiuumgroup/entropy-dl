// Package library provides traversal-safe directory listing and file resolution
// for the audio/video download directories. All public functions guard against
// path-traversal attacks (e.g., ../../etc/passwd).
package library

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Entry represents a single file or directory in the library listing.
type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`     // relative to the root (audio/video dir)
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`     // bytes; 0 for directories
	ModTime string `json:"mod_time"` // ISO 8601
	Ext     string `json:"ext"`      // lowercase file extension without dot
}

// hiddenSuffixes are file extensions that should never appear in listings
// (temporary/incomplete files from yt-dlp).
var hiddenSuffixes = []string{".part", ".ytdl", ".tmp"}

// hiddenNames are filenames (not extensions) to exclude from listings.
var hiddenNames = map[string]bool{
	"dead_links.csv": true,
}

// isHidden returns true if a file should be excluded from library listings.
func isHidden(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	if hiddenNames[strings.ToLower(name)] {
		return true
	}
	lower := strings.ToLower(name)
	for _, suffix := range hiddenSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// ListDir returns the entries in a directory, scoped to one root.
// basePath is the allowed root directory (e.g., AudioDir or VideoDir).
// relPath is the user-requested subdirectory (may be "" for the root itself).
// Entries are sorted: directories first, then files alphabetically.
// Returns an error if the path is invalid, traversal is attempted, or the
// directory cannot be read.
func ListDir(basePath, relPath string) ([]Entry, error) {
	// Sanitize and resolve the target directory.
	target, err := resolveDir(basePath, relPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory: %w", err)
	}

	var out []Entry
	for _, e := range entries {
		name := e.Name()
		if isHidden(name) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue // skip unreadable entries
		}

		entry := Entry{
			Name:    name,
			Path:    filepath.Join(relPath, name),
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		}

		if !info.IsDir() {
			entry.Ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
		}

		out = append(out, entry)
	}

	// Sort: directories first, then files, both alphabetically.
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir // dirs before files
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})

	return out, nil
}

// ResolveFile safely resolves a relative path within a root directory and
// returns the absolute file path. Returns an error if traversal is attempted
// or if the path does not exist.
func ResolveFile(basePath, relPath string) (string, error) {
	if strings.Contains(relPath, "..") {
		return "", fmt.Errorf("path traversal not allowed")
	}

	abs := filepath.Join(basePath, relPath)

	// Verify the resolved path is still within basePath.
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	baseAbs, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("invalid base path: %w", err)
	}
	if !strings.HasPrefix(abs, baseAbs+string(filepath.Separator)) && abs != baseAbs {
		return "", fmt.Errorf("path escapes library root")
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file")
	}

	return abs, nil
}

// resolveDir is like ResolveFile but for directories (used internally by ListDir).
func resolveDir(basePath, relPath string) (string, error) {
	if strings.Contains(relPath, "..") {
		return "", fmt.Errorf("path traversal not allowed")
	}

	abs := filepath.Join(basePath, relPath)

	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	baseAbs, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("invalid base path: %w", err)
	}
	if !strings.HasPrefix(abs, baseAbs+string(filepath.Separator)) && abs != baseAbs {
		return "", fmt.Errorf("path escapes library root")
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("directory not found: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is a file, not a directory")
	}

	return abs, nil
}

// DirCount returns the number of visible entries in a directory (for
// displaying item counts next to directory entries in the UI).
// Returns 0 if the directory cannot be read.
func DirCount(basePath, relPath string) int {
	entries, err := ListDir(basePath, relPath)
	if err != nil {
		return 0
	}
	return len(entries)
}
