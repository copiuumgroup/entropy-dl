package ytdlp

import (
        "bufio"
        "context"
        "encoding/json"
        "errors"
        "fmt"
        "io"
        "log"
        "net/url"
        "os/exec"
        "strings"
        "sync"

        "entropy-gui/pkg/cmdutil"
)

// SearchResult is a flat record returned by yt-dlp --flat-playlist for search.
type SearchResult struct {
        ID        string  `json:"id"`
        Title     string  `json:"title"`
        URL       string  `json:"url"`
        Uploader  string  `json:"uploader"`
        Duration  float64 `json:"duration"`
        Thumbnail string  `json:"thumbnail"`
        Source    string  `json:"source"`
}

// Search runs yt-dlp search and returns results.
// source: "youtube" -> ytsearch, "ytmusic" -> music.youtube.com URL, "soundcloud" -> scsearch
func Search(ctx context.Context, ytdlpBin, source, query string, limit int) ([]SearchResult, error) {
        if limit <= 0 || limit > 50 {
                limit = 15
        }

        source = strings.ToLower(source)

        // Multi-search: query all supported engines concurrently and merge results
        if source == "everything" || source == "all" {
                engines := []string{"youtube", "ytmusic", "soundcloud"}
                results := make(chan []SearchResult, len(engines))
                var wg sync.WaitGroup

                // We divide the limit among engines so the total roughly equals `limit * len(engines)` or we just use `limit` for each to get a good spread.
                // Let's use `limit` for each, meaning an "everything" search returns up to limit*3 results.
                for _, eng := range engines {
                        wg.Add(1)
                        go func(e string) {
                                defer wg.Done()
                                res, err := Search(ctx, ytdlpBin, e, query, limit)
                                if err == nil {
                                        results <- res
                                } else {
                                        log.Printf("multi-search: engine %s failed: %v", e, err)
                                }
                        }(eng)
                }

                go func() {
                        wg.Wait()
                        close(results)
                }()

                var merged []SearchResult
                for res := range results {
                        merged = append(merged, res...)
                }
                
                // Shuffle or interleave could go here, but for now just appending is fine.
                // Or maybe we sort by title or just leave grouped by engine.
                return merged, nil
        }

        var searchArg string
        switch source {
        case "youtube", "yt":
                searchArg = fmt.Sprintf("ytsearch%d:%s", limit, query)
        case "ytmusic":
                // yt-dlp has no ytmsearch prefix; use the YouTube Music search URL extractor instead.
                searchArg = fmt.Sprintf("https://music.youtube.com/search?q=%s#songs", url.QueryEscape(query))

        case "soundcloud", "sc":
                searchArg = fmt.Sprintf("scsearch%d:%s", limit, query)
        default:
                return nil, fmt.Errorf("unsupported source: %s", source)
        }
        // Build the yt-dlp command. For ytmusic, use --playlist-end to cap results
        // since the URL extractor returns all songs and not just `limit` songs.
        var cmd *exec.Cmd
        if source == "ytmusic" {
                cmd = exec.CommandContext(ctx, ytdlpBin,
                        "--flat-playlist",
                        "-J",
                        "--no-warnings",
                        "--skip-download",
                        "--playlist-end", fmt.Sprintf("%d", limit),
                        searchArg,
                )
        } else {
                cmd = exec.CommandContext(ctx, ytdlpBin,
                        "--flat-playlist",
                        "-J",
                        "--no-warnings",
                        "--skip-download",
                        searchArg,
                )
        }
        cmdutil.PrepareCmd(cmd)
        out, err := cmd.Output()
        if err != nil {
                var ee *exec.ExitError
                if errors.As(err, &ee) {
                        return nil, fmt.Errorf("yt-dlp search failed: %s", string(ee.Stderr))
                }
                return nil, err
        }

        var payload struct {
                Entries []struct {
                        ID        string      `json:"id"`
                        Title     string      `json:"title"`
                        URL       string      `json:"url"`
                        Uploader  string      `json:"uploader"`
                        Channel   string      `json:"channel"`
                        Duration  float64     `json:"duration"`
                        Thumbnails []struct {
                                URL string `json:"url"`
                        } `json:"thumbnails"`
                        Thumbnail string `json:"thumbnail"`
                        IEKey     string `json:"ie_key"`
                } `json:"entries"`
        }
        if err := json.Unmarshal(out, &payload); err != nil {
                return nil, fmt.Errorf("parse yt-dlp json: %w", err)
        }

        results := make([]SearchResult, 0, len(payload.Entries))
        for _, e := range payload.Entries {
                thumb := e.Thumbnail
                if thumb == "" && len(e.Thumbnails) > 0 {
                        thumb = e.Thumbnails[len(e.Thumbnails)-1].URL
                }
                // Build canonical URL when missing
                u := e.URL
                if u == "" && e.ID != "" {
                        if source == "ytmusic" {
                                u = "https://music.youtube.com/watch?v=" + e.ID
                        } else if source == "youtube" || source == "yt" {
                                u = "https://www.youtube.com/watch?v=" + e.ID
                        }
                }
                uploader := e.Uploader
                if uploader == "" {
                        uploader = e.Channel
                }
                results = append(results, SearchResult{
                        ID:        e.ID,
                        Title:     e.Title,
                        URL:       u,
                        Uploader:  uploader,
                        Duration:  e.Duration,
                        Thumbnail: thumb,
                        Source:    source,
                })
        }
        return results, nil
}

// MetaInfo is a single track/video probe.
type MetaInfo struct {
        ID         string  `json:"id"`
        Title      string  `json:"title"`
        URL        string  `json:"webpage_url"`
        Uploader   string  `json:"uploader"`
        Channel    string  `json:"channel"`
        Duration   float64 `json:"duration"`
        Extractor  string  `json:"extractor"`
        Thumbnail  string  `json:"thumbnail"`
        PlaylistID string  `json:"playlist_id"`
        PlaylistTitle string `json:"playlist_title"`
        NEntries   int     `json:"n_entries"`
        IsPlaylist bool    `json:"-"`
}

// Probe runs yt-dlp -J on a URL and returns metadata. For playlists/albums,
// returns one MetaInfo per entry.
func Probe(ctx context.Context, ytdlpBin, urlStr string) ([]MetaInfo, error) {
        cmd := exec.CommandContext(ctx, ytdlpBin,
                "-J",
                "--no-warnings",
                "--skip-download",
                "--flat-playlist",
                urlStr,
        )
        cmdutil.PrepareCmd(cmd)
        out, err := cmd.Output()
        if err != nil {
                var ee *exec.ExitError
                if errors.As(err, &ee) {
                        return nil, fmt.Errorf("yt-dlp probe failed: %s", string(ee.Stderr))
                }
                return nil, err
        }
        var raw map[string]interface{}
        if err := json.Unmarshal(out, &raw); err != nil {
                return nil, err
        }
        if entries, ok := raw["entries"].([]interface{}); ok && len(entries) > 0 {
                results := make([]MetaInfo, 0, len(entries))
                plTitle, _ := raw["title"].(string)
                plID, _ := raw["id"].(string)
                for _, ent := range entries {
                        m, ok := ent.(map[string]interface{})
                        if !ok {
                                continue
                        }
                        info := metaFromMap(m)
                        info.PlaylistID = plID
                        info.PlaylistTitle = plTitle
                        info.IsPlaylist = true
                        if info.URL == "" {
                                if id, _ := m["id"].(string); id != "" {
                                        if ext, _ := m["ie_key"].(string); strings.EqualFold(ext, "Youtube") {
                                                info.URL = "https://www.youtube.com/watch?v=" + id
                                        }
                                }
                        }
                        results = append(results, info)
                }
                return results, nil
        }
        info := metaFromMap(raw)
        return []MetaInfo{info}, nil
}

func metaFromMap(m map[string]interface{}) MetaInfo {
        var info MetaInfo
        b, _ := json.Marshal(m)
        _ = json.Unmarshal(b, &info)
        if info.URL == "" {
                if u, ok := m["url"].(string); ok {
                        info.URL = u
                }
        }
        return info
}

// ReadLines reads from r and invokes cb for each newline-terminated chunk.
func ReadLines(r io.Reader, cb func(string)) {
        scanner := bufio.NewScanner(r)
        scanner.Buffer(make([]byte, 0, 1024*64), 1024*1024)
        for scanner.Scan() {
                cb(scanner.Text())
        }
        if err := scanner.Err(); err != nil {
                log.Printf("ytdlp read line error: %v", err)
        }
}
