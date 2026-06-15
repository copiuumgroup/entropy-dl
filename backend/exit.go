package main

import (
        "context"
        "sync/atomic"
        "time"
)

// shutdownRequested is set by handleShutdown to signal the main goroutine.
var shutdownRequested atomic.Bool

// triggerGracefulShutdown signals the main goroutine to begin graceful shutdown.
// This replaces the old os.Exit(0) approach which skipped deferred closers
// (including BoltDB.Close(), corrupting the database).
func triggerGracefulShutdown() {
        if shutdownRequested.CompareAndSwap(false, true) {
                // If the signal context hasn't been cancelled yet, cancel it to unblock <-ctx.Done()
                cancelSignal()
        }
}

// exitProc is kept for platform-specific build tags but now delegates to
// graceful shutdown instead of os.Exit.
func exitProc() { triggerGracefulShutdown() }

// shutdownAfter triggers a graceful shutdown after a small delay so the
// HTTP response has time to be flushed.
func shutdownAfter(d time.Duration) {
        go func() {
                time.Sleep(d)
                triggerGracefulShutdown()
        }()
}

// cancelSignal is set in main() to allow handleShutdown to unblock the signal waiter.
var cancelSignal context.CancelFunc