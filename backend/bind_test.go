package main

import (
	"os"
	"strings"
	"testing"
)

// setEnv sets env vars for the duration of a test and restores them on cleanup.
func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	// Snapshot originals.
	orig := map[string]string{}
	for k := range kv {
		orig[k], _ = os.LookupEnv(k)
	}
	for k, v := range kv {
		if v == "" {
			os.Unsetenv(k)
		} else {
			os.Setenv(k, v)
		}
	}
	t.Cleanup(func() {
		for k, v := range orig {
			if _, ok := os.LookupEnv(k); ok && v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})
}

// TestIsLoopbackHost covers the core classifier the guard depends on. It must
// be conservative: anything we cannot positively identify as loopback counts
// as network-exposed.
func TestIsLoopbackHost(t *testing.T) {
	cases := []struct {
		name string
		host string
		want bool
	}{
		{"empty defaults to loopback", "", true},
		{"ipv4 loopback", "127.0.0.1", true},
		{"ipv6 loopback", "::1", true},
		{"localhost name", "localhost", true},
		{"localhost case-insensitive", "LOCALHOST", true},
		{"wildcard ipv4", "0.0.0.0", false},
		{"wildcard ipv6", "::", false},
		{"lan ipv4", "192.168.1.10", false},
		{"public ipv4", "8.8.8.8", false},
		{"hostname", "homeserver.local", false},
		{"other loopback 127.x", "127.1.2.3", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLoopbackHost(tc.host); got != tc.want {
				t.Errorf("isLoopbackHost(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

// TestResolveBindHost confirms the default is loopback (the out-of-the-box
// experience must not change) and that an explicit HOST is honored.
func TestResolveBindHost(t *testing.T) {
	t.Run("defaults to 127.0.0.1 when unset", func(t *testing.T) {
		setEnv(t, map[string]string{hostEnv: ""})
		if got := resolveBindHost(); got != "127.0.0.1" {
			t.Errorf("resolveBindHost() = %q, want 127.0.0.1", got)
		}
	})
	t.Run("honors explicit HOST", func(t *testing.T) {
		setEnv(t, map[string]string{hostEnv: "0.0.0.0"})
		if got := resolveBindHost(); got != "0.0.0.0" {
			t.Errorf("resolveBindHost() = %q, want 0.0.0.0", got)
		}
	})
}

// TestValidateBindConfig is the load-bearing safety property of Phase 0:
// non-loopback binds must be rejected unless BOTH TLS and auth are configured.
func TestValidateBindConfig(t *testing.T) {
	cases := []struct {
		name      string
		host      string
		useHTTPS  string // "" = unset
		adminPw   string
		wantError bool
		wantSubs  []string // substrings that must appear in the error message
	}{
		{
			name: "loopback always allowed, no flags needed",
			host: "127.0.0.1", useHTTPS: "", adminPw: "",
			wantError: false,
		},
		{
			name: "loopback localhost alias allowed",
			host: "localhost", useHTTPS: "", adminPw: "",
			wantError: false,
		},
		{
			name: "non-loopback rejected without any flags",
			host: "0.0.0.0", useHTTPS: "", adminPw: "",
			wantError: true, wantSubs: []string{"USE_HTTPS=1", "ADMIN_PASSWORD"},
		},
		{
			name: "non-loopback rejected with TLS but no auth",
			host: "0.0.0.0", useHTTPS: "1", adminPw: "",
			wantError: true, wantSubs: []string{"ADMIN_PASSWORD"},
		},
		{
			name: "non-loopback rejected with auth but no TLS",
			host: "0.0.0.0", useHTTPS: "", adminPw: "hunter2",
			wantError: true, wantSubs: []string{"USE_HTTPS=1"},
		},
		{
			name: "non-loopback allowed only with TLS AND auth",
			host: "0.0.0.0", useHTTPS: "1", adminPw: "hunter2",
			wantError: false,
		},
		{
			name: "LAN IP treated as exposed and rejected without flags",
			host: "192.168.1.10", useHTTPS: "", adminPw: "",
			wantError: true,
		},
		{
			name: "hostname treated as exposed and rejected without flags",
			host: "nas.local", useHTTPS: "", adminPw: "",
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{}
			if tc.useHTTPS != "" {
				env[useHTTPSEnv] = tc.useHTTPS
			} else {
				env[useHTTPSEnv] = ""
			}
			if tc.adminPw != "" {
				env[adminPasswordEnv] = tc.adminPw
			} else {
				env[adminPasswordEnv] = ""
			}
			setEnv(t, env)

			err := validateBindConfig(tc.host)
			if tc.wantError && err == nil {
				t.Fatalf("validateBindConfig(%q): expected error, got nil", tc.host)
			}
			if !tc.wantError && err != nil {
				t.Fatalf("validateBindConfig(%q): expected no error, got %v", tc.host, err)
			}
			if tc.wantError {
				for _, sub := range tc.wantSubs {
					if !strings.Contains(err.Error(), sub) {
						t.Errorf("error %q missing substring %q", err.Error(), sub)
					}
				}
			}
		})
	}
}
