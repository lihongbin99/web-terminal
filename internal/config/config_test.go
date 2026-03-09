package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte(`
server:
  port: 9090
auth:
  username: testuser
  password: testpass
  max_attempts: 3
  block_duration: 10m
terminal:
  shell: powershell.exe
`)
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Auth.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", cfg.Auth.Username)
	}
	if cfg.Auth.MaxAttempts != 3 {
		t.Errorf("expected max_attempts 3, got %d", cfg.Auth.MaxAttempts)
	}
	if cfg.Auth.BlockDuration != 10*time.Minute {
		t.Errorf("expected block_duration 10m, got %v", cfg.Auth.BlockDuration)
	}
	if cfg.Terminal.Shell != "powershell.exe" {
		t.Errorf("expected shell powershell.exe, got %s", cfg.Terminal.Shell)
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte(`
auth:
  username: admin
  password: pass
`)
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Terminal.Shell != "cmd.exe" {
		t.Errorf("expected default shell cmd.exe, got %s", cfg.Terminal.Shell)
	}
}
