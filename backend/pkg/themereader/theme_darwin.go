//go:build darwin

package themereader

import (
	"os/exec"
	"strconv"
	"strings"
)

// appleAccentColorMap maps macOS AppleAccentColor integer values to hex colors.
var appleAccentColorMap = map[int]string{
	-1: "#7C4DFF",
	0:  "#F44336",
	1:  "#FF9800",
	2:  "#FFEB3B",
	3:  "#4CAF50",
	4:  "#2196F3",
	5:  "#9C27B0",
	6:  "#E91E63",
}

// GetSystemAccentColor runs `defaults read -g AppleAccentColor` and maps the
// result to a hex color string. Returns empty string on any failure.
func GetSystemAccentColor() string {
	out, err := exec.Command("defaults", "read", "-g", "AppleAccentColor").Output()
	if err != nil {
		return ""
	}

	trimmed := strings.TrimSpace(string(out))
	val, err := strconv.Atoi(trimmed)
	if err != nil {
		return ""
	}

	if color, ok := appleAccentColorMap[val]; ok {
		return color
	}
	return ""
}

// GetSystemAccentSource returns the source identifier for the accent color on this platform.
func GetSystemAccentSource() string {
	return "defaults"
}