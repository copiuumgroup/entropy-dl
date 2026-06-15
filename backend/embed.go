package main

import (
        "embed"
        "io/fs"
)

// Embedded frontend build. The build.bat script copies frontend\build into
// backend\webdist before running `go build`, so the entire React app gets
// baked into the final .exe — distributing a single file.
//
// A placeholder index.html ships in source so `go build` works even when no
// frontend has been built yet (dev mode).
//
//go:embed all:webdist
var webdistFS embed.FS

// embeddedWeb returns the embedded React build as an http.FileSystem-ready fs.FS,
// or nil if only the placeholder is present.
func embeddedWeb() fs.FS {
        sub, err := fs.Sub(webdistFS, "webdist")
        if err != nil {
                return nil
        }
        // Heuristic: real build has an assets/ folder with JS bundles; placeholder doesn't.
        assetsDir, err := fs.Stat(sub, "assets")
        if err != nil || !assetsDir.IsDir() {
                return nil
        }
        return sub
}
