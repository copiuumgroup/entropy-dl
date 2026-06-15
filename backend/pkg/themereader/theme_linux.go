//go:build linux

package themereader

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// gnomeAccentColorMap maps GNOME accent-color name strings to hex colors.
var gnomeAccentColorMap = map[string]string{
	"blue":   "#4285F4",
	"teal":   "#009688",
	"green":  "#0F9D58",
	"yellow": "#F4B400",
	"orange": "#FF6D00",
	"red":    "#DB4437",
	"pink":   "#E91E63",
	"purple": "#AB47BC",
	"slate":  "#78909C",
}

var (
	accentSource string
	accentMu     sync.RWMutex
)

// GetSystemAccentColor tries GNOME gsettings first, then falls back to KDE
// kdeglobals parsing. Returns empty string if both fail.
func GetSystemAccentColor() string {
	accentMu.Lock()
	accentSource = ""
	accentMu.Unlock()

	if color := getGNOMEAccentColor(); color != "" {
		accentMu.Lock()
		accentSource = "gsettings"
		accentMu.Unlock()
		return color
	}
	if color := getKDEAccentColor(); color != "" {
		accentMu.Lock()
		accentSource = "kdeglobals"
		accentMu.Unlock()
		return color
	}
	return ""
}

// GetSystemAccentSource returns which backend provided the accent color ("gsettings", "kdeglobals", or "").
func GetSystemAccentSource() string {
	accentMu.RLock()
	defer accentMu.RUnlock()
	return accentSource
}

// getGNOMEAccentColor runs `gsettings get org.gnome.desktop.interface accent-color`.
func getGNOMEAccentColor() string {
	out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "accent-color").Output()
	if err != nil {
		return ""
	}

	// gsettings wraps string values in single quotes, e.g. 'blue'
	trimmed := strings.TrimSpace(string(out))
	trimmed = strings.Trim(trimmed, "'\"")

	if color, ok := gnomeAccentColorMap[trimmed]; ok {
		return color
	}
	return ""
}

// getKDEAccentColor parses ~/.config/kdeglobals for the [Colors:Selection] section's
// Background=RR,GG,BB value and converts it to #RRGGBB.
func getKDEAccentColor() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	f, err := os.Open(home + "/.config/kdeglobals")
	if err != nil {
		return ""
	}
	defer f.Close()

	inSection := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[Colors:Selection]") {
			inSection = true
			continue
		}

		if strings.HasPrefix(line, "[") && inSection {
			// Entered a new section, stop
			break
		}

		if inSection && strings.HasPrefix(line, "Background=") {
			val := strings.TrimPrefix(line, "Background=")
			parts := strings.Split(val, ",")
			if len(parts) == 3 {
				r, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
				g, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
				b, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
				return fmt.Sprintf("#%02x%02x%02x", r, g, b)
			}
			return ""
		}
	}

	return ""
}