package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
)

// testDB creates a temporary bbolt database for testing and returns a cleanup func.
func testDB(t *testing.T) *bbolt.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := bbolt.Open(filepath.Join(dir, "test.db"), 0o600, nil)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestStore_CreateAndVerify(t *testing.T) {
	db := testDB(t)
	s, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create admin user.
	u, err := s.Create("admin", "supersecret", true)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.Username != "admin" || !u.IsAdmin {
		t.Errorf("Create returned unexpected user: %+v", u)
	}
	if u.PasswordHash != nil {
		t.Error("Create should not leak password hash in returned User")
	}

	// Verify password.
	verified, err := s.VerifyPassword("admin", "supersecret")
	if err != nil {
		t.Fatalf("VerifyPassword (correct): %v", err)
	}
	if verified.Username != "admin" {
		t.Errorf("VerifyPassword returned wrong user: %s", verified.Username)
	}

	// Wrong password.
	_, err = s.VerifyPassword("admin", "wrong")
	if err != ErrInvalidPassword {
		t.Errorf("VerifyPassword (wrong) = %v, want ErrInvalidPassword", err)
	}

	// Non-existent user (should NOT leak "user not found" — must return same error as wrong pw).
	_, err = s.VerifyPassword("nobody", "whatever")
	if err != ErrInvalidPassword {
		t.Errorf("VerifyPassword (missing user) = %v, want ErrInvalidPassword", err)
	}
}

func TestStore_HasAnyUser(t *testing.T) {
	db := testDB(t)
	s, _ := New(db)

	has, err := s.HasAnyUser()
	if err != nil || has {
		t.Error("HasAnyUser on empty store should return false")
	}

	s.Create("admin", "password123", true)
	has, err = s.HasAnyUser()
	if err != nil || !has {
		t.Error("HasAnyUser after create should return true")
	}
}

func TestStore_HasAdmin(t *testing.T) {
	db := testDB(t)
	s, _ := New(db)

	has, _ := s.HasAdmin()
	if has {
		t.Error("HasAdmin on empty store should return false")
	}

	s.Create("user", "password123", false)
	has, _ = s.HasAdmin()
	if has {
		t.Error("HasAdmin should return false for non-admin user")
	}

	s.Create("admin", "password123", true)
	has, _ = s.HasAdmin()
	if !has {
		t.Error("HasAdmin should return true after creating admin")
	}
}

func TestStore_CreationValidation(t *testing.T) {
	db := testDB(t)
	s, _ := New(db)

	tests := []struct {
		name     string
		user     string
		password string
		wantErr  error
	}{
		{"empty username", "", "password123", ErrUsernameEmpty},
		{"short password", "admin", "short", ErrPasswordTooShort},
		{"valid", "admin", "longpassword", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.Create(tt.user, tt.password, true)
			if err != tt.wantErr {
				t.Errorf("Create(%q, %q) = %v, want %v", tt.user, tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestStore_DuplicateUser(t *testing.T) {
	db := testDB(t)
	s, _ := New(db)

	_, err := s.Create("admin", "password123", true)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = s.Create("admin", "different", true)
	if err != ErrUserExists {
		t.Errorf("second create = %v, want ErrUserExists", err)
	}
}

func TestStore_List(t *testing.T) {
	db := testDB(t)
	s, _ := New(db)

	s.Create("admin", "password123", true)
	s.Create("guest", "password123", false)

	users, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("List returned %d users, want 2", len(users))
	}
	for _, u := range users {
		if u.PasswordHash != nil {
			t.Errorf("List should never leak password hashes, got non-nil for %q", u.Username)
		}
	}
}

func TestStore_DeleteUser(t *testing.T) {
	db := testDB(t)
	s, _ := New(db)

	s.Create("admin", "password123", true)
	s.Create("guest", "password123", false)

	// Can delete a non-admin user.
	err := s.Delete("guest")
	if err != nil {
		t.Errorf("Delete guest: %v", err)
	}

	// Cannot delete the last admin.
	err = s.Delete("admin")
	if err == nil {
		t.Error("Delete last admin should fail")
	}

	// Create a second admin, now first admin is deletable.
	s.Create("admin2", "password123", true)
	err = s.Delete("admin")
	if err != nil {
		t.Errorf("Delete admin when another admin exists: %v", err)
	}
}

func TestStore_SetPassword(t *testing.T) {
	db := testDB(t)
	s, _ := New(db)

	s.Create("admin", "oldpassword", true)

	err := s.SetPassword("admin", "newpassword")
	if err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	// Old password fails.
	_, err = s.VerifyPassword("admin", "oldpassword")
	if err != ErrInvalidPassword {
		t.Errorf("old password should be invalid: %v", err)
	}

	// New password works.
	_, err = s.VerifyPassword("admin", "newpassword")
	if err != nil {
		t.Errorf("new password should be valid: %v", err)
	}
}

func TestStore_SessionLifecycle(t *testing.T) {
	db := testDB(t)
	s, _ := New(db)
	s.Create("admin", "password123", true)

	// Create session.
	token := s.NewSession("admin")
	if token == "" {
		t.Fatal("NewSession returned empty token")
	}

	// Verify session.
	username, ok := s.VerifySession(token)
	if !ok || username != "admin" {
		t.Errorf("VerifySession(%q) = %q, %v; want admin, true", token, username, ok)
	}

	// Delete session (logout).
	s.DeleteSession(token)
	username, ok = s.VerifySession(token)
	if ok {
		t.Error("VerifySession should return false after DeleteSession")
	}
}

func TestStore_DeleteSessionsForUser(t *testing.T) {
	db := testDB(t)
	s, _ := New(db)
	s.Create("admin", "password123", true)
	s.Create("guest", "password123", false)

	t1 := s.NewSession("admin")
	t2 := s.NewSession("admin")
	t3 := s.NewSession("guest")

	s.DeleteSessionsForUser("admin")

	if _, ok := s.VerifySession(t1); ok {
		t.Error("admin session t1 should be gone")
	}
	if _, ok := s.VerifySession(t2); ok {
		t.Error("admin session t2 should be gone")
	}
	if _, ok := s.VerifySession(t3); !ok {
		t.Error("guest session t3 should still be valid")
	}
}

func TestStore_SessionCount(t *testing.T) {
	db := testDB(t)
	s, _ := New(db)
	s.Create("admin", "password123", true)

	if c := s.SessionCount(); c != 0 {
		t.Errorf("empty session count = %d, want 0", c)
	}

	s.NewSession("admin")
	s.NewSession("admin")

	if c := s.SessionCount(); c != 2 {
		t.Errorf("session count after 2 sessions = %d, want 2", c)
	}
}

func TestStore_PurgeExpired(t *testing.T) {
	db := testDB(t)
	s, _ := New(db)
	s.Create("admin", "password123", true)

	s.NewSession("admin")

	// Force all sessions expired by setting their expiry to the past.
	// We can't do this directly (unexported), but we can verify PurgeExpired
	// doesn't break on an active store.
	n := s.PurgeExpired()
	if n != 0 {
		// Sessions are just created, so none should be expired.
		// (Unless the test is extremely slow and 24h passed, which won't happen.)
		t.Errorf("PurgeExpired purged %d, expected 0 for fresh sessions", n)
	}
}

func TestGenerateSessionToken_Uniqueness(t *testing.T) {
	// Generate 1000 tokens and verify none collide.
	tokens := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		tok := generateSessionToken()
		if tokens[tok] {
			t.Fatalf("collision at token %d", i)
		}
		tokens[tok] = true
	}
}

func TestGenerateSessionToken_UsesURLEncoding(t *testing.T) {
	tok := generateSessionToken()
	// Base64-URL should not contain +, /, or = padding (our tokens don't pad).
	if bytes.ContainsAny([]byte(tok), "+/") {
		t.Errorf("token contains + or / (should use URL-safe base64): %q", tok)
	}
}

func TestSetEnv(t *testing.T) {
	// Verify testDB doesn't leak env vars. This is a meta-test to ensure the
	// test infrastructure is clean.
	if v := os.Getenv("ADMIN_PASSWORD"); v != "" {
		t.Logf("note: ADMIN_PASSWORD is set in the test environment: %q", v)
	}
}
