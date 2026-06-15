// Package importer parses URLs from various file formats for bulk import.
// Supports: plain text (one URL per line), .m3u/.m3u8 playlists, and CSV files
// (YouTube History export, generic CSV with URL columns).
package importer

import (
        "bufio"
        "encoding/csv"
        "fmt"
        "io"
        "path/filepath"
        "strings"

        "entropy-gui/pkg/cleaner"
)

// Result contains parsed URLs and any warnings from an import operation.
type Result struct {
        URLs     []string `json:"urls"`
        Warnings []string `json:"warnings"`
        Skipped  int      `json:"skipped"` // lines that looked valid but weren't URLs
}

// Parse reads from r and extracts URLs based on the filename extension.
// For unknown extensions, falls back to plain text (one URL per line).
func Parse(r io.Reader, filename string) (*Result, error) {
        ext := strings.ToLower(filepath.Ext(filename))
        switch ext {
        case ".m3u", ".m3u8":
                return parseM3U(r)
        case ".csv":
                return parseCSV(r)
        default:
                return parseText(r)
        }
}

// parseText handles plain text files with one URL per line.
// Lines starting with # are treated as comments (compatible with m3u comments).
func parseText(r io.Reader) (*Result, error) {
        result := &Result{}
        scanner := bufio.NewScanner(r)
        scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // up to 1MB lines

        for scanner.Scan() {
                line := strings.TrimSpace(scanner.Text())
                if line == "" || strings.HasPrefix(line, "#") {
                        continue
                }
                // Try to parse as URL
                cleaned := cleaner.CleanURL(line)
                if cleaned != "" {
                        result.URLs = append(result.URLs, cleaned)
                } else if strings.Contains(line, "://") {
                        // Had a scheme but cleaner rejected it — warn
                        result.Skipped++
                        result.Warnings = append(result.Warnings, fmt.Sprintf("skipped invalid URL: %.80s", line))
                }
        }

        if err := scanner.Err(); err != nil {
                return nil, fmt.Errorf("reading input: %w", err)
        }
        return result, nil
}

// parseM3U handles M3U/M3U8 playlist files.
// Only reads #EXTINF lines for title metadata (no audio streaming).
// Extracts URLs from non-comment, non-directive lines.
func parseM3U(r io.Reader) (*Result, error) {
        result := &Result{}
        scanner := bufio.NewScanner(r)
        scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

        for scanner.Scan() {
                line := strings.TrimSpace(scanner.Text())
                // Skip comments and directives (but not URLs)
                if line == "" || strings.HasPrefix(line, "#EXTM3U") || strings.HasPrefix(line, "#EXTINF") {
                        continue
                }
                if strings.HasPrefix(line, "#") {
                        continue
                }

                cleaned := cleaner.CleanURL(line)
                if cleaned != "" {
                        result.URLs = append(result.URLs, cleaned)
                }
        }

        if err := scanner.Err(); err != nil {
                return nil, fmt.Errorf("reading m3u: %w", err)
        }
        return result, nil
}

// parseCSV handles CSV files. It scans all cells looking for values that
// look like HTTP(S) URLs. This works for YouTube History exports and
// generic CSV files without needing to know the column schema.
func parseCSV(r io.Reader) (*Result, error) {
        result := &Result{}
        reader := csv.NewReader(r)
        // Allow variable-length records
        reader.FieldsPerRecord = -1
        // Allow unescaped quotes
        reader.LazyQuotes = true

        for {
                record, err := reader.Read()
                if err == io.EOF {
                        break
                }
                if err != nil {
                        // If we get a parse error on one line, skip and continue
                        result.Warnings = append(result.Warnings, fmt.Sprintf("CSV parse error: %v", err))
                        continue
                }
                for _, cell := range record {
                        cell = strings.TrimSpace(cell)
                        if cell == "" {
                                continue
                        }
                        // Check if it looks like a URL
                        if strings.HasPrefix(cell, "http://") || strings.HasPrefix(cell, "https://") {
                                cleaned := cleaner.CleanURL(cell)
                                if cleaned != "" {
                                        result.URLs = append(result.URLs, cleaned)
                                }
                        }
                }
        }

        return result, nil
}

// Deduplicate removes duplicate URLs from the slice, preserving order.
// Returns the deduplicated slice and the count of removed duplicates.
func Deduplicate(urls []string) ([]string, int) {
        seen := make(map[string]bool, len(urls))
        deduped := make([]string, 0, len(urls))
        removed := 0
        for _, u := range urls {
                if seen[u] {
                        removed++
                        continue
                }
                seen[u] = true
                deduped = append(deduped, u)
        }
        return deduped, removed
}
