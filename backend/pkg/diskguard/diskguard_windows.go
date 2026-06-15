//go:build windows

package diskguard

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	modkernel32     = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFree = modkernel32.NewProc("GetDiskFreeSpaceExW")
)

// FreeSpace returns the number of free bytes on the filesystem containing path.
func FreeSpace(path string) (uint64, error) {
	// GetDiskFreeSpaceEx needs a root path like "C:\"
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, err
	}
	// Use the root of the drive
	root := filepath.VolumeName(abs) + string(filepath.Separator)

	var freeBytes uint64
	var totalBytes uint64
	var availBytes uint64

	ptrRoot, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0, err
	}

	ret, _, err := procGetDiskFree.Call(
		uintptr(unsafe.Pointer(ptrRoot)),
		uintptr(unsafe.Pointer(&freeBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&availBytes)),
	)
	if ret == 0 {
		return 0, err
	}
	return freeBytes, nil
}
