//go:build !windows && !darwin && !linux

package themereader

// GetSystemAccentColor returns empty string on unsupported platforms.
func GetSystemAccentColor() string {
	return ""
}

// GetSystemAccentSource returns empty string on unsupported platforms.
func GetSystemAccentSource() string {
	return ""
}