package main

import (
        "context"
        "net/http"
        "os"
        "os/exec"
        "path/filepath"
        "runtime"
        "time"

        "entropy-gui/pkg/cmdutil"
)

// launchAppWindow opens the app in a Chrome/Edge "app mode" chromeless window
// (looks like a native desktop window). Falls back to the default browser
// if no Chromium-based browser is found.
func launchAppWindow(url string) {
        // Wait briefly for the server to start listening.
        waitForServer(url, 3*time.Second)

        // Try Chromium-based browsers first — they support --app= chromeless mode.
        candidates := chromiumCandidates()
        if candidates != nil {
                for _, exe := range candidates {
                        if tryLaunch(exe, "--app="+url, "--new-window",
                                "--no-first-run", "--no-default-browser-check") {
                                return
                        }
                }
        }

        // Fall back to Firefox-based browsers — no app mode, but opens a new window.
        ffCandidates := firefoxCandidates()
        if ffCandidates != nil {
                for _, exe := range ffCandidates {
                        if tryLaunch(exe, "--new-window", url) {
                                return
                        }
                }
        }

        // Final fallback: OS default handler.
        switch runtime.GOOS {
        case "windows":
                cmd := exec.Command("cmd", "/c", "start", "", url)
                cmdutil.PrepareCmd(cmd)
                _ = cmd.Start()
        case "darwin":
                _ = exec.Command("open", url).Start()
        case "linux":
                _ = exec.Command("xdg-open", url).Start()
        }
}

// tryLaunch resolves and starts an executable with the given args.
// Returns true if the process started successfully.
func tryLaunch(exe string, args ...string) bool {
        var cmdPath string
        if filepath.IsAbs(exe) {
                if _, err := os.Stat(exe); err != nil {
                        return false
                }
                cmdPath = exe
        } else {
                var err error
                cmdPath, err = exec.LookPath(exe)
                if err != nil {
                        return false
                }
        }
        cmd := exec.Command(cmdPath, args...)
        return cmd.Start() == nil
}

func chromiumCandidates() []string {
        switch runtime.GOOS {
        case "windows":
                pf := envOrEmpty("ProgramFiles", `C:\Program Files`)
                pfx86 := envOrEmpty("ProgramFiles(x86)", `C:\Program Files (x86)`)
                local := envOrEmpty("LocalAppData", "")

                return []string{
                        // Google Chrome
                        pf + `\Google\Chrome\Application\chrome.exe`,
                        pfx86 + `\Google\Chrome\Application\chrome.exe`,
                        local + `\Google\Chrome\Application\chrome.exe`,
                        // Microsoft Edge (pre-installed on Win10/11)
                        pf + `\Microsoft\Edge\Application\msedge.exe`,
                        pfx86 + `\Microsoft\Edge\Application\msedge.exe`,
                        // Brave
                        pf + `\BraveSoftware\Brave-Browser\Application\brave.exe`,
                        pfx86 + `\BraveSoftware\Brave-Browser\Application\brave.exe`,
                        local + `\BraveSoftware\Brave-Browser\Application\brave.exe`,
                        // Vivaldi
                        local + `\Vivaldi\Application\vivaldi.exe`,
                        pf + `\Vivaldi\Application\vivaldi.exe`,
                        // Opera & Opera GX
                        local + `\Programs\Opera\opera.exe`,
                        local + `\Programs\Opera GX\opera.exe`,
                        pf + `\Opera\opera.exe`,
                        // Chromium (unbranded)
                        local + `\Chromium\Application\chromium.exe`,
                        pf + `\Chromium\Application\chromium.exe`,
                        // Arc Browser
                        local + `\Programs\Arc\Arc.exe`,
                        // Thorium
                        pf + `\Thorium\Application\thorium.exe`,
                }
        case "darwin":
                return []string{
                        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
                        "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
                        "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
                        "/Applications/Vivaldi.app/Contents/MacOS/Vivaldi",
                        "/Applications/Opera.app/Contents/MacOS/Opera",
                        "/Applications/Chromium.app/Contents/MacOS/Chromium",
                        "/Applications/Arc.app/Contents/MacOS/Arc",
                        "/Applications/Thorium.app/Contents/MacOS/Thorium",
                }
        case "linux":
                return []string{
                        // Google Chrome
                        "google-chrome",
                        "google-chrome-stable",
                        "google-chrome-beta",
                        "google-chrome-unstable",
                        // Chromium
                        "chromium",
                        "chromium-browser",
                        "ungoogled-chromium",
                        // Brave (official + AUR package names)
                        "brave-browser",
                        "brave",
                        "brave-bin",
                        // Microsoft Edge
                        "microsoft-edge",
                        "microsoft-edge-stable",
                        "microsoft-edge-beta",
                        "microsoft-edge-dev",
                        // Vivaldi
                        "vivaldi",
                        "vivaldi-stable",
                        // Opera
                        "opera",
                        "opera-stable",
                        // Thorium
                        "thorium-browser",
                        // Arc (Linux beta)
                        "arc",
                }
        }
        return nil
}

func firefoxCandidates() []string {
        switch runtime.GOOS {
        case "windows":
                pf := envOrEmpty("ProgramFiles", `C:\Program Files`)
                pfx86 := envOrEmpty("ProgramFiles(x86)", `C:\Program Files (x86)`)
                local := envOrEmpty("LocalAppData", "")

                return []string{
                        // Mozilla Firefox
                        pf + `\Mozilla Firefox\firefox.exe`,
                        pfx86 + `\Mozilla Firefox\firefox.exe`,
                        local + `\Mozilla Firefox\firefox.exe`,
                        // Firefox Developer Edition / Nightly
                        pf + `\Firefox Developer Edition\firefox.exe`,
                        pfx86 + `\Firefox Developer Edition\firefox.exe`,
                        pf + `\Firefox Nightly\firefox.exe`,
                        // LibreWolf
                        pf + `\LibreWolf\librewolf.exe`,
                        pfx86 + `\LibreWolf\librewolf.exe`,
                        // Waterfox
                        pf + `\Waterfox\waterfox.exe`,
                        pfx86 + `\Waterfox\waterfox.exe`,
                        // Floorp
                        pf + `\Floorp\floorp.exe`,
                        pfx86 + `\Floorp\floorp.exe`,
                        // Zen Browser
                        local + `\Programs\Zen Browser\zen.exe`,
                        pf + `\Zen Browser\zen.exe`,
                        // Mullvad Browser
                        pf + `\Mullvad Browser\mullvadbrowser.exe`,
                }
        case "darwin":
                return []string{
                        "/Applications/Firefox.app/Contents/MacOS/firefox",
                        "/Applications/Firefox Developer Edition.app/Contents/MacOS/firefox",
                        "/Applications/Firefox Nightly.app/Contents/MacOS/firefox",
                        "/Applications/LibreWolf.app/Contents/MacOS/librewolf",
                        "/Applications/Waterfox.app/Contents/MacOS/waterfox",
                        "/Applications/Floorp.app/Contents/MacOS/floorp",
                        "/Applications/Zen Browser.app/Contents/MacOS/zen",
                        "/Applications/Mullvad Browser.app/Contents/MacOS/mullvadbrowser",
                }
        case "linux":
                return []string{
                        // Mozilla Firefox
                        "firefox",
                        "firefox-esr",
                        "firefox-developer-edition",
                        "firefox-nightly",
                        // LibreWolf
                        "librewolf",
                        // Waterfox
                        "waterfox",
                        "waterfox-current",
                        // Floorp
                        "floorp",
                        // Zen Browser
                        "zen-browser",
                        "zen",
                        // Mullvad Browser
                        "mullvad-browser",
                }
        }
        return nil
}

// waitForServer polls the server health endpoint until it's up or timeout
func waitForServer(url string, timeout time.Duration) {
        deadline := time.Now().Add(timeout)
        client := &http.Client{Timeout: 250 * time.Millisecond}
        for time.Now().Before(deadline) {
                ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
                req, _ := http.NewRequestWithContext(ctx, "GET", url+"/api/health", nil)
                resp, err := client.Do(req)
                cancel()
                if err == nil {
                        resp.Body.Close()
                        return
                }
                time.Sleep(150 * time.Millisecond)
        }
}

func envOrEmpty(k, def string) string {
        if v := osGetenv(k); v != "" {
                return v
        }
        return def
}

// osGetenv is wrapped so the package compiles cleanly on non-windows builds.
func osGetenv(k string) string { return getenv(k, "") }

// shutdownAfter is defined in exit.go (needs access to signal cancellation).
