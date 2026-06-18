//go:build windows

package themereader

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// GetSystemAccentColor reads the Windows accent color from the registry.
//
// It tries the more reliable sources first on modern Windows 10/11:
//  1. ColorizationColor — the active title bar / DWM glass color. Always set
//     and updated live when the user picks an accent. Stored as 0xAABBGGRR.
//  2. AccentColor — the raw accent DWORD under the DWM key. On Windows 11
//     this is frequently the sentinel 0xFF000000 ("no custom accent") even
//     when the user has a color, so it's only a fallback.
//
// Returns empty string if no usable color is found or on any error.
func GetSystemAccentColor() string {
	if hex := fromColorizationColor(); hex != "" {
		return hex
	}
	if hex := fromAccentColor(); hex != "" {
		return hex
	}
	return ""
}

// GetSystemAccentSource returns the source identifier for the accent color on this platform.
func GetSystemAccentSource() string {
	return "dwm"
}

// fromColorizationColor reads HKCU\Software\Microsoft\Windows\DWM\ColorizationColor.
// This DWORD is 0xAABBGGRR (alpha typically 0xFF). Strips the alpha byte and
// returns #RRGGBB.
func fromColorizationColor() string {
	val, err := readDWMUInt("ColorizationColor")
	if err != nil {
		return ""
	}
	// Mask off the alpha byte; format the RGB portion as #RRGGBB.
	r := uint8((val >> 16) & 0xFF)
	g := uint8((val >> 8) & 0xFF)
	b := uint8(val & 0xFF)
	// Filter out the classic Aero blue default and pure black, which almost
	// always indicate the value hasn't been customized.
	if r == 0 && g == 0 && b == 0 {
		return ""
	}
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// fromAccentColor reads HKCU\Software\Microsoft\Windows\DWM\AccentColor.
// The DWORD is 0xAABBGGRR. Returns empty string for the sentinel 0xFF000000
// (meaning "no custom accent"), since that would render as opaque black.
func fromAccentColor() string {
	val, err := readDWMUInt("AccentColor")
	if err != nil {
		return ""
	}
	// Sentinel value meaning "no custom accent set" — opaque black.
	if (val & 0xFFFFFFFF) == 0xFF000000 || val == 0xFF000000 {
		return ""
	}
	r := uint8(val & 0xFF)
	g := uint8((val >> 8) & 0xFF)
	b := uint8((val >> 16) & 0xFF)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// readDWMUInt opens HKCU\Software\Microsoft\Windows\DWM and reads a uint32
// value by name, returning it as uint64 for safe bit manipulation.
func readDWMUInt(name string) (uint64, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\DWM`, registry.QUERY_VALUE)
	if err != nil {
		return 0, err
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue(name)
	if err != nil {
		return 0, err
	}
	return val, nil
}
