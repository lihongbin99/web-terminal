package auth

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"web-terminal/internal/config"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type AuthService struct {
	cfg     *config.AuthConfig
	db      *sql.DB
	tokens  map[string]bool // active session tokens
	tokenMu sync.RWMutex
}

func New(cfg *config.AuthConfig, dbPath string) (*AuthService, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := initDB(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("init database: %w", err)
	}

	return &AuthService{
		cfg:    cfg,
		db:     db,
		tokens: make(map[string]bool),
	}, nil
}

func initDB(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS login_failures (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip TEXT NOT NULL,
			attempted_at DATETIME NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE IF NOT EXISTS blocked_ips (
			ip TEXT PRIMARY KEY,
			blocked_at DATETIME NOT NULL DEFAULT (datetime('now')),
			blocked_until DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_login_failures_ip ON login_failures(ip, attempted_at);
	`)
	return err
}

func (s *AuthService) IsIPBlocked(ip string) (bool, error) {
	var blockedUntil time.Time
	err := s.db.QueryRow(
		"SELECT blocked_until FROM blocked_ips WHERE ip = ?", ip,
	).Scan(&blockedUntil)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if time.Now().After(blockedUntil) {
		s.db.Exec("DELETE FROM blocked_ips WHERE ip = ?", ip)
		return false, nil
	}
	return true, nil
}

func (s *AuthService) RecordFailure(ip string) error {
	_, err := s.db.Exec(
		"INSERT INTO login_failures (ip, attempted_at) VALUES (?, datetime('now'))", ip,
	)
	if err != nil {
		return err
	}

	var count int
	windowStart := time.Now().Add(-5 * time.Minute).UTC().Format("2006-01-02 15:04:05")
	err = s.db.QueryRow(
		"SELECT COUNT(*) FROM login_failures WHERE ip = ? AND attempted_at >= ?",
		ip, windowStart,
	).Scan(&count)
	if err != nil {
		return err
	}

	if count >= s.cfg.MaxAttempts {
		blockedUntil := time.Now().Add(s.cfg.BlockDuration).UTC().Format("2006-01-02 15:04:05")
		_, err = s.db.Exec(
			`INSERT OR REPLACE INTO blocked_ips (ip, blocked_at, blocked_until)
			 VALUES (?, datetime('now'), ?)`, ip, blockedUntil,
		)
		if err != nil {
			return err
		}
		s.db.Exec("DELETE FROM login_failures WHERE ip = ?", ip)
	}

	return nil
}

func (s *AuthService) Login(username, password, ip string) (string, error) {
	blocked, err := s.IsIPBlocked(ip)
	if err != nil {
		return "", fmt.Errorf("check blocked: %w", err)
	}
	if blocked {
		return "", fmt.Errorf("ip is blocked")
	}

	if username != s.cfg.Username || password != s.cfg.Password {
		s.RecordFailure(ip)
		return "", fmt.Errorf("invalid credentials")
	}

	token := uuid.New().String()
	s.tokenMu.Lock()
	s.tokens[token] = true
	s.tokenMu.Unlock()

	s.db.Exec("DELETE FROM login_failures WHERE ip = ?", ip)

	return token, nil
}

func (s *AuthService) ValidateToken(token string) bool {
	s.tokenMu.RLock()
	defer s.tokenMu.RUnlock()
	return s.tokens[token]
}

func (s *AuthService) Close() error {
	return s.db.Close()
}
