package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

        "entropy-gui/pkg/cleaner"
        "entropy-gui/pkg/cmdutil"
        "entropy-gui/pkg/diskguard"
        "entropy-gui/pkg/importer"
        "entropy-gui/pkg/jobs"
        "entropy-gui/pkg/ratelimit"
        "entropy-gui/pkg/store"
        "entropy-gui/pkg/themereader"
        "entropy-gui/pkg/ytdlp"

	"github.com/google/uuid"
)

// version is set at build time via -ldflags "-X main.version=dev"
var version = "dev"

// maxBodyBytes limits JSON request body size to prevent OOM attacks.
const maxBodyBytes = 1 << 20 // 1 MB

// maxURLsPerRequest caps the number of URLs/items per batch request to prevent resource exhaustion.
const maxURLsPerRequest = 500

// sessionCookieName is the HttpOnly cookie set on every response.
// Required by /api/shutdown to prevent cross-origin CSRF.
const sessionCookieName = "entropy_session"

type Server struct {
        mgr             *jobs.Manager
        ytdlpBin        string
        store           *store.Store
        shutdownToken   string // CSRF token; set as HttpOnly cookie
        secureCookie    bool   // true if server runs over HTTPS
        lastToolUpdate  time.Time // rate-limit tool updates
        toolUpdateMu    sync.Mutex
}

func main() {
        loadEnvFile("/app/backend/.env")
        loadEnvFile(".env") // also try .env beside the binary (for portable builds)

        port := getenv("PORT", "8001")
        ytdlpBin := resolveTool("YTDLP_BIN", "yt-dlp")
        aria2cBin := resolveTool("ARIA2C_BIN", "aria2c")
        ffmpegBin := resolveTool("FFMPEG_BIN", "ffmpeg")
        defaultOutputDir := getenv("DOWNLOAD_DIR", defaultDownloadDir())
        statePath := resolveStatePath("STATE_FILE", "/app/backend/entropy.db")
        workers, _ := strconv.Atoi(getenv("MAX_WORKERS", "2"))
        if workers <= 0 {
                workers = 2
        }

        // State File Corruption Protection (A/B swap): keep a .bak copy
        // just in case the bbolt file is corrupted due to kernel panic/hard reset.
        if _, err := os.Stat(statePath); err == nil {
                if b, err := os.ReadFile(statePath); err == nil {
                        _ = os.WriteFile(statePath+".bak", b, 0o600)
                }
        }

        st, err := store.New(statePath)
        if err != nil {
                // If corrupted, try to restore from backup
                log.Printf("store: failed to open %q (%v), attempting restore from .bak", statePath, err)
                if b, bakErr := os.ReadFile(statePath + ".bak"); bakErr == nil {
                        _ = os.WriteFile(statePath, b, 0o600)
                        st, err = store.New(statePath)
                }
                if err != nil {
                        log.Fatalf("store: fatal: %v", err)
                }
        }
        // FIX #5: ensure store is closed on exit
        defer func() {
                log.Println("[entropy] closing store")
                if err := st.Close(); err != nil {
                        log.Printf("warning: store close: %v", err)
                }
        }()

        settings, err := st.LoadSettings()
        if err != nil {
                log.Printf("warning: could not load settings: %v", err)
        }

        // Persisted settings override env defaults if present.
        audioDir := filepath.Join(defaultOutputDir, "Entropy Music")
        videoDir := filepath.Join(defaultOutputDir, "Entropy Videos")
        if home, err := os.UserHomeDir(); err == nil {
                audioDir = filepath.Join(home, "Music", "Entropy")
                videoDir = filepath.Join(home, "Videos", "Entropy")
        }

        if settings.AudioDir != "" {
                audioDir = settings.AudioDir
        }
        if settings.VideoDir != "" {
                videoDir = settings.VideoDir
        }
        if settings.MaxWorkers > 0 {
                workers = settings.MaxWorkers
        }

        ensureWritable := func(dir string) string {
                if err := os.MkdirAll(dir, 0o755); err != nil {
                        log.Printf("warning: output dir %q not writable, falling back to local downloads", dir)
                        d := filepath.Join("downloads", filepath.Base(dir))
                        _ = os.MkdirAll(d, 0o755)
                        return d
                }
                return dir
        }
        audioDir = ensureWritable(audioDir)
        videoDir = ensureWritable(videoDir)

        mgr := jobs.NewManager(ytdlpBin, aria2cBin, ffmpegBin, audioDir, videoDir, workers)

        // Restore persisted bandwidth limit if set
        if settings.BandwidthLimit != "" {
                mgr.SetBandwidth(settings.BandwidthLimit)
        }
        // Restore smart routing preference
        if settings.SmartRouting {
                mgr.SetSmartRouting(true)
        }

        // Restore jobs from previous run
        rawJobs, err := st.LoadJobs()
        if err != nil {
                log.Printf("warning: could not load jobs: %v", err)
        }
        var restored []*jobs.Job
        for _, raw := range rawJobs {
                var j jobs.Job
                if err := json.Unmarshal(raw, &j); err == nil {
                        restored = append(restored, &j)
                }
        }

        saveJob := func(j *jobs.Job) {
                if b, err := json.Marshal(j); err == nil {
                        _ = st.SaveJob(j.ID, b)
                }
        }
        deleteJob := func(id string) {
                _ = st.DeleteJob(id)
        }
        mgr.AttachPersistence(saveJob, deleteJob, restored)

        // FIX #1: generate a unique session token for CSRF protection on shutdown.
        // This token is set as an HttpOnly cookie; SameSite=Lax prevents
        // cross-origin requests from including it.
        shutdownToken := uuid.NewString()
        secureCookie := getenv("USE_HTTPS", "0") == "1"

        srv := &Server{mgr: mgr, ytdlpBin: ytdlpBin, store: st, shutdownToken: shutdownToken, secureCookie: secureCookie}

        mux := http.NewServeMux()
        mux.HandleFunc("/api/health", srv.handleHealth)
        mux.HandleFunc("/api/config", srv.handleConfig)
        mux.HandleFunc("/api/settings", srv.handleSettings)
        mux.HandleFunc("/api/search", srv.handleSearch)
        mux.HandleFunc("/api/clean-url", srv.handleCleanURL)
        mux.HandleFunc("/api/jobs", srv.handleJobs)
        mux.HandleFunc("/api/jobs/", srv.handleJobByID)
        mux.HandleFunc("/api/jobs/clear", srv.handleJobsClear)
        mux.HandleFunc("/api/jobs/stream", srv.handleStream)
        mux.HandleFunc("/api/logs", srv.handleLogs)
        mux.HandleFunc("/api/env", srv.handleEnv)
        mux.HandleFunc("/api/onboarding", srv.handleSetOnboarding)
        mux.HandleFunc("/api/tools/update", srv.handleToolUpdate)
        mux.HandleFunc("/api/concurrency", srv.handleConcurrency)
        mux.HandleFunc("/api/bandwidth", srv.handleBandwidth)
        mux.HandleFunc("/api/smart-routing", srv.handleSmartRouting)
        mux.HandleFunc("/api/shutdown", srv.handleShutdown)
        mux.HandleFunc("/api/jobs/retry-failed", srv.handleRetryFailed)
        mux.HandleFunc("/api/stats", srv.handleStats)
        mux.HandleFunc("/api/import", srv.handleImport)
        mux.HandleFunc("/api/theme", srv.handleTheme)

        // Serve the React frontend. Priority:
        //   1. ./web/ folder beside the binary  (build-time-portable bundle)
        //   2. Embedded FS baked into the binary (single-file deployment)
        //   3. Stub welcome page
        webDir := resolveWebDir()
        if webDir != "" {
                fs := http.FileServer(http.Dir(webDir))
                mux.Handle("/", spaHandler(webDir, fs))
                log.Printf("serving frontend from %s (external)", webDir)
        } else if efs := embeddedWeb(); efs != nil {
                mux.Handle("/", embeddedSPAHandler(efs))
                log.Printf("serving frontend from embedded FS")
        } else {
                mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
                        if r.URL.Path != "/" {
                                http.NotFound(w, r)
                                return
                        }
                        w.Header().Set("Content-Type", "text/html")
                        fmt.Fprintf(w, `<!doctype html><html><body style="background:#000;color:#fff;font-family:monospace;padding:40px">
<h1>ENTROPY DL</h1>
<p>Backend running on port %s. Frontend build not found.</p>
<p>API: <a style="color:#4ade80" href="/api/health">/api/health</a></p>
</body></html>`, port)
                })
        }

        rateLimiter := ratelimit.New(30, 60) // 30 req/sec, burst 60
        handler := withSessionCookie(srv, withSecurityHeaders(withCORS(port, withLogging(withRateLimit(mux, rateLimiter)))))

        // Auto-launch the app window in a Chromium app-mode window.
        if getenv("ENTROPY_NO_LAUNCH", "") == "" {
                go launchAppWindow("http://127.0.0.1:" + port)
        }

        // FIX #5: graceful shutdown via SIGINT/SIGTERM or UI quit button
        ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
        cancelSignal = stop // allow exit.go to unblock main via triggerGracefulShutdown
        defer stop()

        // Bind to loopback only — not reachable from LAN or internet.
        addr := "127.0.0.1:" + port
        log.Printf("entropy-gui backend listening on http://%s | audio=%s | video=%s | workers=%d | state=%s | restored=%d",
                addr, audioDir, videoDir, workers, statePath, len(restored))

        server := &http.Server{Addr: addr, Handler: handler}

        // Listen in a goroutine so we can handle shutdown signals
        go func() {
                if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                        log.Fatalf("listen: %v", err)
                }
        }()

        // Block until signal received
        <-ctx.Done()
        log.Println("[entropy] shutdown signal received, draining...")

        // Stop accepting new jobs and wait for workers to finish current jobs.
        mgr.Close()

        // Give active requests 5 seconds to finish
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := server.Shutdown(shutdownCtx); err != nil {
                log.Printf("shutdown error: %v", err)
        }
        log.Println("[entropy] server stopped")
}

// --- handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusOK, map[string]any{
                "status":  "ok",
                "service": "entropy-gui",
                "time":    time.Now().UTC().Format(time.RFC3339),
        })
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusOK, map[string]any{
                "audio_dir":  s.mgr.AudioDir(),
                "video_dir":  s.mgr.VideoDir(),
                "formats":     []string{"mp3", "m4a", "flac", "opus", "wav", "mp4", "webm", "mkv", "best"},
                "bitrates":    []string{"96", "128", "192", "256", "320"},
                "resolutions": []string{"BEST", "4K", "1440p", "1080p", "720p", "480p"},
                "engines":     []string{"ytdlp", "aria2c"},
                "cookie_browsers": []string{
                        "none", "chrome", "edge", "firefox", "brave",
                        "chromium", "opera", "vivaldi", "safari",
                },
                "default_bandwidth": s.mgr.Bandwidth(), // current global default (5M = ~5 MB/s)
        })
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
                writeJSON(w, http.StatusOK, map[string]any{
                        "audio_dir":      s.mgr.AudioDir(),
                        "video_dir":      s.mgr.VideoDir(),
                        "bandwidth_limit": s.mgr.Bandwidth(),
                        "smart_routing":  s.mgr.SmartRouting(),
                })
        case http.MethodPost:
                var req struct {
                        AudioDir string `json:"audio_dir"`
                        VideoDir string `json:"video_dir"`
                }
                r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes) // FIX #2
                if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                        http.Error(w, "invalid body", http.StatusBadRequest)
                        return
                }
                req.AudioDir = strings.TrimSpace(req.AudioDir)
                req.VideoDir = strings.TrimSpace(req.VideoDir)
                if req.AudioDir == "" || req.VideoDir == "" {
                        writeJSON(w, http.StatusBadRequest, map[string]any{"error": "audio_dir and video_dir required"})
                        return
                }
                
                checkDir := func(dir string) (string, error) {
                        if strings.HasPrefix(dir, "~") {
                                if home, err := os.UserHomeDir(); err == nil {
                                        dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
                                }
                        }
                        abs, err := filepath.Abs(dir)
                        if err != nil {
                                return "", err
                        }
                        if isProtectedPath(abs) {
                                return "", fmt.Errorf("path %q is a protected system path", abs)
                        }
                        if err := os.MkdirAll(abs, 0o755); err != nil {
                                return "", fmt.Errorf("cannot create dir: %v", err)
                        }
                        probe := filepath.Join(abs, ".entropy-write-probe")
                        if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
                                return "", fmt.Errorf("directory not writable")
                        }
                        _ = os.Remove(probe)
                        return abs, nil
                }

                absAudio, err := checkDir(req.AudioDir)
                if err != nil {
                        writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
                        return
                }
                absVideo, err := checkDir(req.VideoDir)
                if err != nil {
                        writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
                        return
                }

                s.mgr.SetAudioDir(absAudio)
                s.mgr.SetVideoDir(absVideo)
                _ = s.store.SaveSettings(store.Settings{
                        AudioDir:       absAudio,
                        VideoDir:       absVideo,
                        MaxWorkers:     s.mgr.Workers(),
                        BandwidthLimit: s.mgr.Bandwidth(),
                        SmartRouting:   s.mgr.SmartRouting(),
                })
                writeJSON(w, http.StatusOK, map[string]any{"audio_dir": absAudio, "video_dir": absVideo})
        default:
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        var req struct {
                Source string `json:"source"`
                Query  string `json:"query"`
                Limit  int    `json:"limit"`
        }
        r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes) // FIX #2
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                http.Error(w, "invalid body", http.StatusBadRequest)
                return
        }
        if strings.TrimSpace(req.Query) == "" {
                http.Error(w, "query required", http.StatusBadRequest)
                return
        }
        // FIX: reject queries that look like URLs (SSRF mitigation)
        if looksLikeURL(req.Query) {
                writeJSON(w, http.StatusBadRequest, map[string]any{"error": "search query must be text, not a URL. Use the URL input to add links directly."})
                return
        }
        if req.Limit <= 0 {
                req.Limit = 15
        }
        ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
        defer cancel()
        results, err := ytdlp.Search(ctx, s.ytdlpBin, req.Source, req.Query, req.Limit)
        if err != nil {
                errMsg := err.Error()
                if len(errMsg) > 200 {
                        errMsg = errMsg[:200] + "..."
                }
                writeJSON(w, http.StatusBadGateway, map[string]any{"error": errMsg})
                return
        }
        writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleCleanURL(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        var req struct {
                URL  string `json:"url"`
                Text string `json:"text"`
        }
        r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes) // FIX #2
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                http.Error(w, "invalid body", http.StatusBadRequest)
                return
        }
        if req.Text != "" {
                writeJSON(w, http.StatusOK, map[string]any{"urls": cleaner.CleanLines(req.Text)})
                return
        }
        writeJSON(w, http.StatusOK, map[string]any{"url": cleaner.CleanURL(req.URL)})
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
                writeJSON(w, http.StatusOK, map[string]any{"jobs": s.mgr.List()})
        case http.MethodPost:
                s.createJobs(w, r)
        default:
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
}

func (s *Server) createJobs(w http.ResponseWriter, r *http.Request) {
        var req struct {
                URLs    []string          `json:"urls"`
                Text    string            `json:"text"`
                Items   []jobs.DirectItem `json:"items"`
                Options jobs.Options      `json:"options"`
        }
        r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes) // FIX #2
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                http.Error(w, "invalid body", http.StatusBadRequest)
                return
        }
        if len(req.URLs)+len(req.Items) > maxURLsPerRequest {
                writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": fmt.Sprintf("too many urls/items (max %d per request)", maxURLsPerRequest)})
                return
        }
        if req.Text != "" {
                req.URLs = append(req.URLs, cleaner.CleanLines(req.Text)...)
        }
        var created []*jobs.Job
        if len(req.Items) > 0 {
                created = append(created, s.mgr.AddDirect(req.Items, req.Options)...)
        }
	if len(req.URLs) > 0 {
		// Budget enough time to probe every URL. Each playlist/album probe
		// can take a while on slowly-paginating sources (e.g. a 65-track
		// SoundCloud set), so we scale the deadline with the batch size:
		// ~120s per URL, clamped to a sane window. A single fixed 60s cap
		// previously starved multi-playlist batches and truncated results.
		timeout := time.Duration(len(req.URLs)*120) * time.Second
		if timeout < 60*time.Second {
			timeout = 60 * time.Second
		}
		if timeout > 600*time.Second {
			timeout = 600 * time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		more, err := s.mgr.AddURLs(ctx, req.URLs, req.Options)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		created = append(created, more...)
	}
        if len(created) == 0 {
                writeJSON(w, http.StatusBadRequest, map[string]any{"error": "no valid urls/items"})
                return
        }
        writeJSON(w, http.StatusCreated, map[string]any{"jobs": created})
}

func openInFileBrowser(filePath string) error {
        // Security: verify the file exists and is a regular file (not a pipe/socket/symlink trick)
        info, err := os.Stat(filePath)
        if err != nil || info.IsDir() {
                return fmt.Errorf("file not found or not a regular file")
        }
        // Reject paths with shell metacharacters that could cause argument injection.
        // Note: backslash is NOT rejected here — it's a valid path separator on Windows.
        // The os/exec package handles argument escaping safely on all platforms.
        if strings.ContainsAny(filePath, "\"'`\n\r\t") {
                return fmt.Errorf("file path contains unsafe characters")
        }
        switch runtime.GOOS {
        case "windows":
                return exec.Command("explorer", "/select,", filePath).Start()
        case "darwin":
                return exec.Command("open", "-R", filePath).Start()
        default:
                return exec.Command("xdg-open", filepath.Dir(filePath)).Start()
        }
}

func (s *Server) handleJobByID(w http.ResponseWriter, r *http.Request) {
        // Path: /api/jobs/<id> or /api/jobs/<id>/retry or /api/jobs/<id>/open-folder
        path := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
        if path == "" || path == "clear" || path == "stream" {
                http.NotFound(w, r)
                return
        }
        parts := strings.Split(path, "/")
        id := parts[0]
        if len(parts) > 1 {
                switch parts[1] {
                case "retry":
                        if r.Method != http.MethodPost {
                                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                                return
                        }
                        if s.mgr.Retry(id) {
                                writeJSON(w, http.StatusOK, map[string]any{"ok": true})
                                return
                        }
                        http.Error(w, "cannot retry", http.StatusBadRequest)
                        return
                case "open-folder":
                        if r.Method != http.MethodPost {
                                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                                return
                        }
                        job, ok := s.mgr.Get(id)
                        if !ok || job.OutputFile == "" {
                                http.Error(w, "job not found or no output file", http.StatusNotFound)
                                return
                        }
                        if err := openInFileBrowser(job.OutputFile); err != nil {
                                http.Error(w, err.Error(), http.StatusInternalServerError)
                                return
                        }
                        writeJSON(w, http.StatusOK, map[string]any{"ok": true})
                        return
                }
        }
        if r.Method == http.MethodDelete {
                if s.mgr.Remove(id) {
                        writeJSON(w, http.StatusOK, map[string]any{"ok": true})
                        return
                }
                http.NotFound(w, r)
                return
        }
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleJobsClear(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        var req struct {
                What string `json:"what"` // completed | failed
        }
        r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes) // FIX #2
        _ = json.NewDecoder(r.Body).Decode(&req)
        if req.What != "completed" && req.What != "failed" {
                writeJSON(w, http.StatusBadRequest, map[string]any{"error": "what must be 'completed' or 'failed'"})
                return
        }
        removed := s.mgr.Clear(req.What)
        writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
        limit := 200
        if v := r.URL.Query().Get("limit"); v != "" {
                if n, err := strconv.Atoi(v); err == nil {
                        limit = n
                }
        }
        // Cap at 2000 to prevent excessive response sizes
        if limit > 2000 {
                limit = 2000
        }
        writeJSON(w, http.StatusOK, map[string]any{"logs": s.mgr.RecentLogs(limit)})
}

// handleEnv probes the system environment, returns OS, distro, browsers, pkg managers
// and the status of external binaries.
func (s *Server) handleEnv(w http.ResponseWriter, r *http.Request) {
        probe := func(envVar, name, versionFlag string) map[string]any {
                path := resolveTool(envVar, name)
                ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
                defer cancel()
                cmd := exec.CommandContext(ctx, path, versionFlag)
                cmdutil.PrepareCmd(cmd)
                out, err := cmd.CombinedOutput()
                if err != nil && len(out) == 0 {
                        return map[string]any{
                                "name":  name,
                                "found": false,
                                "path":  path,
                                "version": "",
                                "error":   err.Error(),
                        }
                }
                ver := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
                return map[string]any{
                        "name":  name,
                        "found": true,
                        "path":  path,
                        "version": ver,
                }
        }

        // Detect OS/Distro
        distro := "unknown"
        if runtime.GOOS == "linux" {
                if b, err := os.ReadFile("/etc/os-release"); err == nil {
                        for _, line := range strings.Split(string(b), "\n") {
                                if strings.HasPrefix(line, "ID=") {
                                        distro = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
                                        break
                                }
                        }
                }
        } else if runtime.GOOS == "windows" {
                distro = "windows"
        } else if runtime.GOOS == "darwin" {
                distro = "macos"
        }

        // Detect Package Managers
        pkgMgrs := []string{}
        for _, pm := range []string{"apt", "pacman", "dnf", "zypper", "brew", "winget", "choco"} {
                if _, err := exec.LookPath(pm); err == nil {
                        pkgMgrs = append(pkgMgrs, pm)
                }
        }

        // Load Settings to check onboarding status
        settings, _ := s.store.LoadSettings()

        // Disk space info for the env endpoint
        freeDiskGB, _ := diskguard.FreeSpaceGB(s.mgr.VideoDir())

        writeJSON(w, http.StatusOK, map[string]any{
                "platform":        runtime.GOOS,
                "distro":          distro,
                "pkg_mgrs":        pkgMgrs,
                "onboarding_done": settings.OnboardingDone,
                "free_disk_gb":    freeDiskGB,
                "tools": map[string]any{
                        "yt_dlp": probe("YTDLP_BIN", "yt-dlp", "--version"),
                        "aria2c": probe("ARIA2C_BIN", "aria2c", "--version"),
                        "ffmpeg": probe("FFMPEG_BIN", "ffmpeg", "-version"),
                },
        })
}

// handleSetOnboarding marks the onboarding flow as completed
func (s *Server) handleSetOnboarding(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        settings, err := s.store.LoadSettings()
        if err != nil {
                writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to load settings"})
                return
        }
        settings.OnboardingDone = true
        if err := s.store.SaveSettings(settings); err != nil {
                writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to save settings"})
                return
        }
        writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleToolUpdate triggers an update for a specific tool. Currently only yt-dlp is supported.
func (s *Server) handleToolUpdate(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        // Rate-limit: one update per 60 seconds
        s.toolUpdateMu.Lock()
        if time.Since(s.lastToolUpdate) < 60*time.Second {
                s.toolUpdateMu.Unlock()
                writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "tool update already ran recently, wait 60s"})
                return
        }
        s.lastToolUpdate = time.Now()
        s.toolUpdateMu.Unlock()

        // Right now we only update yt-dlp
        path := resolveTool("YTDLP_BIN", "yt-dlp")
        ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
        defer cancel()

        cmd := exec.CommandContext(ctx, path, "-U")
        cmdutil.PrepareCmd(cmd)
        out, err := cmd.CombinedOutput()

        if err != nil && len(out) == 0 {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }

        writeJSON(w, http.StatusOK, map[string]any{
                "output": string(out),
        })
}

// handleConcurrency updates the max simultaneous download workers at runtime.
func (s *Server) handleConcurrency(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet {
                writeJSON(w, http.StatusOK, map[string]any{"workers": s.mgr.Workers()})
                return
        }
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        var req struct {
                Workers int `json:"workers"`
        }
        r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes) // FIX #2
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Workers < 1 {
                http.Error(w, "invalid body: workers must be >= 1", http.StatusBadRequest)
                return
        }
        if req.Workers > 32 {
                req.Workers = 32
        }
        s.mgr.SetWorkers(req.Workers)
        _ = s.store.SaveSettings(store.Settings{
                AudioDir:       s.mgr.AudioDir(),
                VideoDir:       s.mgr.VideoDir(),
                MaxWorkers:     req.Workers,
                BandwidthLimit: s.mgr.Bandwidth(),
                SmartRouting:   s.mgr.SmartRouting(),
        })
        writeJSON(w, http.StatusOK, map[string]any{"workers": req.Workers})
}

// handleBandwidth gets or sets the global default bandwidth limit.
// GET /api/bandwidth — returns current limit (e.g., "5M", "10M", "0")
// POST /api/bandwidth — updates limit. Accepts {"limit": "10M"} or {"limit": "0"} for unlimited.
func (s *Server) handleBandwidth(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet {
                writeJSON(w, http.StatusOK, map[string]any{
                        "bandwidth_limit": s.mgr.Bandwidth(),
                })
                return
        }
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        var req struct {
                Limit string `json:"limit"`
        }
        r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                http.Error(w, "invalid body", http.StatusBadRequest)
                return
        }
        req.Limit = strings.TrimSpace(req.Limit)
        // Validate format: number followed by optional K/M/G suffix
        if !isValidBandwidth(req.Limit) {
                writeJSON(w, http.StatusBadRequest, map[string]any{
                        "error": "invalid bandwidth format. Use a number with optional suffix: 5M, 10M, 1G, 500K, or 0 for unlimited",
                })
                return
        }
        s.mgr.SetBandwidth(req.Limit)
        _ = s.store.SaveSettings(store.Settings{
                AudioDir:       s.mgr.AudioDir(),
                VideoDir:       s.mgr.VideoDir(),
                MaxWorkers:     s.mgr.Workers(),
                BandwidthLimit: req.Limit,
                SmartRouting:   s.mgr.SmartRouting(),
        })
        writeJSON(w, http.StatusOK, map[string]any{"bandwidth_limit": req.Limit})
}

// POST /api/smart-routing
func (s *Server) handleSmartRouting(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        var req struct {
                Enabled bool `json:"enabled"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                http.Error(w, "invalid body", http.StatusBadRequest)
                return
        }
        s.mgr.SetSmartRouting(req.Enabled)
        _ = s.store.SaveSettings(store.Settings{
                AudioDir:       s.mgr.AudioDir(),
                VideoDir:       s.mgr.VideoDir(),
                MaxWorkers:     s.mgr.Workers(),
                BandwidthLimit: s.mgr.Bandwidth(),
                SmartRouting:   req.Enabled,
        })
        writeJSON(w, http.StatusOK, map[string]any{"smart_routing": req.Enabled})
}

// isValidBandwidth checks if a bandwidth limit string is valid for yt-dlp --limit-rate.
// Valid formats: "0", "500K", "5M", "1G", "10.5M", etc.
func isValidBandwidth(s string) bool {
        if s == "" {
                return false
        }
        s = strings.ToUpper(s)
        if s == "0" {
                return true
        }
        // Must end with exactly one of K, M, or G
        switch {
        case strings.HasSuffix(s, "K"):
                s = strings.TrimSuffix(s, "K")
        case strings.HasSuffix(s, "M"):
                s = strings.TrimSuffix(s, "M")
        case strings.HasSuffix(s, "G"):
                s = strings.TrimSuffix(s, "G")
        default:
                return false
        }
        _, err := strconv.ParseFloat(s, 64)
        return err == nil
}

// handleShutdown gracefully exits the process. Triggered by the UI quit button.
// FIX #1: requires the entropy_session cookie to match, preventing CSRF from
// other origins (SameSite=Lax + HttpOnly means cross-site pages can't send it).
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }

        // Verify the session cookie matches our expected token.
        cookie, err := r.Cookie(sessionCookieName)
        if err != nil || cookie.Value != s.shutdownToken {
                http.Error(w, "forbidden: missing or invalid session cookie", http.StatusForbidden)
                return
        }

        writeJSON(w, http.StatusOK, map[string]any{"ok": true})
        shutdownAfter(300 * time.Millisecond)
}

// handleImport accepts a text/csv/m3u file upload and extracts URLs for bulk download.
// POST /api/import — multipart form with file field named "file".
// Returns parsed URLs, duplicate count, and warnings.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        // Limit upload size to 50MB.
        r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

        file, header, err := r.FormFile("file")
        if err != nil {
                writeJSON(w, http.StatusBadRequest, map[string]any{"error": "no file uploaded (field name: 'file')"})
                return
        }
        defer file.Close()

        // Check extension
        allowedExts := map[string]bool{
                ".txt": true, ".csv": true, ".m3u": true, ".m3u8": true,
                ".url": true, "": true, // allow no extension (plain text)
        }
        ext := strings.ToLower(filepath.Ext(header.Filename))
        if !allowedExts[ext] {
                writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("unsupported file type: %s (allowed: .txt, .csv, .m3u, .m3u8)", ext)})
                return
        }

        result, err := importer.Parse(file, header.Filename)
        if err != nil {
                writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
                return
        }

        // Deduplicate within the file itself
        deduped, dupesInFile := importer.Deduplicate(result.URLs)
        result.URLs = deduped

        // Count how many are already done (dedup against existing jobs)
        alreadyDone := s.mgr.DuplicateCount(result.URLs)

        // Filter out already-done URLs from the result
        var freshURLs []string
        for _, u := range result.URLs {
                if !s.mgr.IsDuplicateURL(u) {
                        freshURLs = append(freshURLs, u)
                }
        }

        writeJSON(w, http.StatusOK, map[string]any{
                "filename":         header.Filename,
                "total_urls":       len(deduped),
                "fresh_urls":       len(freshURLs),
                "duplicates_in_file": dupesInFile,
                "already_downloaded": alreadyDone,
                "warnings":         result.Warnings,
                "urls":              freshURLs,
        })
}

// handleRetryFailed re-queues all failed and canceled jobs in one click.
// POST /api/jobs/retry-failed
func (s *Server) handleRetryFailed(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        retried := s.mgr.RetryAllFailed()
        writeJSON(w, http.StatusOK, map[string]any{"retried": retried})
}

// handleStats returns aggregate job statistics and system info for the dashboard.
// GET /api/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        stats := s.mgr.Stats()
        // Enrich with disk space info
        freeGB, err := diskguard.FreeSpaceGB(s.mgr.VideoDir())
        if err == nil {
                stats.FreeDiskGB = freeGB
        }
        writeJSON(w, http.StatusOK, stats)
}

// handleTheme returns the OS accent/highlight color as JSON.
// GET /api/theme → {"seed":"#RRGGBB","platform":"linux","source":"gsettings"}
func (s *Server) handleTheme(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        seed := themereader.GetSystemAccentColor()
        source := themereader.GetSystemAccentSource()
        writeJSON(w, http.StatusOK, map[string]any{
                "seed":     seed,
                "platform": runtime.GOOS,
                "source":   source,
        })
}

// embeddedSPAHandler serves the embedded React build with SPA fallback.
func embeddedSPAHandler(efs fs.FS) http.Handler {
        fileServer := http.FileServer(http.FS(efs))
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                clean := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
                if clean == "" || clean == "." {
                        serveEmbeddedIndex(w, efs)
                        return
                }
                if f, err := efs.Open(clean); err == nil {
                        info, _ := f.Stat()
                        f.Close()
                        if info != nil && !info.IsDir() {
                                fileServer.ServeHTTP(w, r)
                                return
                        }
                }
                serveEmbeddedIndex(w, efs)
        })
}

func serveEmbeddedIndex(w http.ResponseWriter, efs fs.FS) {
        data, err := fs.ReadFile(efs, "index.html")
        if err != nil {
                http.Error(w, "index missing", http.StatusInternalServerError)
                return
        }
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write(data)
}

// FIX #7: SSE stream now checks subscription limit.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
        flusher, ok := w.(http.Flusher)
        if !ok {
                http.Error(w, "streaming unsupported", http.StatusInternalServerError)
                return
        }

        // Cap concurrent SSE subscriptions to prevent memory exhaustion.
        subID, ch, err := s.mgr.Subscribe()
        if err != nil {
                http.Error(w, err.Error(), http.StatusServiceUnavailable)
                return
        }
        defer s.mgr.Unsubscribe(subID)

        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")
        w.Header().Set("X-Accel-Buffering", "no")

        // Send initial snapshot
        snap := jobs.Event{Type: "snapshot", Jobs: s.mgr.List()}
        writeSSE(w, snap)
        flusher.Flush()

        keepalive := time.NewTicker(15 * time.Second)
        defer keepalive.Stop()

        // Track last event time for idle timeout
        idleTimeout := 5 * time.Minute
        lastEvent := time.Now()

        notify := r.Context().Done()
        for {
                select {
                case <-notify:
                        return
                case ev, open := <-ch:
                        if !open {
                                return
                        }
                        lastEvent = time.Now()
                        writeSSE(w, ev)
                        flusher.Flush()
                case <-keepalive.C:
                        if time.Since(lastEvent) > idleTimeout {
                                log.Printf("[entropy] SSE idle timeout after %v", idleTimeout)
                                return
                        }
                        fmt.Fprintf(w, ": ping\n\n")
                        flusher.Flush()
                }
        }
}

func writeSSE(w http.ResponseWriter, ev jobs.Event) {
        b, err := json.Marshal(ev)
        if err != nil {
                return
        }
        fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, string(b))
}

// --- middleware/utils ---

func writeJSON(w http.ResponseWriter, code int, v any) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(code)
        _ = json.NewEncoder(w).Encode(v)
}

// withSessionCookie sets an HttpOnly, SameSite=Lax cookie on every response.
// This cookie is required by /api/shutdown for CSRF protection.
func withSessionCookie(srv *Server, h http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                // Set the cookie on every response so the browser always has it.
                http.SetCookie(w, &http.Cookie{
                        Name:     sessionCookieName,
                        Value:    srv.shutdownToken,
                        Path:     "/",
                        MaxAge:   86400, // 24h; refreshed on every request
                        HttpOnly: true,
                        Secure:   srv.secureCookie,
                        SameSite: http.SameSiteLaxMode,
                })
                h.ServeHTTP(w, r)
        })
}

func withCORS(port string, h http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                // Only allow requests from the exact localhost origin on our port.
                origin := r.Header.Get("Origin")
                allowed1 := "http://127.0.0.1:" + port
                allowed2 := "http://localhost:" + port
                if origin == allowed1 || origin == allowed2 {
                        w.Header().Set("Access-Control-Allow-Origin", origin)
                        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
                        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
                }
                if r.Method == http.MethodOptions {
                        w.WriteHeader(http.StatusNoContent)
                        return
                }
                h.ServeHTTP(w, r)
        })
}

func withLogging(h http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                start := time.Now()
                h.ServeHTTP(w, r)
                // Sanitize path to prevent log injection
                safePath := strings.Map(func(r rune) rune {
                        if r == '\n' || r == '\r' || r == '\t' {
                                return ' '
                        }
                        return r
                }, r.URL.Path)
                log.Printf("%s %s %s", r.Method, safePath, time.Since(start))
        })
}

// withSecurityHeaders adds Content-Security-Policy and other security headers.
// Since this is a localhost desktop app, the CSP is relaxed enough to not break
// the embedded React app while still providing baseline XSS protection.
func withSecurityHeaders(h http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; img-src 'self' data: https:; connect-src 'self'; font-src 'self' data: https://fonts.gstatic.com")
                w.Header().Set("X-Content-Type-Options", "nosniff")
                w.Header().Set("X-Frame-Options", "DENY")
                w.Header().Set("Referrer-Policy", "no-referrer")
                w.Header().Set("X-XSS-Protection", "1; mode=block")
                h.ServeHTTP(w, r)
        })
}

// withRateLimit applies token-bucket rate limiting to all requests except SSE and health.
func withRateLimit(h http.Handler, limiter *ratelimit.Limiter) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                // SSE and health endpoints are exempt
                if r.URL.Path == "/api/jobs/stream" || r.URL.Path == "/api/health" {
                        h.ServeHTTP(w, r)
                        return
                }
                if !limiter.Allow() {
                        w.Header().Set("Content-Type", "application/json")
                        w.Header().Set("Retry-After", "1")
                        w.WriteHeader(http.StatusTooManyRequests)
                        json.NewEncoder(w).Encode(map[string]any{"error": "rate limit exceeded"})
                        return
                }
                h.ServeHTTP(w, r)
        })
}

// isProtectedPath returns true for system directories that should never
// be used as download output directories.
func isProtectedPath(p string) bool {
        // Resolve .. components first, then normalize to forward slashes for comparison
        normalized := filepath.ToSlash(filepath.Clean(p))
        protected := []string{
                "/", "/proc", "/sys", "/dev", "/boot", "/etc", "/sbin", "/bin",
                "/usr/bin", "/usr/sbin", "/usr/lib", "/var", "/tmp",
                "/windows", "/program files",
        }
        for _, pp := range protected {
                if normalized == pp {
                        return true
                }
                // Block any path under a protected directory (e.g., /etc/anything)
                if strings.HasPrefix(normalized, pp+"/") {
                        return true
                }
        }
        return false
}

// looksLikeURL returns true if the string appears to be a URL rather than a search query.
// This prevents the search endpoint from being abused as an SSRF vector.
func looksLikeURL(s string) bool {
        s = strings.TrimSpace(strings.ToLower(s))
        return strings.HasPrefix(s, "http://") ||
                strings.HasPrefix(s, "https://") ||
                strings.HasPrefix(s, "ftp://") ||
                strings.HasPrefix(s, "ftps://")
}

func getenv(k, def string) string {
        if v := os.Getenv(k); v != "" {
                return v
        }
        return def
}

// resolveTool finds an external CLI. Priority:
//  1. $ENV_VAR if set
//  2. ./tools/<name>(.exe) beside the binary (portable bundle)
//  3. PATH lookup
//  4. fallback to the bare name (will fail later with a clear error)
func resolveTool(envVar, name string) string {
        if v := os.Getenv(envVar); v != "" {
                return v
        }
        exe := name
        if runtime.GOOS == "windows" {
                exe = name + ".exe"
        }
        if bin, err := os.Executable(); err == nil {
                candidate := filepath.Join(filepath.Dir(bin), "tools", exe)
                if _, err := os.Stat(candidate); err == nil {
                        return candidate
                }
        }
        if p, err := exec.LookPath(exe); err == nil {
                return p
        }
        return name
}

// resolveWebDir locates the frontend build. Priority:
//  1. ./web/ beside the binary
//  2. ./frontend/build/ (dev convenience)
//  3. "" (none — serves a stub welcome page)
func resolveWebDir() string {
        candidates := []string{}
        if bin, err := os.Executable(); err == nil {
                base := filepath.Dir(bin)
                candidates = append(candidates,
                        filepath.Join(base, "web"),
                        filepath.Join(base, "frontend", "build"),
                )
        }
        candidates = append(candidates, "./web", "./frontend/build", "../frontend/build")
        for _, c := range candidates {
                if fi, err := os.Stat(c); err == nil && fi.IsDir() {
                        if _, err := os.Stat(filepath.Join(c, "index.html")); err == nil {
                                return c
                        }
                }
        }
        return ""
}

// spaHandler serves the React SPA: any non-file route returns index.html
// so client-side routing works.
func spaHandler(dir string, fs http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                path := filepath.Join(dir, filepath.Clean(r.URL.Path))
                if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
                        fs.ServeHTTP(w, r)
                        return
                }
                http.ServeFile(w, r, filepath.Join(dir, "index.html"))
        })
}

// defaultDownloadDir picks a sensible default per OS.
func defaultDownloadDir() string {
        home, err := os.UserHomeDir()
        if err != nil {
                return "downloads"
        }
        return filepath.Join(home, "Downloads", "Entropy")
}

// loadEnvFile loads KEY=VALUE pairs from a .env file into the environment.
// FIX: reads the full file (up to 64KB) instead of truncating at 4096 bytes.
func loadEnvFile(path string) {
        f, err := os.Open(path)
        if err != nil {
                return
        }
        defer f.Close()
        // Read up to 64KB — more than enough for any reasonable .env file.
        data, err := io.ReadAll(io.LimitReader(f, 65536))
        if err != nil {
                return
        }
        for _, line := range strings.Split(string(data), "\n") {
                line = strings.TrimSpace(line)
                if line == "" || strings.HasPrefix(line, "#") {
                        continue
                }
                eq := strings.Index(line, "=")
                if eq <= 0 {
                        continue
                }
                k := strings.TrimSpace(line[:eq])
                v := strings.TrimSpace(line[eq+1:])
                v = strings.Trim(v, `"'`)
                if os.Getenv(k) == "" {
                        _ = os.Setenv(k, v)
                }
        }
}

// resolveStatePath determines the most suitable writable path for storing state.json.
func resolveStatePath(envVar, defaultPath string) string {
        if v := os.Getenv(envVar); v != "" {
                return v
        }

        // Try defaultPath
        dir := filepath.Dir(defaultPath)
        if err := os.MkdirAll(dir, 0o755); err == nil {
                probe := filepath.Join(dir, ".state-write-probe")
                if err := os.WriteFile(probe, []byte("ok"), 0o644); err == nil {
                        _ = os.Remove(probe)
                        return defaultPath
                }
        }

        // Fallback 1: beside the binary
        if bin, err := os.Executable(); err == nil {
                dir := filepath.Dir(bin)
                probe := filepath.Join(dir, ".state-write-probe")
                if err := os.WriteFile(probe, []byte("ok"), 0o644); err == nil {
                        _ = os.Remove(probe)
                        return filepath.Join(dir, "entropy.db")
                }
        }

        // Fallback 2: user home config directory
        if home, err := os.UserHomeDir(); err == nil {
                dir := filepath.Join(home, ".config", "entropy-gui")
                if err := os.MkdirAll(dir, 0o755); err == nil {
                        return filepath.Join(dir, "entropy.db")
                }
        }

        // Fallback 3: current directory
        return "entropy.db"
}
