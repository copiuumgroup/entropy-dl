package auth

import (
	"context"
	"net/http"
)

// contextKey is an unexported type to prevent collisions with other packages
// that might also use context values. The value is always *User.
type contextKey struct{}

// FromRequest extracts the authenticated user from the request context.
// Returns the zero User if no user is present (e.g. loopback passthrough or
// an unauthenticated path that bypassed withAuth).
func FromRequest(r *http.Request) *User {
	u, _ := r.Context().Value(contextKey{}).(*User)
	if u == nil {
		return nil
	}
	return u
}

// WithUser injects a user into the request context. Used by loopback passthrough
// to provide a synthetic admin user without needing a cookie.
func WithUser(r *http.Request, u *User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), contextKey{}, u))
}

// AuthMiddleware is the HTTP middleware factory. It creates a closure that:
//   - On exempt paths (health, static assets, SSE when loopback): passes through.
//   - On /api/login /api/setup: passes through (these must be reachable to
//     establish a session in the first place).
//   - On all other /api/* paths: requires a valid session cookie, populates
//     the request context with the authenticated User, and returns 401 on
//     missing/invalid/expired sessions.
func AuthMiddleware(s *Store, cookieName string, exemptPaths map[string]bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Exempt paths — no auth required.
			if exemptPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// For /api/* endpoints that require auth, read the session cookie.
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			username, ok := s.VerifySession(cookie.Value)
			if !ok {
				// Cookie present but invalid/expired.
				http.Error(w, "session expired", http.StatusUnauthorized)
				return
			}

			// Load the full user record (for IsAdmin).
			user, err := s.Get(username)
			if err != nil {
				// User was deleted while session was active.
				s.DeleteSession(cookie.Value)
				http.Error(w, "user no longer exists", http.StatusUnauthorized)
				return
			}

			// Inject into context so handlers can read the user.
			ctx := context.WithValue(r.Context(), contextKey{}, &user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminOnly is a middleware that requires the authenticated user to have
// IsAdmin=true. It should be layered *on top of* AuthMiddleware — the user
// is already in the context at this point.
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := FromRequest(r)
		if u == nil || !u.IsAdmin {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
