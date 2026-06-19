package library

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestDir creates a temporary directory with sample files and subdirs.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Root-level files
	writeFile(t, dir, "song_one.mp3", "audio data")
	writeFile(t, dir, "song_two.flac", "flac data")
	writeFile(t, dir, "video_one.mp4", "video data")

	// Hidden / junk files (should be filtered)
	writeFile(t, dir, ".hidden_file.txt", "hidden")
	writeFile(t, dir, "dead_links.csv", "csv data")
	writeFile(t, dir, "partial.mp3.part", "partial data")
	writeFile(t, dir, "meta.mp3.ytdl", "ytdl meta")
	writeFile(t, dir, "temp.wav.tmp", "temp data")

	// Subdirectory
	sub := filepath.Join(dir, "Albums")
	os.MkdirAll(sub, 0o755)
	writeFile(t, sub, "track_a.m4a", "m4a data")
	writeFile(t, sub, "track_b.opus", "opus data")

	// Nested subdirectory
	nested := filepath.Join(sub, "2024")
	os.MkdirAll(nested, 0o755)
	writeFile(t, nested, "live.mp3", "live data")

	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile(%s): %v", name, err)
	}
}

// --- ListDir ---

func TestListDir_RootListing(t *testing.T) {
	dir := setupTestDir(t)
	entries, err := ListDir(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	// Should see: 1 dir (Albums), 3 files (song_one.mp3, song_two.flac, video_one.mp4)
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries (1 dir + 3 files), got %d: %+v", len(entries), entries)
	}

	// First entry should be the directory
	if !entries[0].IsDir || entries[0].Name != "Albums" {
		t.Fatalf("first entry should be Albums dir, got %+v", entries[0])
	}

	// Files should follow alphabetically
	var files []Entry
	for _, e := range entries {
		if !e.IsDir {
			files = append(files, e)
		}
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	if files[0].Name != "song_one.mp3" {
		t.Errorf("first file should be song_one.mp3, got %s", files[0].Name)
	}
	if files[0].Ext != "mp3" {
		t.Errorf("ext should be mp3, got %s", files[0].Ext)
	}
}

func TestListDir_Subdirectory(t *testing.T) {
	dir := setupTestDir(t)
	entries, err := ListDir(dir, "Albums")
	if err != nil {
		t.Fatal(err)
	}

	// Should see: 1 dir (2024), 2 files (track_a.m4a, track_b.opus)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(entries), entries)
	}

	if !entries[0].IsDir || entries[0].Name != "2024" {
		t.Fatalf("first entry should be 2024 dir, got %+v", entries[0])
	}
}

func TestListDir_NestedSubdirectory(t *testing.T) {
	dir := setupTestDir(t)
	entries, err := ListDir(dir, filepath.Join("Albums", "2024"))
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "live.mp3" {
		t.Errorf("expected live.mp3, got %s", entries[0].Name)
	}
}

func TestListDir_HiddenFilesFiltered(t *testing.T) {
	dir := setupTestDir(t)
	entries, err := ListDir(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	// None of these should appear
	for _, e := range entries {
		if e.Name == ".hidden_file.txt" || e.Name == "dead_links.csv" ||
			e.Name == "partial.mp3.part" || e.Name == "meta.mp3.ytdl" ||
			e.Name == "temp.wav.tmp" {
			t.Errorf("hidden/junk file should be filtered: %s", e.Name)
		}
	}
}

func TestListDir_PathTraversalPrevented(t *testing.T) {
	dir := setupTestDir(t)
	_, err := ListDir(dir, "../../etc")
	if err == nil {
		t.Fatal("should reject path traversal")
	}
}

func TestListDir_NonexistentDirectory(t *testing.T) {
	dir := setupTestDir(t)
	_, err := ListDir(dir, "Nonexistent")
	if err == nil {
		t.Fatal("should error for nonexistent directory")
	}
}

func TestListDir_FileAsPath(t *testing.T) {
	dir := setupTestDir(t)
	_, err := ListDir(dir, "song_one.mp3")
	if err == nil {
		t.Fatal("should error when path is a file, not a directory")
	}
}

// --- ResolveFile ---

func TestResolveFile_ValidPath(t *testing.T) {
	dir := setupTestDir(t)
	abs, err := ResolveFile(dir, "song_one.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(abs) != "song_one.mp3" {
		t.Errorf("unexpected resolved path: %s", abs)
	}
}

func TestResolveFile_NestedFile(t *testing.T) {
	dir := setupTestDir(t)
	abs, err := ResolveFile(dir, filepath.Join("Albums", "track_a.m4a"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(abs) != "track_a.m4a" {
		t.Errorf("unexpected resolved path: %s", abs)
	}
}

func TestResolveFile_TraversalPrevented(t *testing.T) {
	dir := setupTestDir(t)
	_, err := ResolveFile(dir, "../../etc/passwd")
	if err == nil {
		t.Fatal("should reject path traversal")
	}
}

func TestResolveFile_DirectoryRejected(t *testing.T) {
	dir := setupTestDir(t)
	_, err := ResolveFile(dir, "Albums")
	if err == nil {
		t.Fatal("should reject directory as file")
	}
}

func TestResolveFile_Nonexistent(t *testing.T) {
	dir := setupTestDir(t)
	_, err := ResolveFile(dir, "nonexistent.mp3")
	if err == nil {
		t.Fatal("should reject nonexistent file")
	}
}

func TestResolveFile_HiddenFileStillResolves(t *testing.T) {
	// ResolveFile doesn't filter hidden files — that's ListDir's job.
	// This ensures ResolveFile can still serve them if someone has a direct link.
	dir := setupTestDir(t)
	_, err := ResolveFile(dir, "dead_links.csv")
	if err != nil {
		t.Fatalf("ResolveFile should resolve hidden files (filtering is ListDir's job): %v", err)
	}
}

// --- DirCount ---

func TestDirCount(t *testing.T) {
	dir := setupTestDir(t)
	count := DirCount(dir, "")
	if count != 4 {
		t.Fatalf("root should have 4 entries, got %d", count)
	}
	count = DirCount(dir, "Albums")
	if count != 3 {
		t.Fatalf("Albums should have 3 entries, got %d", count)
	}
	count = DirCount(dir, "Nonexistent")
	if count != 0 {
		t.Fatalf("nonexistent dir should return 0, got %d", count)
	}
}

// --- isHidden ---

func TestIsHidden(t *testing.T) {
	tests := []struct {
		name string
		hide bool
	}{
		{".gitignore", true},
		{".DS_Store", true},
		{"song.mp3", false},
		{"dead_links.csv", true},
		{"partial.mp3.part", true},
		{"info.mp4.ytdl", true},
		{"temp.wav.tmp", true},
		{"VIDEO.mp4", false}, // case insensitivity on extensions
		{"DEAD_LINKS.CSV", true},
	}

	for _, tt := range tests {
		got := isHidden(tt.name)
		if got != tt.hide {
			t.Errorf("isHidden(%q) = %v, want %v", tt.name, got, tt.hide)
		}
	}
}
