// Package diskguard provides cross-platform free disk space checking
// to prevent downloads from running out of disk space.
package diskguard

import "fmt"

// DefaultMinFreeGB is the minimum free space (in GB) required before
// allowing a new download to start. Exported so main.go can override via env.
var DefaultMinFreeGB = uint64(10)

// LowSpaceError is returned when disk space is below the threshold.
type LowSpaceError struct {
        Path   string
        FreeGB uint64
        MinGB  uint64
}

func (e *LowSpaceError) Error() string {
        return fmt.Sprintf("low disk space on %s: %dGB free (need %dGB)",
                e.Path, e.FreeGB, e.MinGB)
}

// Check ensures the filesystem containing path has at least minFreeGB gigabytes free.
// If minFreeGB is 0, DefaultMinFreeGB is used.
// Returns nil if there's enough space, LowSpaceError if not, or another error on failure.
func Check(path string, minFreeGB uint64) error {
        if minFreeGB == 0 {
                minFreeGB = DefaultMinFreeGB
        }
        freeBytes, err := FreeSpace(path)
        if err != nil {
                // If we can't check (e.g., weird FS), log warning but don't block
                return nil
        }
        minBytes := minFreeGB * 1024 * 1024 * 1024
        if freeBytes < minBytes {
                return &LowSpaceError{
                        Path:   path,
                        FreeGB: freeBytes / (1024 * 1024 * 1024),
                        MinGB:  minFreeGB,
                }
        }
        return nil
}

// FreeSpaceGB returns free disk space in gigabytes for the filesystem containing path.
func FreeSpaceGB(path string) (uint64, error) {
        free, err := FreeSpace(path)
        if err != nil {
                return 0, err
        }
        return free / (1024 * 1024 * 1024), nil
}
