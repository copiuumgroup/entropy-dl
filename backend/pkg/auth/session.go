package auth

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
)

// Session config. Sessions are in-memory only: a server restart logs everyone
// out, which is the safest default for a household tool.
const (
	SessionMaxAge = 24 * time.Hour      // cookie/session lifetime
	sessionBytes  = 32                   // 256-bit random token
)

// NewSession creates a new session token, stores it in memory mapped to the
// given username, and returns the raw token string for use in a cookie.
func (s *Store) NewSession(username string) string {
	token := generateSessionToken()
	s.mu.Lock()
	s.sessions[token] = sessionEntry{
		username: username,
		expires:  time.Now().Add(SessionMaxAge),
	}
	s.mu.Unlock()
	return token
}

// VerifySession checks whether a token is valid and not expired. Returns the
// associated username.
func (s *Store) VerifySession(token string) (string, bool) {
	s.mu.RLock()
	entry, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return "", false
	}
	if time.Now().After(entry.expires) {
		// Lazily purge expired tokens.
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return "", false
	}
	return entry.username, true
}

// DeleteSession removes a session token (logout).
func (s *Store) DeleteSession(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// DeleteSessionsForUser removes all sessions for a given username (e.g. after
// password change or account deletion).
func (s *Store) DeleteSessionsForUser(username string) {
	s.mu.Lock()
	for tok, entry := range s.sessions {
		if entry.username == username {
			delete(s.sessions, tok)
		}
	}
	s.mu.Unlock()
}

// PurgeExpired removes all expired sessions. Called periodically; low urgency.
func (s *Store) PurgeExpired() int {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for tok, entry := range s.sessions {
		if now.After(entry.expires) {
			delete(s.sessions, tok)
			n++
		}
	}
	return n
}

// SessionCount returns the number of active (non-expired) sessions. Useful for
// the /api/stats endpoint and for capping concurrent sessions.
func (s *Store) SessionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	n := 0
	for _, entry := range s.sessions {
		if now.Before(entry.expires) {
			n++
		}
	}
	return n
}

func generateSessionToken() string {
	b := make([]byte, sessionBytes)
	// crypto/rand.Read returns nil on success or an error we never expect.
	// If it ever fails, we fall back to UUID v4 — still cryptographically random,
	// just shorter. Belt and suspenders.
	_, err := rand.Read(b)
	if err != nil {
		return uuid.NewString()
	}
	return base64.URLEncoding.EncodeToString(b)
}
