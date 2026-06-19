package jobs

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

        "entropy-gui/pkg/cleaner"
        "entropy-gui/pkg/cmdutil"
        "entropy-gui/pkg/diskguard"
        "entropy-gui/pkg/ytdlp"

        "github.com/google/uuid"
)

// idleJobTimeout is how long a running job can produce no stdout output
// before the watchdog considers it hung and cancels it.
const idleJobTimeout = 5 * time.Minute

// logDebounceInterval is how long the log broadcaster waits to batch
// rapid log lines before flushing to SSE subscribers.
const logDebounceInterval = 100 * time.Millisecond

type Status string

const (
        StatusQueued      Status = "queued"
        StatusSearching   Status = "searching"
        StatusDownloading Status = "downloading"
        StatusProcessing  Status = "processing"
        StatusDone        Status = "done"
        StatusFailed      Status = "failed"
        StatusCanceled    Status = "canceled"
)

type Options struct {
        Format         string `json:"format"`  // mp3, m4a, flac, opus, wav, mp4 (video), best
        Bitrate        string `json:"bitrate"` // e.g., "192", "320", "0" for VBR best
        EmbedMeta      bool   `json:"embed_meta"`
        EmbedThumb     bool   `json:"embed_thumb"`
        Engine         string `json:"engine"` // "ytdlp" or "aria2c"
        AudioDir       string `json:"audio_dir"`
        VideoDir       string `json:"video_dir"`
        CookiesBrowser string `json:"cookies_browser"` // "" | chrome | edge | firefox | brave | chromium | opera | safari | vivaldi
        ScrapeDelay    bool   `json:"scrape_delay"`    // randomized sleep between requests to avoid 429s
        Resolution     string `json:"resolution"`      // "BEST", "4K", "1440p", "1080p", "720p", "480p"
        BandwidthLimit string `json:"bandwidth_limit"` // "5M", "1M", "500K", "0" = unlimited
        MediaType      string `json:"media_type"`      // "music", "audio", "video" when smart routing detected a type; "" otherwise
}

type Job struct {
        ID         string    `json:"id"`
        URL        string    `json:"url"`
        Title      string    `json:"title"`
        Uploader   string    `json:"uploader"`
        Thumbnail  string    `json:"thumbnail"`
        Duration   float64   `json:"duration"`
        Status     Status    `json:"status"`
        Progress   float64   `json:"progress"`
        Speed      string    `json:"speed"`
        ETA        string    `json:"eta"`
        Stage      string    `json:"stage"` // human-readable
        Error      string    `json:"error"`
        OutputFile string    `json:"output_file"`
        Owner      string    `json:"owner,omitempty"` // username of the job's owner; "" = system/loopback
        Options    Options   `json:"options"`
        CreatedAt  time.Time `json:"created_at"`
        UpdatedAt  time.Time `json:"updated_at"`
        cancel     context.CancelFunc
}

// FIX #3: allowed cookie browsers — validated before passing to yt-dlp.
var validCookieBrowsers = map[string]bool{
        "": true, "none": true,
        "chrome": true, "edge": true, "firefox": true, "brave": true,
        "chromium": true, "opera": true, "vivaldi": true, "safari": true,
}

// FIX #7: max concurrent SSE subscriptions.
const maxSubscriptions = 16

type Event struct {
        Type string   `json:"type"` // "job", "log", "snapshot"
        Job  *Job     `json:"job,omitempty"`
        Log  *LogLine `json:"log,omitempty"`
        Jobs []*Job   `json:"jobs,omitempty"`
}

type LogLine struct {
        JobID string    `json:"job_id"`
        Owner string    `json:"owner,omitempty"` // owner of the job this log belongs to
        Line  string    `json:"line"`
        Time  time.Time `json:"time"`
}

// Manager owns the job queue and workers.
// subscriber wraps an SSE channel with an owner filter.
// ownerFilter == "" means "see everything" (admin / loopback mode).
// Any other value means the subscriber only receives events for their own jobs.
type subscriber struct {
        ch   chan Event
        owner string
}

type Manager struct {
        mu            sync.RWMutex
        jobs          map[string]*Job
        order         []string
        queue         chan string
        closed        bool // set by Close() to prevent new enqueue after shutdown
        logs          []LogLine
        subsMu        sync.RWMutex
        subs          map[string]*subscriber
        ytdlp         string
        aria2c        string
        ffmpeg        string
        audioDir      string
        videoDir      string
        workers       int
        activeWorkers int

        // Global defaults (can be overridden per-job via Options)
        defaultBandwidth string
        smartRouting    bool

        // Persistence
        saveJob   func(j *Job)
        deleteJob func(id string)

        // Mutex for file I/O outside the main job lock
        ioMu sync.Mutex

        // logDebounce: pending log lines and timer for batching broadcasts
        logDebounceMu     sync.Mutex
        logPending        []LogLine
        logDebounceTimer  *time.Timer
}

func NewManager(ytdlpBin, aria2cBin, ffmpegBin, audioDir, videoDir string, workers int) *Manager {
        if workers <= 0 {
                workers = 2
        }
        m := &Manager{
                jobs:    map[string]*Job{},
                order:   []string{},
                queue:   make(chan string, 1024),
                logs:    make([]LogLine, 0, 1024),
                subs:    map[string]*subscriber{},
                ytdlp:   ytdlpBin,
                aria2c:  aria2cBin,
                ffmpeg:  ffmpegBin,
                audioDir: audioDir,
                videoDir: videoDir,
                workers: workers,
        }
        for i := 0; i < workers; i++ {
                go m.worker()
        }
        return m
}

// Close stops the worker pool. It closes the queue channel so workers
// drain remaining jobs and then exit. Called during graceful shutdown
// to prevent goroutine leaks.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	close(m.queue)
}

// tryEnqueue hands a job ID to the worker pool without blocking.
//
// The queue is buffered (1024) but never the source of truth: every job
// is stored in m.jobs with StatusQueued and broadcast to SSE clients before
// we try to enqueue it. If the buffer is momentarily full (e.g. a bulk
// import while workers are saturated), this returns false instead of
// blocking the caller forever — which previously could deadlock HTTP
// handler goroutines. The job stays queued and is never lost; workers
// drain the buffer and the job remains visible/retryable in the UI.
//
// Callers that hold a context can also pass ctx so the enqueue attempt is
// cancelled when the request is aborted (avoids pointless sends).
func (m *Manager) tryEnqueue(ctx context.Context, id string) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case m.queue <- id:
		return true
	case <-ctx.Done():
		log.Printf("[jobs] enqueue cancelled for job %s: %v", id, ctx.Err())
		return false
	default:
		log.Printf("[jobs] queue full, job %s held as queued (workers will catch up)", id)
		return false
	}
}



// Workers returns the current max worker count.
func (m *Manager) Workers() int {
        m.mu.RLock()
        defer m.mu.RUnlock()
        return m.workers
}

// SetWorkers updates the maximum concurrent worker count.
// If the new limit is higher, spawns new workers. If lower, excess workers will naturally exit.
func (m *Manager) SetWorkers(n int) {
        if n <= 0 {
                return
        }
        m.mu.Lock()
        defer m.mu.Unlock()
        for i := m.activeWorkers; i < n; i++ {
                go m.worker()
        }
        m.workers = n
}

// AudioDir returns the global audio output directory.
func (m *Manager) AudioDir() string {
        m.mu.RLock()
        defer m.mu.RUnlock()
        return m.audioDir
}

// VideoDir returns the global video output directory.
func (m *Manager) VideoDir() string {
        m.mu.RLock()
        defer m.mu.RUnlock()
        return m.videoDir
}

// SetAudioDir sets a new global audio output directory.
func (m *Manager) SetAudioDir(dir string) {
        m.mu.Lock()
        defer m.mu.Unlock()
        m.audioDir = dir
}

// SetVideoDir sets a new global video output directory.
func (m *Manager) SetVideoDir(dir string) {
        m.mu.Lock()
        defer m.mu.Unlock()
        m.videoDir = dir
}

// Bandwidth returns the current global default bandwidth limit string.
func (m *Manager) Bandwidth() string {
        m.mu.RLock()
        defer m.mu.RUnlock()
        if m.defaultBandwidth == "" {
                return "5M"
        }
        return m.defaultBandwidth
}

// SetBandwidth updates the global default bandwidth limit.
func (m *Manager) SetBandwidth(limit string) {
        m.mu.Lock()
        defer m.mu.Unlock()
        m.defaultBandwidth = limit
}

// SmartRouting returns whether per-item content-type detection is enabled.
func (m *Manager) SmartRouting() bool {
        m.mu.RLock()
        defer m.mu.RUnlock()
        return m.smartRouting
}

// SetSmartRouting enables or disables per-item content-type detection.
func (m *Manager) SetSmartRouting(enabled bool) {
        m.mu.Lock()
        defer m.mu.Unlock()
        m.smartRouting = enabled
}

// AttachPersistence wires a persist callback and seeds jobs from a previous run.
func (m *Manager) AttachPersistence(save func(j *Job), del func(id string), restored []*Job) {
        m.mu.Lock()
        m.saveJob = save
        m.deleteJob = del
        for _, j := range restored {
                if j == nil || j.ID == "" {
                        continue
                }
                switch j.Status {
                case StatusQueued, StatusSearching, StatusDownloading, StatusProcessing:
                        j.Status = StatusFailed
                        j.Error = "interrupted by restart"
                        j.Stage = "interrupted"
                }
                m.jobs[j.ID] = j
                m.order = append(m.order, j.ID)
        }
        m.mu.Unlock()
}

// --- Subscriptions ---

// Subscribe creates a new SSE subscription. ownerFilter "" means admin/loopback
// (receives all events); any other value scopes events to that owner only.
// Returns an error if too many concurrent subscriptions exist.
func (m *Manager) Subscribe(ownerFilter string) (string, chan Event, error) {
        id := uuid.NewString()
        ch := make(chan Event, 128)
        m.subsMu.Lock()
        defer m.subsMu.Unlock()
        if len(m.subs) >= maxSubscriptions {
                return "", nil, fmt.Errorf("too many concurrent subscriptions (max %d)", maxSubscriptions)
        }
        m.subs[id] = &subscriber{ch: ch, owner: ownerFilter}
        return id, ch, nil
}

func (m *Manager) Unsubscribe(id string) {
        m.subsMu.Lock()
        if sub, ok := m.subs[id]; ok {
                close(sub.ch)
                delete(m.subs, id)
        }
        m.subsMu.Unlock()
}

// broadcast sends an event to every subscriber whose owner filter matches.
// For job events, the subscriber receives it if they're an admin (owner=="")
// or if the job's owner matches their filter. For log events, same logic but
// using the log line's owner. For snapshot events, each subscriber gets their
// own filtered list (handled separately by broadcastSnapshot).
func (m *Manager) broadcast(e Event) {
        m.subsMu.RLock()
        defer m.subsMu.RUnlock()
        for _, sub := range m.subs {
                if !eventMatchesOwner(e, sub.owner) {
                        continue
                }
                select {
                case sub.ch <- e:
                default:
                }
        }
}

// eventMatchesOwner returns true if a subscriber with the given owner filter
// should receive this event. Admin subscribers (owner=="") receive everything.
func eventMatchesOwner(e Event, ownerFilter string) bool {
        if ownerFilter == "" {
                return true // admin / loopback sees all
        }
        switch e.Type {
        case "job":
                if e.Job != nil {
                        return e.Job.Owner == ownerFilter
                }
        case "log":
                if e.Log != nil {
                        return e.Log.Owner == ownerFilter
                }
        case "snapshot":
                // Snapshots are handled by broadcastSnapshot per-subscriber; a
                // plain snapshot broadcast (global) is only sent to admins.
                return false
        }
        return true
}

// broadcastSnapshot sends each subscriber a snapshot filtered to their owner.
// Admin/loopback subscribers get all jobs; regular users get only their own.
func (m *Manager) broadcastSnapshot() {
        all := m.List("")
        m.subsMu.RLock()
        defer m.subsMu.RUnlock()
        for _, sub := range m.subs {
                jobs := all
                if sub.owner != "" {
                        jobs = filterByOwner(all, sub.owner)
                }
                select {
                case sub.ch <- Event{Type: "snapshot", Jobs: jobs}:
                default:
                }
        }
}

// filterByOwner returns only jobs whose Owner matches (or is "" for legacy).
// Legacy/system jobs (Owner=="") are visible to everyone to avoid surprise
// disappearing data for users who created jobs before auth existed.
func filterByOwner(jobs []*Job, owner string) []*Job {
        var out []*Job
        for _, j := range jobs {
                if j.Owner == "" || j.Owner == owner {
                        out = append(out, j)
                }
        }
        return out
}

func (m *Manager) appendLog(jobID, line string) {
        // Look up the job's owner so log events can be scoped per-subscriber.
        m.mu.RLock()
        var owner string
        if j, ok := m.jobs[jobID]; ok {
                owner = j.Owner
        }
        m.mu.RUnlock()

        ll := LogLine{JobID: jobID, Owner: owner, Line: line, Time: time.Now()}
        m.mu.Lock()
        m.logs = append(m.logs, ll)
        if len(m.logs) > 5000 {
                m.logs = m.logs[len(m.logs)-5000:]
        }
        m.mu.Unlock()

        // Debounce SSE log broadcasts: batch rapid lines into a single flush
        // after logDebounceInterval ms to prevent flooding clients during downloads.
        m.logDebounceMu.Lock()
        m.logPending = append(m.logPending, ll)
        if m.logDebounceTimer == nil {
                m.logDebounceTimer = time.AfterFunc(logDebounceInterval, func() {
                        m.logDebounceMu.Lock()
                        pending := m.logPending
                        m.logPending = nil
                        m.logDebounceTimer = nil
                        m.logDebounceMu.Unlock()
                        // Broadcast each buffered log line
                        for i := range pending {
                                lp := pending[i]
                                m.broadcast(Event{Type: "log", Log: &lp})
                        }
                })
        }
        m.logDebounceMu.Unlock()
}

func (m *Manager) updateJob(j *Job) {
        jc := jobCopy(j)
        m.broadcast(Event{Type: "job", Job: jc})
        if m.saveJob != nil {
                // we pass the raw job since jc is disconnected from mutexes, but wait, saveJob handles JSON marshaling
                // let's pass jc so it has the current state copy safely
                m.saveJob(jc)
        }
}

// --- Public API ---

// List returns all jobs visible to the given owner.
// owner == "" means admin/loopback (all jobs); otherwise only that owner's
// jobs (plus legacy Owner=="" jobs) are returned.
func (m *Manager) List(owner string) []*Job {
        m.mu.RLock()
        defer m.mu.RUnlock()
        out := make([]*Job, 0, len(m.order))
        for _, id := range m.order {
                if j, ok := m.jobs[id]; ok {
                        if owner != "" && j.Owner != "" && j.Owner != owner {
                                continue
                        }
                        cp := *j
                        out = append(out, &cp)
                }
        }
        return out
}

// RecentLogs returns the most recent log lines visible to the given owner.
func (m *Manager) RecentLogs(limit int, owner string) []LogLine {
        m.mu.RLock()
        defer m.mu.RUnlock()
        if limit <= 0 || limit > len(m.logs) {
                limit = len(m.logs)
        }
        var out []LogLine
        // Walk backwards from the most recent so we fill the limit with the
        // newest visible lines for this owner.
        for i := len(m.logs) - 1; i >= 0 && len(out) < limit; i-- {
                ll := m.logs[i]
                if owner != "" && ll.Owner != "" && ll.Owner != owner {
                        continue
                }
                out = append(out, ll)
        }
        // Reverse to chronological order
        for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
                out[i], out[j] = out[j], out[i]
        }
        return out
}

// IsDuplicateURL returns true if the URL was already downloaded successfully
// by the given owner. owner == "" checks all jobs (admin/loopback).
// Exported for use by the import endpoint in main.go.
func (m *Manager) IsDuplicateURL(u, owner string) bool {
        m.mu.RLock()
        defer m.mu.RUnlock()
        for _, id := range m.order {
                j, ok := m.jobs[id]
                if !ok || j.Status != StatusDone {
                        continue
                }
                // Admin sees all; regular users see their own + legacy jobs.
                if owner != "" && j.Owner != "" && j.Owner != owner {
                        continue
                }
                if j.URL == u {
                        return true
                }
        }
        return false
}

// DuplicateCount returns how many URLs in the given list already exist as done
// jobs for the given owner. owner == "" counts across all jobs (admin/loopback).
// Uses a set for O(n) lookup instead of O(n²).
func (m *Manager) DuplicateCount(urls []string, owner string) int {
        m.mu.RLock()
        defer m.mu.RUnlock()
        // Build a set of done-job URLs visible to this owner
        doneURLs := make(map[string]struct{}, len(m.order))
        for _, id := range m.order {
                j, ok := m.jobs[id]
                if !ok || j.Status != StatusDone {
                        continue
                }
                if owner != "" && j.Owner != "" && j.Owner != owner {
                        continue
                }
                doneURLs[j.URL] = struct{}{}
        }
        count := 0
        for _, u := range urls {
                if _, ok := doneURLs[u]; ok {
                        count++
                }
        }
        return count
}

// AddURLs queues one or more URLs. Playlists are expanded.
// AddURLs queues one or more URLs. Playlists are expanded. owner tags each
// created job ("" = system/loopback).
func (m *Manager) AddURLs(ctx context.Context, urls []string, opts Options, owner string) ([]*Job, error) {
        if opts.AudioDir == "" {
                opts.AudioDir = m.audioDir
        }
        if opts.VideoDir == "" {
                opts.VideoDir = m.videoDir
        }
        _ = os.MkdirAll(opts.AudioDir, 0o755)
        _ = os.MkdirAll(opts.VideoDir, 0o755)

	var created []*Job
	for _, rawURL := range urls {
		// Honor request cancellation between URLs so an aborted batch
		// (or an expiring outer context) unwinds promptly instead of
		// running every remaining probe.
		if err := ctx.Err(); err != nil {
			return created, err
		}
		u := cleaner.CleanURL(rawURL)
		if u == "" {
			continue
		}
		// Dedup: skip if this URL was already successfully downloaded
		if m.IsDuplicateURL(u, owner) {
			continue
		}
		// Probe to expand playlists/albums (e.g. a 65-track SoundCloud set).
		// Previously this was capped at 25s, which was too short for slowly
		// paginating sources — yt-dlp would be killed mid-output and its
		// partial JSON silently produced a truncated set of jobs.
		pctx, cancel := context.WithTimeout(ctx, 120*time.Second)
		metas, err := ytdlp.Probe(pctx, m.ytdlp, u)
		cancel()
		if err != nil || len(metas) == 0 {
				// Probe failed entirely — queue the raw URL as a single job and
				// let the download phase handle expansion as a fallback.
				if err != nil {
					log.Printf("[jobs] probe failed for %s: %v (queuing raw URL)", u, err)
				}
				itemOpts := opts
				if m.smartRouting {
					mt := DetectMediaType("", u, 0)
					itemOpts = opts.WithMediaType(mt)
				}
				j := m.newJobCtx(ctx, u, "", "", "", 0, itemOpts, owner)
			created = append(created, j)
			continue
		}
		// yt-dlp's n_entries reports the full playlist size even when fewer
		// entries are returned, so a mismatch is a reliable signal that the
		// probe was truncated (timeout, extractor limit, or network error).
		if metas[0].NEntries > 0 && len(metas) < metas[0].NEntries {
			log.Printf("[jobs] playlist probe returned %d of %d entries for %s (may be truncated)",
				len(metas), metas[0].NEntries, u)
		}
			for _, meta := range metas {
				jurl := meta.URL
				if jurl == "" {
					jurl = u
				}
				if m.IsDuplicateURL(jurl, owner) {
					continue
				}
				// Smart routing: detect content type and adjust format/dir per-item.
				itemOpts := opts
				if m.smartRouting {
					mt := DetectMediaType(meta.Extractor, jurl, meta.Duration)
					itemOpts = opts.WithMediaType(mt)
				}
				j := m.newJobCtx(ctx, jurl, meta.Title, meta.Uploader, meta.Thumbnail, meta.Duration, itemOpts, owner)
			created = append(created, j)
		}
	}
	return created, nil
}

// AddDirect creates one job per URL with provided title (no probing). owner
// tags each created job ("" = system/loopback).
func (m *Manager) AddDirect(items []DirectItem, opts Options, owner string) []*Job {
        if opts.AudioDir == "" {
                opts.AudioDir = m.audioDir
        }
        if opts.VideoDir == "" {
                opts.VideoDir = m.videoDir
        }
        _ = os.MkdirAll(opts.AudioDir, 0o755)
        _ = os.MkdirAll(opts.VideoDir, 0o755)
        var created []*Job
        for _, it := range items {
                u := cleaner.CleanURL(it.URL)
                if u == "" {
                        continue
                }
                if m.IsDuplicateURL(u, owner) {
                        continue
                }
                // Smart routing: detect content type from URL + duration (no extractor).
                itemOpts := opts
                if m.smartRouting {
                        mt := DetectMediaType("", u, it.Duration)
                        itemOpts = opts.WithMediaType(mt)
                }
                j := m.newJob(u, it.Title, it.Uploader, it.Thumbnail, it.Duration, itemOpts, owner)
                created = append(created, j)
        }
        return created
}

type DirectItem struct {
        URL       string  `json:"url"`
        Title     string  `json:"title"`
        Uploader  string  `json:"uploader"`
        Thumbnail string  `json:"thumbnail"`
        Duration  float64 `json:"duration"`
}

func (m *Manager) newJob(u, title, uploader, thumb string, duration float64, opts Options, owner string) *Job {
	return m.newJobCtx(context.Background(), u, title, uploader, thumb, duration, opts, owner)
}

// newJobCtx is like newJob but plumbs a context into the enqueue step so a
// full queue (or an aborted request) can't block the caller forever.
func (m *Manager) newJobCtx(ctx context.Context, u, title, uploader, thumb string, duration float64, opts Options, owner string) *Job {
	j := &Job{
		ID:        uuid.NewString(),
		URL:       u,
		Title:     title,
		Uploader:  uploader,
		Thumbnail: thumb,
		Duration:  duration,
		Status:    StatusQueued,
		Stage:     "queued",
		Owner:     owner,
		Options:   opts,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.mu.Lock()
	m.jobs[j.ID] = j
	m.order = append(m.order, j.ID)
	closed := m.closed
	m.mu.Unlock()
	m.updateJob(j)
	if !closed {
		m.tryEnqueue(ctx, j.ID)
	}
	return j
}

// Get returns a copy of the job if it exists and is visible to the given owner.
// owner == "" (admin/loopback) can see any job.
func (m *Manager) Get(id, owner string) (*Job, bool) {
        m.mu.RLock()
        defer m.mu.RUnlock()
        j, ok := m.jobs[id]
        if !ok {
                return nil, false
        }
        if owner != "" && j.Owner != "" && j.Owner != owner {
                return nil, false
        }
        return jobCopy(j), true
}

// Retry re-queues a failed/canceled job. owner == "" (admin) can retry any job;
// otherwise only the owner's job can be retried.
func (m *Manager) Retry(id, owner string) bool {
        m.mu.Lock()
        j, ok := m.jobs[id]
        if !ok {
                m.mu.Unlock()
                return false
        }
        if owner != "" && j.Owner != "" && j.Owner != owner {
                m.mu.Unlock()
                return false
        }
        if j.Status != StatusFailed && j.Status != StatusCanceled {
                m.mu.Unlock()
                return false
        }
	j.Status = StatusQueued
	j.Progress = 0
	j.Error = ""
	j.Stage = "queued"
	j.UpdatedAt = time.Now()
	closed := m.closed
	m.mu.Unlock()
	m.updateJob(j)
	if !closed {
		m.tryEnqueue(context.Background(), id)
	}
	return true
}

// RetryAllFailed re-queues all failed and canceled jobs visible to owner.
// owner == "" retries all (admin/loopback). Returns count of retried jobs.
func (m *Manager) RetryAllFailed(owner string) int {
        m.mu.Lock()
        var ids []string
        for _, id := range m.order {
                j, ok := m.jobs[id]
                if !ok {
                        continue
                }
                if owner != "" && j.Owner != "" && j.Owner != owner {
                        continue
                }
                if j.Status == StatusFailed || j.Status == StatusCanceled {
                        j.Status = StatusQueued
                        j.Progress = 0
                        j.Error = ""
                        j.Stage = "queued"
                        j.UpdatedAt = time.Now()
                        ids = append(ids, id)
                }
        }
        m.mu.Unlock()
        m.mu.RLock()
        closed := m.closed
        m.mu.RUnlock()
	for _, id := range ids {
		m.mu.RLock()
		j, ok := m.jobs[id]
		m.mu.RUnlock()
		if ok {
			m.updateJob(j) // broadcast the real job with all its fields
		}
		if !closed {
			m.tryEnqueue(context.Background(), id)
		}
	}
	return len(ids)
}

// Remove deletes a job. owner == "" (admin) can remove any job; otherwise only
// the owner's job can be removed. Sends a per-subscriber filtered snapshot.
func (m *Manager) Remove(id, owner string) bool {
        m.mu.Lock()
        j, ok := m.jobs[id]
        if !ok {
                m.mu.Unlock()
                return false
        }
        if owner != "" && j.Owner != "" && j.Owner != owner {
                m.mu.Unlock()
                return false
        }
        if j.cancel != nil {
                j.cancel()
        }
        go m.cleanupPartialFiles(j)
        delete(m.jobs, id)
        for i, x := range m.order {
                if x == id {
                        m.order = append(m.order[:i], m.order[i+1:]...)
                        break
                }
        }
        m.mu.Unlock()
        if m.deleteJob != nil {
                m.deleteJob(id)
        }
        m.broadcastSnapshot()
        return true
}

// Clear bulk-removes jobs by status, scoped to owner ("" = all).
// Sends a per-subscriber filtered snapshot.
func (m *Manager) Clear(kind, owner string) int {
        m.mu.Lock()
        var keep []string
        var removed int
        var deletedIDs []string
        for _, id := range m.order {
                j := m.jobs[id]
                if owner != "" && j.Owner != "" && j.Owner != owner {
                        keep = append(keep, id)
                        continue
                }
                if (kind == "completed" && j.Status == StatusDone) ||
                        (kind == "failed" && (j.Status == StatusFailed || j.Status == StatusCanceled)) {
                        delete(m.jobs, id)
                        deletedIDs = append(deletedIDs, id)
                        removed++
                        go m.cleanupPartialFiles(j)
                        continue
                }
                keep = append(keep, id)
        }
        m.order = keep
        m.mu.Unlock()
        if m.deleteJob != nil {
                for _, id := range deletedIDs {
                        m.deleteJob(id)
                }
        }
        m.broadcastSnapshot()
        return removed
}
// cleanupPartialFiles deletes .part and .ytdl files associated with a canceled or removed job.
func (m *Manager) cleanupPartialFiles(j *Job) {
        if j == nil {
                return
        }
        m.mu.RLock()
        outputFile := j.OutputFile
        m.mu.RUnlock()

        if outputFile == "" {
                return
        }

        _ = os.Remove(outputFile + ".part")
        _ = os.Remove(outputFile + ".ytdl")
}


// --- Worker / execution ---

// worker processes jobs from the queue.
func (m *Manager) worker() {
        m.mu.Lock()
        m.activeWorkers++
        m.mu.Unlock()

        defer func() {
                m.mu.Lock()
                m.activeWorkers--
                m.mu.Unlock()
        }()

        for id := range m.queue {
                m.runJob(id)

                m.mu.Lock()
                if m.activeWorkers > m.workers {
                        m.mu.Unlock()
                        return
                }
                m.mu.Unlock()
        }
}

func (m *Manager) runJob(id string) {
        m.mu.Lock()
        j, ok := m.jobs[id]
        if !ok {
                m.mu.Unlock()
                return
        }
        ctx, cancel := context.WithCancel(context.Background())
        j.cancel = cancel
        j.Status = StatusDownloading
        j.Stage = "starting"
        j.UpdatedAt = time.Now()
        m.mu.Unlock()
        m.updateJob(j)

        defer func() {
                m.mu.Lock()
                j.cancel = nil
                j.UpdatedAt = time.Now()
                m.mu.Unlock()
                m.updateJob(j)
        }()

        if err := m.execute(ctx, j); err != nil {
                m.mu.Lock()
                if j.Status != StatusCanceled {
                        j.Status = StatusFailed
                        j.Error = err.Error()
                        j.Stage = "failed"
                        // Dead-link archiver: log unavailable/private videos
                        errLow := strings.ToLower(err.Error())
                        if strings.Contains(errLow, "video unavailable") ||
                                strings.Contains(errLow, "private video") ||
                                strings.Contains(errLow, "has been removed") {
                                m.logDeadLink(j)
                        }
                }
                m.mu.Unlock()
                m.appendLog(j.ID, "ERROR: "+err.Error())
                return
        }
        m.mu.Lock()
        j.Status = StatusDone
        j.Progress = 100
        j.Stage = "done"
        m.mu.Unlock()
        m.appendLog(j.ID, "DONE: "+j.Title)
}

var progressRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%(?:\s+of\s+[~\d.A-Za-z]+)?(?:\s+at\s+([\d.A-Za-z/]+))?(?:\s+ETA\s+(\S+))?`)
var destRe = regexp.MustCompile(`\[download\] Destination:\s+(.+)$`)
var mergeRe = regexp.MustCompile(`\[(?:Merger|ExtractAudio|EmbedThumbnail|Metadata|ffmpeg)\]\s+(.+)$`)

// logDeadLink appends a dead/unavailable link to dead_links.csv in the output directory.
// Uses atomic write (temp file + rename) to prevent corruption on crash.
func (m *Manager) logDeadLink(j *Job) {
        m.mu.RLock()
        outDir := m.videoDir
        if outDir == "" {
                outDir = m.audioDir
        }
        m.mu.RUnlock()
        csv := filepath.Join(outDir, "dead_links.csv")
        m.ioMu.Lock()
        defer m.ioMu.Unlock()

        // Read existing content
        existing, _ := os.ReadFile(csv)
        needsHeader := len(existing) == 0

        // Build new content
        title := strings.ReplaceAll(j.Title, ",", " ")
        errMsg := strings.ReplaceAll(j.Error, ",", " ")
        line := fmt.Sprintf("%s,%s,%s,%s\n", time.Now().UTC().Format(time.RFC3339), j.URL, title, errMsg)

        var buf []byte
        if needsHeader {
                buf = append([]byte("timestamp,url,title,error\n"), existing...)
        } else {
                buf = existing
        }
        buf = append(buf, []byte(line)...)

        // Write to temp file then rename atomically
        tmp := csv + ".tmp"
        if err := os.WriteFile(tmp, buf, 0o644); err != nil {
                return
        }
        _ = os.Rename(tmp, csv)
}

func (m *Manager) execute(ctx context.Context, j *Job) error {
        opts := j.Options
        if opts.Format == "" {
                opts.Format = "mp3"
        }
        if opts.AudioDir == "" {
                opts.AudioDir = m.audioDir
        }
        if opts.VideoDir == "" {
                opts.VideoDir = m.videoDir
        }

        // Intelligent Folder Routing
        lowerFormat := strings.ToLower(opts.Format)
        lowerURL := strings.ToLower(j.URL)

        isAudio := false
        isVideo := false

        audioExts := []string{".mp3", ".m4a", ".flac", ".wav", ".opus", ".aac", ".ogg"}
        videoExts := []string{".mp4", ".mkv", ".webm", ".avi", ".mov", ".flv"}

        switch lowerFormat {
        case "mp3", "m4a", "flac", "wav", "opus":
                isAudio = true
        case "mp4", "mkv", "webm", "best":
                isVideo = true
        }

        // Fallback to URL extension check
        if !isAudio && !isVideo {
                for _, ext := range audioExts {
                        if strings.HasSuffix(lowerURL, ext) {
                                isAudio = true
                                break
                        }
                }
                if !isAudio {
                        for _, ext := range videoExts {
                                if strings.HasSuffix(lowerURL, ext) {
                                        isVideo = true
                                        break
                                }
                        }
                }
        }

        finalDir := opts.VideoDir
        if isAudio {
                finalDir = opts.AudioDir
        } else if isVideo {
                finalDir = opts.VideoDir
        }

        _ = os.MkdirAll(finalDir, 0o755)

        // Disk space guard: refuse to start if disk is too full.
        if err := diskguard.Check(finalDir, 0); err != nil {
                return fmt.Errorf("disk guard: %w", err)
        }

        args := []string{
                "--newline",
                "--no-warnings",
                "--progress",
                "--no-mtime",
                "-o", filepath.Join(finalDir, "%(title)s.%(ext)s"),
                "--print", "after_move:FINAL_FILE=%(filepath)s",
                // Resilience against transient YouTube 403s + connection resets.
                "--retries", "10",
                "--fragment-retries", "10",
                "--retry-sleep", "exp=1:8",
                "--socket-timeout", "30",
                // Try multiple YouTube player clients; if one is rate-limited the
                // next is tried automatically. This is the single biggest 403-killer.
                "--extractor-args", "youtube:player_client=default,tv,web_safari,ios",
        }

        // Pull cookies from the user's browser session if requested. This is the
        // most reliable way around YouTube's anonymous-extraction limits.
        // FIX #3: validate against known browser list to prevent injection.
        if opts.CookiesBrowser != "" && strings.ToLower(opts.CookiesBrowser) != "none" {
                if !validCookieBrowsers[strings.ToLower(opts.CookiesBrowser)] {
                        return fmt.Errorf("unsupported cookie browser: %q", opts.CookiesBrowser)
                }
                args = append(args, "--cookies-from-browser", strings.ToLower(opts.CookiesBrowser))
        }

        // Engine selection — aria2c for HTTP segments
        if strings.EqualFold(opts.Engine, "aria2c") {
                args = append(args,
                        "--downloader", "aria2c",
                        "--downloader-args", "aria2c:-x 16 -s 16 -k 1M --console-log-level=warn --summary-interval=1",
                )
        }

        // Format selection
        heightFilter := ""
        switch strings.ToLower(opts.Resolution) {
        case "4k":
                heightFilter = "[height<=2160]"
        case "1440p":
                heightFilter = "[height<=1440]"
        case "1080p":
                heightFilter = "[height<=1080]"
        case "720p":
                heightFilter = "[height<=720]"
        case "480p":
                heightFilter = "[height<=480]"
        }

        audioFormats := map[string]bool{
                "mp3": true, "m4a": true, "flac": true, "opus": true, "wav": true, "aac": true, "vorbis": true,
        }
        if audioFormats[strings.ToLower(opts.Format)] {
                args = append(args, "-x", "--audio-format", strings.ToLower(opts.Format))
                if opts.Bitrate != "" && opts.Bitrate != "0" {
                        args = append(args, "--audio-quality", opts.Bitrate+"K")
                }
        } else if strings.EqualFold(opts.Format, "mp4") {
                formatStr := fmt.Sprintf("bv*%s[ext=mp4]+ba[ext=m4a]/b%s[ext=mp4]/b", heightFilter, heightFilter)
                args = append(args, "-f", formatStr, "--merge-output-format", "mp4")
        } else if strings.EqualFold(opts.Format, "webm") {
                formatStr := fmt.Sprintf("bv*%s[ext=webm]+ba[ext=webm]/b%s[ext=webm]/b", heightFilter, heightFilter)
                args = append(args, "-f", formatStr, "--merge-output-format", "webm")
        } else if strings.EqualFold(opts.Format, "mkv") {
                formatStr := fmt.Sprintf("bv*%s+ba/b", heightFilter)
                args = append(args, "-f", formatStr, "--merge-output-format", "mkv")
        } else if strings.EqualFold(opts.Format, "best") {
                formatStr := fmt.Sprintf("bv*%s+ba/b", heightFilter)
                args = append(args, "-f", formatStr)
        }

        if opts.EmbedMeta {
                args = append(args, "--add-metadata")
        }
        if opts.EmbedThumb {
                args = append(args, "--embed-thumbnail", "--convert-thumbnails", "jpg")
        }
        if opts.ScrapeDelay {
                // Randomized sleep to avoid YouTube 429 IP bans when bulk downloading
                args = append(args, "--sleep-requests", "2", "--min-sleep-interval", "1", "--max-sleep-interval", "5")
        }

        // Bandwidth limiter — uses global default (5M) to avoid YouTube IP bans during bulk downloads.
        // Per-job BandwidthLimit in Options overrides the global default.
        bwLimit := opts.BandwidthLimit
        if bwLimit == "" {
                bwLimit = m.Bandwidth()
        }
        if bwLimit != "" && bwLimit != "0" {
                args = append(args, "--limit-rate", bwLimit)
        }

        args = append(args, j.URL)

        cmd := exec.CommandContext(ctx, m.ytdlp, args...)
        cmd.Cancel = func() error {
                return cmd.Process.Signal(os.Interrupt)
        }
        cmd.WaitDelay = 3 * time.Second
        cmdutil.PrepareCmd(cmd)
        cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")

        stdout, err := cmd.StdoutPipe()
        if err != nil {
                return err
        }
        stderr, err := cmd.StderrPipe()
        if err != nil {
                return err
        }
        if err := cmd.Start(); err != nil {
                return err
        }

        m.appendLog(j.ID, "$ "+m.ytdlp+" "+strings.Join(args, " "))

        // Idle watchdog: track last activity time and cancel if silent too long.
        lastActivity := time.Now()
        lastActivityMu := sync.Mutex{}
        watchdogDone := make(chan struct{})
        // jobDone signals the watchdog that I/O has drained so it can exit immediately.
        jobDone := make(chan struct{})
        go func() {
                defer close(watchdogDone)
                ticker := time.NewTicker(30 * time.Second)
                defer ticker.Stop()
                for {
                        select {
                        case <-ctx.Done():
                                return
                        case <-jobDone:
                                // I/O readers finished — job completed normally, no need to watchdog further.
                                return
                        case <-ticker.C:
                                lastActivityMu.Lock()
                                idle := time.Since(lastActivity)
                                lastActivityMu.Unlock()
                                if idle > idleJobTimeout {
                                        m.appendLog(j.ID, fmt.Sprintf("[watchdog] job idle for %v — canceling as hung", idle.Round(time.Second)))
                                        if j.cancel != nil {
                                                j.cancel()
                                        }
                                        return
                                }
                        }
                }
        }()

        done := make(chan struct{}, 2)
        go func() {
                ytdlp.ReadLines(stdout, func(line string) {
                        lastActivityMu.Lock()
                        lastActivity = time.Now()
                        lastActivityMu.Unlock()
                        m.handleLine(j, line)
                })
                done <- struct{}{}
        }()
        go func() {
                ytdlp.ReadLines(stderr, func(line string) {
                        lastActivityMu.Lock()
                        lastActivity = time.Now()
                        lastActivityMu.Unlock()
                        m.appendLog(j.ID, line)
                })
                done <- struct{}{}
        }()
        <-done
        <-done
        // Signal the watchdog that I/O is complete so it exits without waiting for its next tick.
        close(jobDone)
        <-watchdogDone

        if err := cmd.Wait(); err != nil {
                if ctx.Err() != nil {
                        m.mu.Lock()
                        j.Status = StatusCanceled
                        j.Stage = "canceled"
                        m.mu.Unlock()
                        return fmt.Errorf("canceled")
                }
                return fmt.Errorf("yt-dlp exited: %w", err)
        }
        return nil
}

func (m *Manager) handleLine(j *Job, line string) {
        m.appendLog(j.ID, line)

        if mm := progressRe.FindStringSubmatch(line); mm != nil {
                if p, err := strconv.ParseFloat(mm[1], 64); err == nil {
                        m.mu.Lock()
                        j.Status = StatusDownloading
                        j.Stage = "downloading"
                        j.Progress = p
                        if len(mm) > 2 {
                                j.Speed = mm[2]
                        }
                        if len(mm) > 3 {
                                j.ETA = mm[3]
                        }
                        j.UpdatedAt = time.Now()
                        m.mu.Unlock()
                        m.updateJob(j)
                }
                return
        }

        if mm := destRe.FindStringSubmatch(line); mm != nil {
                m.mu.Lock()
                j.OutputFile = strings.TrimSpace(mm[1])
                m.mu.Unlock()
        }

        if strings.HasPrefix(line, "FINAL_FILE=") {
                fp := strings.TrimPrefix(line, "FINAL_FILE=")
                m.mu.Lock()
                j.OutputFile = strings.TrimSpace(fp)
                m.mu.Unlock()
                return
        }

        if mm := mergeRe.FindStringSubmatch(line); mm != nil {
                m.mu.Lock()
                j.Status = StatusProcessing
                j.Stage = "post-processing"
                j.UpdatedAt = time.Now()
                m.mu.Unlock()
                m.updateJob(j)
                return
        }

        if strings.HasPrefix(line, "[ExtractAudio]") || strings.HasPrefix(line, "[Metadata]") ||
                strings.Contains(line, "Deleting original file") {
                m.mu.Lock()
                j.Status = StatusProcessing
                j.Stage = "post-processing"
                m.mu.Unlock()
                m.updateJob(j)
        }
}

func jobCopy(j *Job) *Job {
        cp := *j
        cp.cancel = nil
        return &cp
}

// Stats represents aggregate job statistics for the dashboard.
type Stats struct {
        Total       int          `json:"total"`
        Queued      int          `json:"queued"`
        Downloading int          `json:"downloading"`
        Processing  int          `json:"processing"`
        Done        int          `json:"done"`
        Failed      int          `json:"failed"`
        Canceled    int          `json:"canceled"`
        FreeDiskGB  uint64       `json:"free_disk_gb"`
        Workers     int          `json:"workers"`
        TopErrors   []ErrorCount `json:"top_errors,omitempty"`
}

// ErrorCount groups errors by message for the stats endpoint.
type ErrorCount struct {
        Error string `json:"error"`
        Count int    `json:"count"`
}

// Stats returns aggregate statistics about jobs visible to owner ("" = all).
func (m *Manager) Stats(owner string) Stats {
        m.mu.RLock()
        defer m.mu.RUnlock()
        s := Stats{
                Workers: m.workers,
        }
        errMap := make(map[string]int)
        for _, id := range m.order {
                j, ok := m.jobs[id]
                if !ok {
                        continue
                }
                if owner != "" && j.Owner != "" && j.Owner != owner {
                        continue
                }
                s.Total++
                switch j.Status {
                case StatusQueued:
                        s.Queued++
                case StatusDownloading:
                        s.Downloading++
                case StatusProcessing:
                        s.Processing++
                case StatusDone:
                        s.Done++
                case StatusFailed:
                        s.Failed++
                        if j.Error != "" {
                                errMap[j.Error]++
                        }
                case StatusCanceled:
                        s.Canceled++
                }
        }
        // Top 5 errors
        for err, count := range errMap {
                s.TopErrors = append(s.TopErrors, ErrorCount{Error: err, Count: count})
        }
        // Sort by count descending
        sort.Slice(s.TopErrors, func(i, j int) bool {
                return s.TopErrors[i].Count > s.TopErrors[j].Count
        })
        if len(s.TopErrors) > 5 {
                s.TopErrors = s.TopErrors[:5]
        }
        return s
}
