// bind.go — Bind-address resolution and the network-exposure safety guard.
//
// Entropy is local-first: by default it binds to 127.0.0.1 and is unreachable
// from the LAN or the internet. Binding to a non-loopback address (e.g.
// HOST=0.0.0.0 or HOST=192.168.1.10) exposes the app — and everything it can
// drive (yt-dlp, ffmpeg, your filesystem) — to other machines. That is only
// ever safe behind TLS + authentication.
//
// This guard turns accidental exposure into a hard startup error rather than a
// silent footgun. It is the cornerstone of the project's future-proofing: you
// cannot accidentally ship an unauthenticated download orchestrator onto your
// network by flipping a single env var.

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

// Env var names, centralized so the guard, TLS wiring, and (later) the auth
// bootstrap all agree on the same signals.
const (
	hostEnv           = "HOST"          // interface to bind to; defaults to 127.0.0.1
	useHTTPSEnv       = "USE_HTTPS"     // set to "1" to enable TLS
	adminPasswordEnv  = "ADMIN_PASSWORD" // bootstraps the first admin user; also the "auth configured" signal
)

// resolveBindHost returns the interface to bind to. Defaults to loopback so
// the out-of-the-box experience is identical to before this feature existed.
func resolveBindHost() string {
	h := strings.TrimSpace(os.Getenv(hostEnv))
	if h == "" {
		return "127.0.0.1"
	}
	return h
}

// useHTTPS reports whether TLS is enabled via USE_HTTPS=1.
func useHTTPS() bool {
	return os.Getenv(useHTTPSEnv) == "1"
}

// authConfigured reports whether authentication has been set up. Today the only
// signal is an ADMIN_PASSWORD env var (which seeds the first admin user when
// the full auth store arrives). Once that store exists, a persisted admin user
// in the DB will also satisfy this check.
func authConfigured() bool {
	return strings.TrimSpace(os.Getenv(adminPasswordEnv)) != ""
}

// isLoopbackHost reports whether host refers to the loopback interface.
// Conservative by design: any value we cannot positively identify as loopback
// — including 0.0.0.0, ::, LAN IPs, and hostnames — is treated as exposed.
func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return true // default is loopback
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	// A non-IP hostname (e.g. "homeserver.local"). We can't guarantee it
	// resolves only to loopback, so treat it as network-exposed. Anyone who
	// genuinely wants local-only access via an alias can set HOST=127.0.0.1.
	return false
}

// validateBindConfig enforces that a non-loopback bind is permitted only when
// TLS and authentication are both enabled. Loopback binds always pass — this
// preserves the single-user desktop experience with zero friction.
func validateBindConfig(host string) error {
	if isLoopbackHost(host) {
		return nil
	}
	var problems []string
	if !useHTTPS() {
		problems = append(problems, fmt.Sprintf("%s=1 (TLS is required for a non-loopback bind)", useHTTPSEnv))
	}
	if !authConfigured() {
		problems = append(problems, fmt.Sprintf("%s=<password> (authentication is required for a non-loopback bind)", adminPasswordEnv))
	}
	if len(problems) == 0 {
		return nil
	}
	hint := ". If you only need local access, leave HOST unset (it defaults to 127.0.0.1)."
	return errors.New("refusing to bind to non-loopback address " + host + " — to expose Entropy on your network you must ALSO set: " + strings.Join(problems, "; ") + hint)
}
