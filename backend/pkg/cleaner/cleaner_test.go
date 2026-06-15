package cleaner

import (
        "testing"
)

// ---------------------------------------------------------------------------
// CleanURL – tracking param stripping
// ---------------------------------------------------------------------------

func TestCleanURL_StripsSiParam(t *testing.T) {
        in := "https://www.youtube.com/watch?v=abc123&si=deadbeef"
        want := "https://www.youtube.com/watch?v=abc123"
        if got := CleanURL(in); got != want {
                t.Errorf("CleanURL(%q) = %q, want %q", in, got, want)
        }
}

func TestCleanURL_StripsUtmParams(t *testing.T) {
        tests := []struct {
                name string
                in   string
                want string
        }{
                {"utm_source", "https://example.com/page?utm_source=twitter&id=42", "https://example.com/page?id=42"},
                {"utm_medium", "https://example.com/page?utm_medium=email", "https://example.com/page"},
                {"utm_campaign", "https://example.com/page?utm_campaign=spring_sale", "https://example.com/page"},
                {"all_utm_combined", "https://example.com/?utm_source=fb&utm_medium=cpc&utm_campaign=launch&keep=1", "https://example.com/?keep=1"},
                {"utm_case_insensitive", "https://example.com/?UTM_Source=X&keep=yes", "https://example.com/?keep=yes"},
        }
        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        if got := CleanURL(tt.in); got != tt.want {
                                t.Errorf("CleanURL(%q) = %q, want %q", tt.in, got, tt.want)
                        }
                })
        }
}

func TestCleanURL_StripsFbclidAndGclid(t *testing.T) {
        tests := []struct {
                name string
                in   string
                want string
        }{
                {"fbclid", "https://example.com/page?fbclid=IwAR0&x=1", "https://example.com/page?x=1"},
                {"gclid", "https://example.com/page?gclid=Cjw&x=1", "https://example.com/page?x=1"},
                {"both", "https://example.com/?fbclid=a&gclid=b&keep=true", "https://example.com/?keep=true"},
        }
        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        if got := CleanURL(tt.in); got != tt.want {
                                t.Errorf("CleanURL(%q) = %q, want %q", tt.in, got, tt.want)
                        }
                })
        }
}

func TestCleanURL_StripsFeatureAndPp(t *testing.T) {
        tests := []struct {
                name string
                in   string
                want string
        }{
                {"feature", "https://www.youtube.com/watch?v=x&feature=share", "https://www.youtube.com/watch?v=x"},
                {"pp", "https://www.youtube.com/watch?v=x&pp=yg", "https://www.youtube.com/watch?v=x"},
                {"both", "https://www.youtube.com/watch?v=x&feature=share&pp=yg&t=10", "https://www.youtube.com/watch?v=x&t=10"},
        }
        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        if got := CleanURL(tt.in); got != tt.want {
                                t.Errorf("CleanURL(%q) = %q, want %q", tt.in, got, tt.want)
                        }
                })
        }
}

func TestCleanURL_StripsUnderscorePrefixedParams(t *testing.T) {
        tests := []struct {
                name string
                in   string
                want string
        }{
                {"_cb", "https://example.com/?_cb=12345&keep=me", "https://example.com/?keep=me"},
                {"_ga", "https://example.com/?_ga=UA&ref=doc", "https://example.com/"},
                {"multiple", "https://example.com/?_a=1&_b=2&safe=yes", "https://example.com/?safe=yes"},
        }
        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        if got := CleanURL(tt.in); got != tt.want {
                                t.Errorf("CleanURL(%q) = %q, want %q", tt.in, got, tt.want)
                        }
                })
        }
}

// ---------------------------------------------------------------------------
// CleanURL – preserves meaningful params
// ---------------------------------------------------------------------------

func TestCleanURL_PreservesMeaningfulYouTubeParams(t *testing.T) {
        // url.Values.Encode reorders params alphabetically.
        in := "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLabc&t=123"
        want := "https://www.youtube.com/watch?list=PLabc&t=123&v=dQw4w9WgXcQ"
        if got := CleanURL(in); got != want {
                t.Errorf("CleanURL(%q) = %q, want %q", in, got, want)
        }
}

func TestCleanURL_DoesNotStripVListTIndex(t *testing.T) {
        tests := []struct {
                name string
                in   string
                want string
        }{
                {"v param", "https://www.youtube.com/watch?v=xyz", "https://www.youtube.com/watch?v=xyz"},
                {"list param", "https://www.youtube.com/watch?v=xyz&list=PL123", "https://www.youtube.com/watch?list=PL123&v=xyz"},
                {"t param", "https://www.youtube.com/watch?v=xyz&t=30s", "https://www.youtube.com/watch?t=30s&v=xyz"},
                {"index param", "https://www.youtube.com/watch?v=xyz&index=5", "https://www.youtube.com/watch?index=5&v=xyz"},
        }
        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        if got := CleanURL(tt.in); got != tt.want {
                                t.Errorf("CleanURL(%q) = %q, want %q", tt.in, got, tt.want)
                        }
                })
        }
}

// ---------------------------------------------------------------------------
// CleanURL – youtu.be expansion
// ---------------------------------------------------------------------------

func TestCleanURL_ExpandYoutuBe(t *testing.T) {
        in := "https://youtu.be/dQw4w9WgXcQ"
        want := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
        if got := CleanURL(in); got != want {
                t.Errorf("CleanURL(%q) = %q, want %q", in, got, want)
        }
}

func TestCleanURL_ExpandYoutuBeWithTracking(t *testing.T) {
        in := "https://youtu.be/dQw4w9WgXcQ?si=tracking123"
        // si is stripped; the remaining query has no keys, so only v is set.
        want := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
        if got := CleanURL(in); got != want {
                t.Errorf("CleanURL(%q) = %q, want %q", in, got, want)
        }
}

func TestCleanURL_ExpandYoutuBePreservesT(t *testing.T) {
        in := "https://youtu.be/dQw4w9WgXcQ?t=30"
        want := "https://www.youtube.com/watch?t=30&v=dQw4w9WgXcQ"
        if got := CleanURL(in); got != want {
                t.Errorf("CleanURL(%q) = %q, want %q", in, got, want)
        }
}

func TestCleanURL_ExpandYoutuBeSubdomain(t *testing.T) {
        tests := []struct {
                name string
                in   string
                want string
        }{
                {"music subdomain", "https://music.youtu.be/dQw4w9WgXcQ", "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
                {"www subdomain", "https://www.youtu.be/dQw4w9WgXcQ", "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
        }
        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        if got := CleanURL(tt.in); got != tt.want {
                                t.Errorf("CleanURL(%q) = %q, want %q", tt.in, got, tt.want)
                        }
                })
        }
}

// ---------------------------------------------------------------------------
// CleanURL – fragment removal
// ---------------------------------------------------------------------------

func TestCleanURL_RemovesFragment(t *testing.T) {
        in := "https://example.com/page#section"
        want := "https://example.com/page"
        if got := CleanURL(in); got != want {
                t.Errorf("CleanURL(%q) = %q, want %q", in, got, want)
        }
}

func TestCleanURL_RemovesFragmentWithTracking(t *testing.T) {
        in := "https://example.com/page?fbclid=x#here"
        want := "https://example.com/page"
        if got := CleanURL(in); got != want {
                t.Errorf("CleanURL(%q) = %q, want %q", in, got, want)
        }
}

// ---------------------------------------------------------------------------
// CleanURL – edge cases
// ---------------------------------------------------------------------------

func TestCleanURL_EmptyString(t *testing.T) {
        if got := CleanURL(""); got != "" {
                t.Errorf("CleanURL(%q) = %q, want empty string", "", got)
        }
}

func TestCleanURL_WhitespaceTrimmed(t *testing.T) {
        in := "  https://example.com/page  "
        want := "https://example.com/page"
        if got := CleanURL(in); got != want {
                t.Errorf("CleanURL(%q) = %q, want %q", in, got, want)
        }
}

func TestCleanURL_WhitespaceOnly(t *testing.T) {
        if got := CleanURL("   \t\n  "); got != "" {
                t.Errorf("CleanURL(whitespace) = %q, want empty string", got)
        }
}

func TestCleanURL_UnparseableURL(t *testing.T) {
        // Something that looks nothing like a URL – url.Parse won't error on
        // arbitrary strings (they become opaque), so we test a genuinely
        // problematic case. In Go, url.Parse almost never returns an error,
        // but if the input is completely blank after trim we already handle that.
        // Use an unparseable scheme to verify passthrough behaviour.
        in := "://bad-url"
        got := CleanURL(in)
        if got != in {
                t.Errorf("CleanURL(%q) = %q, want original %q", in, got, in)
        }
}

// ---------------------------------------------------------------------------
// CleanURL – isTracking coverage (via exported CleanURL)
// ---------------------------------------------------------------------------

func TestCleanURL_AllTrackingParamsStripped(t *testing.T) {
        // Build a URL with every known tracking param; only "keep" should survive.
        params := []string{
                "si", "feature", "pp", "ab_channel", "app", "in", "ref",
                "referrer", "src", "source", "share_source", "share",
                "fbclid", "gclid", "igshid", "mc_cid", "mc_eid",
                "utm_foo", "_cb", "_ga",
        }
        qs := "keep=yes"
        for _, p := range params {
                qs += "&" + p + "=val"
        }
        in := "https://example.com/?" + qs
        want := "https://example.com/?keep=yes"
        if got := CleanURL(in); got != want {
                t.Errorf("CleanURL(%q) = %q, want %q", in, got, want)
        }
}

// ---------------------------------------------------------------------------
// CleanLines – splitting
// ---------------------------------------------------------------------------

func TestCleanLines_NewlineSplit(t *testing.T) {
        in := "https://a.com/1\nhttps://b.com/2\nhttps://c.com/3"
        want := []string{
                "https://a.com/1",
                "https://b.com/2",
                "https://c.com/3",
        }
        got := CleanLines(in)
        assertStrings(t, got, want)
}

func TestCleanLines_CommaSplit(t *testing.T) {
        in := "https://a.com/1,https://b.com/2,https://c.com/3"
        want := []string{
                "https://a.com/1",
                "https://b.com/2",
                "https://c.com/3",
        }
        got := CleanLines(in)
        assertStrings(t, got, want)
}

func TestCleanLines_SpaceSplit(t *testing.T) {
        in := "https://a.com/1 https://b.com/2 https://c.com/3"
        want := []string{
                "https://a.com/1",
                "https://b.com/2",
                "https://c.com/3",
        }
        got := CleanLines(in)
        assertStrings(t, got, want)
}

func TestCleanLines_TabSplit(t *testing.T) {
        in := "https://a.com/1\thttps://b.com/2\thttps://c.com/3"
        want := []string{
                "https://a.com/1",
                "https://b.com/2",
                "https://c.com/3",
        }
        got := CleanLines(in)
        assertStrings(t, got, want)
}

func TestCleanLines_CarriageReturnSplit(t *testing.T) {
        in := "https://a.com/1\rhttps://b.com/2"
        want := []string{
                "https://a.com/1",
                "https://b.com/2",
        }
        got := CleanLines(in)
        assertStrings(t, got, want)
}

func TestCleanLines_IgnoresNonURLs(t *testing.T) {
        in := "check this out https://a.com/1 and also visit ftp://b.com/2"
        want := []string{"https://a.com/1"}
        got := CleanLines(in)
        assertStrings(t, got, want)
}

func TestCleanLines_MixedTextWithURLs(t *testing.T) {
        in := "Hello\nhttps://youtube.com/watch?v=abc&si=x\nWorld\nhttps://youtu.be/def"
        want := []string{
                "https://youtube.com/watch?v=abc",
                "https://www.youtube.com/watch?v=def",
        }
        got := CleanLines(in)
        assertStrings(t, got, want)
}

func TestCleanLines_EmptyInput(t *testing.T) {
        got := CleanLines("")
        if len(got) != 0 {
                t.Errorf("CleanLines(%q) = %v, want empty slice", "", got)
        }
}

func TestCleanLines_SingleURL(t *testing.T) {
        in := "https://example.com/page"
        want := []string{"https://example.com/page"}
        got := CleanLines(in)
        assertStrings(t, got, want)
}

func TestCleanLines_CleansEachURL(t *testing.T) {
        // Ensure CleanLines actually calls CleanURL on each token.
        in := "https://www.youtube.com/watch?v=x&si=y, https://youtu.be/z?si=w"
        want := []string{
                "https://www.youtube.com/watch?v=x",
                "https://www.youtube.com/watch?v=z",
        }
        got := CleanLines(in)
        assertStrings(t, got, want)
}

func TestCleanLines_HandlesMultipleDelimiters(t *testing.T) {
        // Newlines, commas, spaces and tabs all mixed together.
        in := "https://a.com/1, https://b.com/2\nhttps://c.com/3\thttps://d.com/4"
        want := []string{
                "https://a.com/1",
                "https://b.com/2",
                "https://c.com/3",
                "https://d.com/4",
        }
        got := CleanLines(in)
        assertStrings(t, got, want)
}

func TestCleanLines_SkipsBlankTokens(t *testing.T) {
        in := "https://a.com/1,,  ,,https://b.com/2"
        want := []string{
                "https://a.com/1",
                "https://b.com/2",
        }
        got := CleanLines(in)
        assertStrings(t, got, want)
}

// ---------------------------------------------------------------------------
// isTracking – direct unexported tests (same package)
// ---------------------------------------------------------------------------

func TestIsTracking_ExactMatch(t *testing.T) {
        for key := range trackingParams {
                if !isTracking(key) {
                        t.Errorf("isTracking(%q) = false, want true", key)
                }
        }
}

func TestIsTracking_UtmPrefix(t *testing.T) {
        prefixes := []string{"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term", "UTM_SOURCE"}
        for _, key := range prefixes {
                if !isTracking(key) {
                        t.Errorf("isTracking(%q) = false, want true", key)
                }
        }
}

func TestIsTracking_UnderscorePrefix(t *testing.T) {
        keys := []string{"_cb", "_ga", "_gl", "_hsenc", "_hsmi", "__a"}
        for _, key := range keys {
                if !isTracking(key) {
                        t.Errorf("isTracking(%q) = false, want true", key)
                }
        }
}

func TestIsTracking_NonTracking(t *testing.T) {
        keys := []string{"v", "list", "t", "index", "id", "page", "q", "search", "lang"}
        for _, key := range keys {
                if isTracking(key) {
                        t.Errorf("isTracking(%q) = true, want false", key)
                }
        }
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertStrings(t *testing.T, got, want []string) {
        t.Helper()
        if len(got) != len(want) {
                t.Fatalf("length mismatch: got %d items %v, want %d items %v", len(got), got, len(want), want)
        }
        for i := range got {
                if got[i] != want[i] {
                        t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
                }
        }
}
