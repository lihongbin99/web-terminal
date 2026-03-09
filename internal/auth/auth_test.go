package auth

import (
	"path/filepath"
	"testing"
	"time"

	"web-terminal/internal/config"
)

func newTestAuth(t *testing.T) *AuthService {
	t.Helper()
	cfg := &config.AuthConfig{
		Username:      "admin",
		Password:      "secret",
		MaxAttempts:   3,
		BlockDuration: 1 * time.Minute,
	}
	dbPath := filepath.Join(t.TempDir(), "test.db")
	svc, err := New(cfg, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })
	return svc
}

func TestLoginSuccess(t *testing.T) {
	svc := newTestAuth(t)
	token, err := svc.Login("admin", "secret", "127.0.0.1")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if !svc.ValidateToken(token) {
		t.Fatal("token should be valid")
	}
}

func TestLoginFailure(t *testing.T) {
	svc := newTestAuth(t)
	_, err := svc.Login("admin", "wrong", "127.0.0.1")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestBruteForceBlocking(t *testing.T) {
	svc := newTestAuth(t)
	ip := "192.168.1.100"

	for i := 0; i < 3; i++ {
		svc.Login("admin", "wrong", ip)
	}

	blocked, err := svc.IsIPBlocked(ip)
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Fatal("IP should be blocked after max attempts")
	}

	_, err = svc.Login("admin", "secret", ip)
	if err == nil {
		t.Fatal("expected error for blocked IP")
	}
}

func TestTokenValidation(t *testing.T) {
	svc := newTestAuth(t)
	if svc.ValidateToken("nonexistent") {
		t.Fatal("random token should not be valid")
	}
}
