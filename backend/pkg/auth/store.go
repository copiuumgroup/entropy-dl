// Package auth implements named-account authentication for Entropy's
// network-exposed (homelab) mode.
//
// Design notes:
//   - Users live in the same entropy.db as jobs/settings, in a dedicated bbolt
//     bucket. This keeps the single-file state story intact.
//   - Passwords are hashed with bcrypt (cost 12). Plaintext is never stored.
//   - Sessions are in-memory only: a random token -> username map with an
//     expiry. A server restart logs everyone out, which is the safest default
//     for a household tool. Persistence of sessions would be a future feature.
//   - The package is unaware of loopback: the "skip auth on localhost" policy
//     lives in the HTTP layer (main.go middleware), not here.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

// Standard errors. Use errors.Is to check.
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("user already exists")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrNoUsers           = errors.New("no users have been set up")
	ErrSessionNotFound   = errors.New("session not found or expired")
	ErrUsernameEmpty     = errors.New("username must not be empty")
	ErrPasswordTooShort  = errors.New("password must be at least 8 characters")
)

// MinPasswordLength is the minimum acceptable password length.
const MinPasswordLength = 8

// User is a single named account.
type User struct {
	Username    string    `json:"username"`
	PasswordHash []byte    `json:"password_hash"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
}

// Store persists users to bbolt and tracks sessions in memory.
type Store struct {
	db       *bbolt.DB
	mu       sync.RWMutex // guards the sessions map
	sessions map[string]sessionEntry
}

type sessionEntry struct {
	username string
	expires  time.Time
}

var bucketUsers = []byte("auth_users")

// New wraps an already-open bbolt DB and ensures the users bucket exists.
// It does NOT take ownership of closing the DB — the caller (main.go) owns that.
func New(db *bbolt.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("auth: nil db")
	}
	err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketUsers)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("auth: create bucket: %w", err)
	}
	return &Store{db: db, sessions: map[string]sessionEntry{}}, nil
}

// HasAnyUser reports whether at least one user exists. Drives the first-run
// "setup mode": while false, only /api/setup is reachable.
func (s *Store) HasAnyUser() (bool, error) {
	var count int
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			count++
			if count > 0 {
				return nil // one is enough
			}
		}
		return nil
	})
	return count > 0, err
}

// HasAdmin reports whether at least one admin user exists.
func (s *Store) HasAdmin() (bool, error) {
	var found bool
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, v []byte) error {
			if found {
				return nil
			}
			var u User
			if err := json.Unmarshal(v, &u); err == nil && u.IsAdmin {
				found = true
			}
			return nil
		})
	})
	return found, err
}

// Get returns a user by username (case-sensitive).
func (s *Store) Get(username string) (User, error) {
	var u User
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return ErrUserNotFound
		}
		v := b.Get([]byte(username))
		if len(v) == 0 {
			return ErrUserNotFound
		}
		return json.Unmarshal(v, &u)
	})
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// List returns every user (without password hashes — they're cleared for safety).
func (s *Store) List() ([]User, error) {
	var users []User
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, v []byte) error {
			var u User
			if err := json.Unmarshal(v, &u); err != nil {
				return nil // skip corrupt entries
			}
			u.PasswordHash = nil // never leak hashes to the API
			users = append(users, u)
			return nil
		})
	})
	return users, err
}

// Create adds a new user. Returns ErrUserExists on conflict. The caller decides
// IsAdmin. Passwords are hashed here; never pass a pre-hashed value.
func (s *Store) Create(username, password string, isAdmin bool) (User, error) {
	if err := validateCredentials(username, password); err != nil {
		return User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return User{}, fmt.Errorf("auth: hash password: %w", err)
	}
	u := User{
		Username:    username,
		PasswordHash: hash,
		IsAdmin:     isAdmin,
		CreatedAt:   time.Now().UTC(),
	}
	data, err := json.Marshal(u)
	if err != nil {
		return User{}, err
	}
	err = s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return errors.New("auth: users bucket missing")
		}
		if len(b.Get([]byte(username))) > 0 {
			return ErrUserExists
		}
		return b.Put([]byte(username), data)
	})
	if err != nil {
		return User{}, err
	}
	u.PasswordHash = nil
	return u, nil
}

// SetPassword updates an existing user's password.
func (s *Store) SetPassword(username, password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return ErrUserNotFound
		}
		v := b.Get([]byte(username))
		if len(v) == 0 {
			return ErrUserNotFound
		}
		var u User
		if err := json.Unmarshal(v, &u); err != nil {
			return err
		}
		u.PasswordHash = hash
		data, err := json.Marshal(u)
		if err != nil {
			return err
		}
		return b.Put([]byte(username), data)
	})
}

// Delete removes a user. Returns ErrUserNotFound if absent. Refuses to remove
// the last admin (so the box can never be locked out of admin access).
func (s *Store) Delete(username string) error {
	u, err := s.Get(username)
	if err != nil {
		return err
	}
	if u.IsAdmin {
		ok, err := s.HasAdmin()
		if err != nil {
			return err
		}
		// Count admins to ensure we're not removing the last one.
		admins, aErr := s.adminCount()
		if aErr != nil {
			return aErr
		}
		if ok && admins <= 1 {
			return errors.New("cannot delete the last admin user")
		}
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return ErrUserNotFound
		}
		return b.Delete([]byte(username))
	})
}

func (s *Store) adminCount() (int, error) {
	var n int
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, v []byte) error {
			var u User
			if err := json.Unmarshal(v, &u); err == nil && u.IsAdmin {
				n++
			}
			return nil
		})
	})
	return n, err
}

// VerifyPassword checks a plaintext password against the stored hash. Constant
// time on success/failure via bcrypt.CompareHashAndPassword.
func (s *Store) VerifyPassword(username, password string) (User, error) {
	u, err := s.Get(username)
	if err != nil {
		return User{}, ErrInvalidPassword // don't leak "user not found" vs "wrong pw"
	}
	if err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password)); err != nil {
		return User{}, ErrInvalidPassword
	}
	return u, nil
}

func validateCredentials(username, password string) error {
	if username == "" {
		return ErrUsernameEmpty
	}
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}
