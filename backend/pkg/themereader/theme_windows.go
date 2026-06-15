//go:build windows

package themereader

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// GetSystemAccentColor reads the Windows DWM accent color from the registry.
// The DWORD value is stored as AABBGGRR; this converts it to #RRGGBB.
// Returns empty string if the value is 0xFF000000 (no accent) or on any error.
func GetSystemAccentColor() string {
	val, err := readDWMAccentColor()
	if err != nil || val == 0xFF000000 {
		return ""
	}

	r := uint8(val & 0xFF)
	g := uint8((val >> 8) & 0xFF)
	b := uint8((val >> 16) & 0xFF)

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// GetSystemAccentSource returns the source identifier for the accent color on this platform.
func GetSystemAccentSource() string {
	return "dwm"
}

func readDWMAccentColor() (uint64, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\DWM`, registry.QUERY_VALUE)
	if err != nil {
		return 0, err
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue("AccentColor")
	if err != nil {
		return 0, err
	}
	return val, nil
}