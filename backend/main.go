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

	"entropy-gui/pkg/auth"
	"entropy-gui/pkg/cleaner"
	"entropy-gui/pkg/cmdutil"
	"entropy-gui/pkg/diskguard"
	"entropy-gui/pkg/importer"
	"entropy-gui/pkg/jobs"
	"entropy-gui/pkg/library"
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
	authStore       *auth.Store
	shutdownToken   string // CSRF token; set as HttpOnly cookie
	secureCookie    bool   // true if server runs over HTTPS
	lastToolUpdate  time.Time // rate-limit tool updates
	toolUpdateMu    sync.Mutex
	loopbackMode    bool   // true when serving on loopback (no auth needed)
}

func main() {
        loadEnvFile("/app/backend/.env")
        loadEnvFile(".env") // also try .env beside the binary (for portable builds)

	port := getenv("PORT", "8001")
	host := resolveBindHost()
	if err := validateBindConfig(host); err != nil {
		log.Fatalf("[entropy] unsafe bind configuration: %v", err)
	}
	tlsOn := useHTTPS() && !isLoopbackHost(host)
	scheme := "http"
	if tlsOn {
		scheme = "https"
	}
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

        // --- Auth setup ---
        // Auth is only active when serving over a non-loopback address (the
        // homelab/network case). When loopback, the loopback-passthrough flag
        // makes every handler behave as if a single admin is already authenticated.
        loopbackMode := isLoopbackHost(host)

        var authStore *auth.Store
        if loopbackMode {
                log.Println("[entropy] loopback mode — auth disabled, full access granted")
        } else {
                as, err := auth.New(st.DB())
                if err != nil {
                        log.Fatalf("auth: %v", err)
                }
                authStore = as

                // Bootstrap: if ADMIN_PASSWORD is set and no admin user exists yet,
                // create the first admin. This is the smoothest onboarding path:
                // the homelab operator sets HOST + USE_HTTPS + ADMIN_PASSWORD in .env
                // and the first start "just works".
                if pw := strings.TrimSpace(os.Getenv(adminPasswordEnv)); pw != "" {
                        hasAdmin, _ := as.HasAdmin()
                        if !hasAdmin {
                                if _, err := as.Create("admin", pw, true); err != nil {
                                        log.Fatalf("auth: bootstrap admin user: %v", err)
                                }
                                log.Println("[entropy] auth: created initial admin user from ADMIN_PASSWORD env")
                        }
                }

                hasAny, _ := as.HasAnyUser()
                if !hasAny {
                        log.Println("[entropy] auth: no users configured — setup mode active. POST /api/setup to create the first admin.")
                }
        }

        // FIX #1: generate a unique session token for CSRF protection on shutdown.
        // This token is set as an HttpOnly cookie; SameSite=Lax prevents
        // cross-origin requests from including it.
        shutdownToken := uuid.NewString()
        // Mark session cookies Secure only when we're actually serving over
        // TLS. tlsOn already encodes "non-loopback + USE_HTTPS=1", which is the
        // one configuration where a Secure cookie survives (browsers drop
        // Secure cookies on plain-HTTP responses).
        secureCookie := tlsOn

        srv := &Server{mgr: mgr, ytdlpBin: ytdlpBin, store: st, authStore: authStore, shutdownToken: shutdownToken, secureCookie: secureCookie, loopbackMode: loopbackMode}

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

	// Library — browse and stream downloaded files.
	mux.HandleFunc("/api/library", srv.handleLibrary)
	mux.HandleFunc("/api/library/dir", srv.handleLibraryDir)
	mux.HandleFunc("/api/library/file", srv.handleLibraryFile)

        // Auth endpoints — always reachable (no auth middleware on these).
        mux.HandleFunc("/api/setup", srv.handleSetup)
        mux.HandleFunc("/api/login", srv.handleLogin)
        mux.HandleFunc("/api/logout", srv.handleLogout)
        mux.HandleFunc("/api/me", srv.handleMe)
        mux.HandleFunc("/api/users", srv.handleUsers)

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

        // Build the middleware chain. Auth sits between rate-limiting and the
        // existing security headers. On loopback, auth is a no-op passthrough.
        // On network mode, it enforces sessions and admin-only on power ops.
        var handler http.Handler = mux
        if !loopbackMode && authStore != nil {
                handler = withAuthAndAdminRoles(authStore, sessionCookieName, handler)
        }
        handler = withSessionCookie(srv, withSecurityHeaders(withCORS(port, scheme, withLogging(withRateLimit(handler, rateLimiter)))))

        // Auto-launch the app window in a Chromium app-mode window.
        // Only meaningful for the local desktop experience: if the server is
        // bound to a non-loopback address it's almost certainly running
        // headless on a homelab box, where opening a local browser window is
        // both useless and surprising. ENTROPY_NO_LAUNCH overrides either way.
	if getenv("ENTROPY_NO_LAUNCH", "") == "" && isLoopbackHost(host) {
		go launchAppWindow(scheme + "://" + host + ":" + port)
	}

        // FIX #5: graceful shutdown via SIGINT/SIGTERM or UI quit button
        ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
        cancelSignal = stop // allow exit.go to unblock main via triggerGracefulShutdown
        defer stop()

        // Bind address. Loopback (default) keeps the app unreachable from the
        // LAN/internet. A non-loopback HOST is permitted only because the
        // startup guard (validateBindConfig) already verified TLS+auth are on.
        addr := host + ":" + port
        log.Printf("entropy-gui backend listening on %s://%s | audio=%s | video=%s | workers=%d | state=%s | restored=%d",
                scheme, addr, audioDir, videoDir, workers, statePath, len(restored))

        server := &http.Server{Addr: addr, Handler: handler}

        // Listen in a goroutine so we can handle shutdown signals
        go func() {
                if tlsOn {
                        certFile, keyFile, err := ensureCerts()
                        if err != nil {
                                log.Fatalf("[entropy] %v", err)
                        }
                        if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
                                log.Fatalf("listen: %v", err)
                        }
                } else {
                        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                                log.Fatalf("listen: %v", err)
                        }
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
                writeJSON(w, http.StatusOK, map[string]any{"jobs": s.mgr.List(requestOwner(s, r))})
        case http.MethodPost:
                s.createJobs(w, r)
        default:
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
}

func (s *Server) createJobs(w http.ResponseWriter, r *http.Request) {
        owner := requestOwner(s, r)
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
                created = append(created, s.mgr.AddDirect(req.Items, req.Options, owner)...)
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
		more, err := s.mgr.AddURLs(ctx, req.URLs, req.Options, owner)
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
        owner := requestOwner(s, r)
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
                        if s.mgr.Retry(id, owner) {
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
                        job, ok := s.mgr.Get(id, owner)
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
                if s.mgr.Remove(id, owner) {
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
        removed := s.mgr.Clear(req.What, requestOwner(s, r))
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
        writeJSON(w, http.StatusOK, map[string]any{"logs": s.mgr.RecentLogs(limit, requestOwner(s, r))})
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

        owner := requestOwner(s, r)

        // Count how many are already done (dedup against existing jobs)
        alreadyDone := s.mgr.DuplicateCount(result.URLs, owner)

        // Filter out already-done URLs from the result
        var freshURLs []string
        for _, u := range result.URLs {
                if !s.mgr.IsDuplicateURL(u, owner) {
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
	retried := s.mgr.RetryAllFailed(requestOwner(s, r))
        writeJSON(w, http.StatusOK, map[string]any{"retried": retried})
}

// handleStats returns aggregate job statistics and system info for the dashboard.
// GET /api/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
	stats := s.mgr.Stats(requestOwner(s, r))
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

// --- Library endpoints ---

// allowedMediaExts is the set of file extensions the library will serve.
// Anything else is rejected by handleLibraryFile to prevent serving
// arbitrary files (e.g. the dead_links.csv or a stray .exe).
var allowedMediaExts = map[string]bool{
	// audio
	"mp3": true, "m4a": true, "flac": true, "wav": true, "opus": true,
	"aac": true, "ogg": true,
	// video
	"mp4": true, "mkv": true, "webm": true, "avi": true, "mov": true, "flv": true,
	// images (embedded thumbnails, cover art)
	"jpg": true, "jpeg": true, "png": true, "webp": true, "gif": true,
}

// rootDirFor maps the "root" query param ("audio"|"video") to the configured
// output directory. Returns "" and false if the root is invalid.
func (s *Server) rootDirFor(root string) (string, bool) {
	switch strings.ToLower(root) {
	case "audio":
		return s.mgr.AudioDir(), true
	case "video":
		return s.mgr.VideoDir(), true
	}
	return "", false
}

// handleLibrary lists the top-level contents of both the audio and video dirs.
// GET /api/library → {"audio": [...], "video": [...]}
func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	audioEntries, audioErr := library.ListDir(s.mgr.AudioDir(), "")
	videoEntries, videoErr := library.ListDir(s.mgr.VideoDir(), "")

	// If a root directory is missing/unreadable, return an empty list rather
	// than failing the whole request — one good root is still useful.
	resp := map[string]any{
		"audio":       audioEntries,
		"video":       videoEntries,
		"audio_error": errString(audioErr),
		"video_error": errString(videoErr),
	}
	if audioEntries == nil {
		resp["audio"] = []library.Entry{}
	}
	if videoEntries == nil {
		resp["video"] = []library.Entry{}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleLibraryDir lists the contents of a subdirectory within one root.
// GET /api/library/dir?root=audio&path=Albums → {"entries": [...]}
func (s *Server) handleLibraryDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	root := r.URL.Query().Get("root")
	relPath := r.URL.Query().Get("path")

	rootDir, ok := s.rootDirFor(root)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid root (must be 'audio' or 'video')"})
		return
	}

	entries, err := library.ListDir(rootDir, relPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// handleLibraryFile serves a single media file with HTTP Range support.
// GET /api/library/file?root=audio&path=song.mp3
//
// Streaming uses http.ServeFile which handles Range requests (206 Partial
// Content) natively — clients can seek and the browser video player works.
func (s *Server) handleLibraryFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	root := r.URL.Query().Get("root")
	relPath := r.URL.Query().Get("path")

	rootDir, ok := s.rootDirFor(root)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid root (must be 'audio' or 'video')"})
		return
	}
	if relPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing path parameter"})
		return
	}

	// Reject file extensions we don't serve, even if ResolveFile would accept them.
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(relPath), "."))
	if !allowedMediaExts[ext] {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "file type not allowed"})
		return
	}

	absPath, err := library.ResolveFile(rootDir, relPath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}

	// Inline disposition so browsers play media inline rather than downloading.
	w.Header().Set("Content-Disposition", `inline; filename="`+filepath.Base(absPath)+`"`)
	http.ServeFile(w, r, absPath)
}

// errString returns the error's message, or "" if nil. Used to surface
// directory-listing errors in the library overview without failing the request.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}


// --- auth handlers ---

// handleSetup creates the first admin user. Only works when no users exist yet
// (setup mode). After this, /api/login must be used.
// POST /api/setup {"username": "...", "password": "..."}
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        if s.loopbackMode {
                http.Error(w, "setup not needed in loopback mode", http.StatusBadRequest)
                return
        }
        hasAny, _ := s.authStore.HasAnyUser()
        if hasAny {
                writeJSON(w, http.StatusConflict, map[string]any{"error": "setup already completed — use /api/login"})
                return
        }
        var req struct {
                Username string `json:"username"`
                Password string `json:"password"`
        }
        r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                http.Error(w, "invalid body", http.StatusBadRequest)
                return
        }
        req.Username = strings.TrimSpace(req.Username)
        req.Password = strings.TrimSpace(req.Password)
        if req.Username == "" || len(req.Username) > 64 {
                writeJSON(w, http.StatusBadRequest, map[string]any{"error": "username must be 1-64 characters"})
                return
        }
        user, err := s.authStore.Create(req.Username, req.Password, true)
        if err != nil {
                writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
                return
        }
        // Auto-login: create a session and set the cookie so the frontend
        // can immediately start using the API without a separate /login call.
        token := s.authStore.NewSession(user.Username)
        s.setSessionCookie(w, token)
        log.Printf("[entropy] auth: admin user %q created via setup", user.Username)
        writeJSON(w, http.StatusCreated, map[string]any{"user": user, "token": token})
}

// handleLogin authenticates a user and creates a session.
// POST /api/login {"username": "...", "password": "..."}
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        if s.loopbackMode {
                // In loopback mode, return a synthetic admin identity so the
                // frontend's /me check succeeds without showing a login screen.
                writeJSON(w, http.StatusOK, map[string]any{
                        "username": "admin",
                        "is_admin": true,
                        "loopback": true,
                })
                return
        }
        var req struct {
                Username string `json:"username"`
                Password string `json:"password"`
        }
        r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                http.Error(w, "invalid body", http.StatusBadRequest)
                return
        }
        user, err := s.authStore.VerifyPassword(strings.TrimSpace(req.Username), req.Password)
        if err != nil {
                writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid username or password"})
                return
        }
        token := s.authStore.NewSession(user.Username)
        s.setSessionCookie(w, token)
        writeJSON(w, http.StatusOK, map[string]any{
                "username": user.Username,
                "is_admin": user.IsAdmin,
        })
}

// handleLogout invalidates the current session.
// POST /api/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        if s.loopbackMode {
                writeJSON(w, http.StatusOK, map[string]any{"ok": true})
                return
        }
        if cookie, err := r.Cookie(sessionCookieName); err == nil {
                s.authStore.DeleteSession(cookie.Value)
        }
        s.setSessionCookie(w, "") // clear
        writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleMe returns the current authenticated user's identity.
// GET /api/me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
                return
        }
        if s.loopbackMode {
                writeJSON(w, http.StatusOK, map[string]any{
                        "username": "admin",
                        "is_admin": true,
                        "loopback": true,
                })
                return
        }
        // If we get here, the auth middleware has already validated the session
        // and injected the user into context. But /api/me is exempt from auth
        // middleware (so the frontend can check login state without a 401), so
        // we need to read the cookie ourselves here.
        user := currentUser(s, r)
        if user == nil {
                writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "not authenticated"})
                return
        }
        writeJSON(w, http.StatusOK, map[string]any{
                "username": user.Username,
                "is_admin": user.IsAdmin,
        })
}

// handleUsers manages user accounts (admin only).
// GET /api/users — list all users
// POST /api/users {"username": "...", "password": "...", "is_admin": false} — create user
// DELETE /api/users/{username} — delete a non-last-admin user
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
        if s.loopbackMode {
                http.Error(w, "user management not available in loopback mode", http.StatusBadRequest)
                return
        }
        switch r.Method {
        case http.MethodGet:
                users, err := s.authStore.List()
                if err != nil {
                        writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
                        return
                }
                writeJSON(w, http.StatusOK, map[string]any{"users": users})
        case http.MethodPost:
                var req struct {
                        Username string `json:"username"`
                        Password string `json:"password"`
                        IsAdmin  bool   `json:"is_admin"`
                }
                r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
                if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                        http.Error(w, "invalid body", http.StatusBadRequest)
                        return
                }
                user, err := s.authStore.Create(strings.TrimSpace(req.Username), req.Password, req.IsAdmin)
                if err != nil {
                        writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
                        return
                }
                log.Printf("[entropy] auth: user %q created (admin=%v)", user.Username, user.IsAdmin)
                writeJSON(w, http.StatusCreated, map[string]any{"user": user})
        case http.MethodDelete:
                username := strings.TrimPrefix(r.URL.Path, "/api/users/")
                username = strings.TrimSpace(username)
                if username == "" {
                        writeJSON(w, http.StatusBadRequest, map[string]any{"error": "username required"})
                        return
                }
                if err := s.authStore.Delete(username); err != nil {
                        writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
                        return
                }
                log.Printf("[entropy] auth: user %q deleted", username)
                writeJSON(w, http.StatusOK, map[string]any{"ok": true})
        default:
                http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
}

// setSessionCookie is a helper that sets (or clears) the session cookie.
func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
        maxAge := auth.SessionMaxAge
        if token == "" {
                maxAge = -1 // clear
        }
        http.SetCookie(w, &http.Cookie{
                Name:     sessionCookieName,
                Value:    token,
                Path:     "/",
                MaxAge:   int(maxAge.Seconds()),
                HttpOnly: true,
                Secure:   s.secureCookie,
                SameSite: http.SameSiteLaxMode,
        })
}

// currentUser returns the authenticated user for the request, either from the
// auth middleware's context injection (for protected endpoints) or by reading
// the session cookie directly (for exempt endpoints like /api/me).
func currentUser(s *Server, r *http.Request) *auth.User {
        // Try context first (set by withAuthAndAdminRoles middleware).
        if u := auth.FromRequest(r); u != nil {
                return u
        }
        // Fallback: read cookie directly (for exempt paths).
        if s.authStore != nil {
                if cookie, err := r.Cookie(sessionCookieName); err == nil {
                        if username, ok := s.authStore.VerifySession(cookie.Value); ok {
                                if u, err := s.authStore.Get(username); err == nil {
                                        return &u
                                }
                        }
                }
        }
        return nil
}

// requestOwner returns the owner scope for job operations on this request.
// Returns "" (admin/loopback — sees all jobs) if the caller is an admin or in
// loopback mode, otherwise the caller's username.
func requestOwner(s *Server, r *http.Request) string {
        if s.loopbackMode {
                return ""
        }
        u := currentUser(s, r)
        if u == nil {
                return ""
        }
        if u.IsAdmin {
                return ""
        }
        return u.Username
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
	subID, ch, err := s.mgr.Subscribe(requestOwner(s, r))
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
	snap := jobs.Event{Type: "snapshot", Jobs: s.mgr.List(requestOwner(s, r))}
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

func withCORS(port, scheme string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := []string{
			"http://127.0.0.1:" + port,
			"http://localhost:" + port,
			scheme + "://127.0.0.1:" + port,
			scheme + "://localhost:" + port,
		}
		for _, a := range allowed {
			if origin == a {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				break
			}
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

// withRateLimit applies token-bucket rate limiting to all requests except SSE, health, and media streaming.
func withRateLimit(h http.Handler, limiter *ratelimit.Limiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SSE, health, and file-streaming endpoints are exempt
		if r.URL.Path == "/api/jobs/stream" || r.URL.Path == "/api/health" || r.URL.Path == "/api/library/file" {
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

// withAuthAndAdminRoles wraps the handler chain with authentication and admin-only
// enforcement for the network (non-loopback) case. This is NOT used on loopback.
//
// Endpoint categories:
//   - Exempt (no auth needed): /health, /setup, /login, /me, static assets
//   - Any authenticated user: /config, /search, /clean-url, /jobs*, /logs,
//     /import, /stats, /theme, /onboarding
//   - Admin only: /settings (POST), /concurrency, /bandwidth, /smart-routing,
//     /shutdown, /tools/update, /env, /users
//
// The SSE stream (/api/jobs/stream) is a special case: EventSource cannot set
// custom headers, so auth relies entirely on the session cookie which browsers
// send automatically on same-origin requests. The middleware reads the cookie
// here and validates it — no special handling needed.
func withAuthAndAdminRoles(as *auth.Store, cookieName string, h http.Handler) http.Handler {
        // Paths that never require auth.
        exemptPaths := map[string]bool{
                "/api/health": true,
                "/api/setup":  true,
                "/api/login":  true,
                "/api/me":     true, // used by frontend to check auth state
        }

        // Paths that require admin role. Everything else under /api/* just needs
        // any valid session.
        adminPaths := map[string]bool{
                "/api/settings":       true, // POST changes server config
                "/api/concurrency":    true,
                "/api/bandwidth":      true,
                "/api/smart-routing":  true,
                "/api/shutdown":       true,
                "/api/tools/update":   true,
                "/api/env":            true, // leaks system info
                "/api/users":          true, // user management
        }

        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                path := r.URL.Path

                // Exempt paths pass through.
                if exemptPaths[path] {
                        h.ServeHTTP(w, r)
                        return
                }

                // Static assets (no /api/ prefix) pass through — the SPA must load
                // so the login page can render.
                if !strings.HasPrefix(path, "/api/") {
                        h.ServeHTTP(w, r)
                        return
                }

                // Read session cookie.
                cookie, err := r.Cookie(cookieName)
                if err != nil {
                        writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authentication required"})
                        return
                }

                username, ok := as.VerifySession(cookie.Value)
                if !ok {
                        // Clear the bad cookie so the frontend knows to re-login.
                        http.SetCookie(w, &http.Cookie{
                                Name:   cookieName,
                                Value:  "",
                                MaxAge: -1,
                                Path:   "/",
                        })
                        writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "session expired"})
                        return
                }

                // Load user record.
                user, err := as.Get(username)
                if err != nil {
                        as.DeleteSession(cookie.Value)
                        writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "user no longer exists"})
                        return
                }

                // Check admin-only enforcement.
                if adminPaths[path] && !user.IsAdmin {
                        writeJSON(w, http.StatusForbidden, map[string]any{"error": "admin access required"})
                        return
                }

                // Inject user into context so handlers can identify the caller.
                r = auth.WithUser(r, &user)
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
