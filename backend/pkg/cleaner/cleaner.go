package cleaner

import (
	"net/url"
	"strings"
)

// Tracking parameter prefixes/exact names to remove
var trackingParams = map[string]bool{
	"si":              true,
	"feature":         true,
	"pp":              true,
	"ab_channel":      true,
	"app":             true,
	"in":              true,
	"ref":             true,
	"referrer":        true,
	"src":             true,
	"source":          true,
	"share_source":    true,
	"share":           true,
	"fbclid":          true,
	"gclid":           true,
	"igshid":          true,
	"mc_cid":          true,
	"mc_eid":          true,
}

func isTracking(key string) bool {
	if trackingParams[key] {
		return true
	}
	lk := strings.ToLower(key)
	if strings.HasPrefix(lk, "utm_") {
		return true
	}
	if strings.HasPrefix(lk, "_") {
		return true
	}
	return false
}

// CleanURL strips tracking params from YouTube/SoundCloud URLs while
// preserving meaningful ones (v, list, t for YouTube; path for SoundCloud).
func CleanURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	host := strings.ToLower(u.Host)
	q := u.Query()
	for key := range q {
		if isTracking(key) {
			q.Del(key)
		}
	}

	// youtu.be shortlinks: turn into youtube.com/watch?v=<id>
	if host == "youtu.be" || strings.HasSuffix(host, ".youtu.be") {
		id := strings.TrimPrefix(u.Path, "/")
		if id != "" {
			nu := url.URL{Scheme: "https", Host: "www.youtube.com", Path: "/watch"}
			nq := nu.Query()
			nq.Set("v", id)
			if t := q.Get("t"); t != "" {
				nq.Set("t", t)
			}
			nu.RawQuery = nq.Encode()
			return nu.String()
		}
	}

	u.RawQuery = q.Encode()
	// Drop fragment if it's a tracking-style fragment
	u.Fragment = ""
	return u.String()
}

// CleanLines splits text by newlines/commas/spaces and cleans each URL.
func CleanLines(text string) []string {
	var out []string
	splitFn := func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ' ' || r == '\t'
	}
	for _, tok := range strings.FieldsFunc(text, splitFn) {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if !(strings.HasPrefix(tok, "http://") || strings.HasPrefix(tok, "https://")) {
			continue
		}
		out = append(out, CleanURL(tok))
	}
	return out
}
