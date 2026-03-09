# Web Terminal Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a web-based terminal that runs Windows CMD via ConPTY, accessible from a browser with authentication and brute-force protection.

**Architecture:** Go HTTP server serves embedded static files (xterm.js frontend). Login via POST returns a UUID session token. WebSocket endpoint bridges xterm.js to a ConPTY-attached cmd.exe process. SQLite stores login failures and blocked IPs.

**Tech Stack:** Go 1.25, ConPTY (`github.com/UserExistsError/conpty`), WebSocket (`github.com/gorilla/websocket`), SQLite (`github.com/mattn/go-sqlite3`), YAML config (`gopkg.in/yaml.v3`), xterm.js 5.x (CDN)

**Design doc:** `docs/plans/2026-03-09-web-terminal-design.md`

---

### Task 1: Project Scaffolding & Config

**Files:**
- Create: `config.yaml`
- Create: `internal/config/config.go`

**Step 1: Create config.yaml**

```yaml
server:
  port: 8080
auth:
  username: admin
  password: changeme
  max_attempts: 5
  block_duration: 30m
terminal:
  shell: cmd.exe
```

**Step 2: Create config loader**

Create `internal/config/config.go`:

```go
package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Auth     AuthConfig     `yaml:"auth"`
	Terminal TerminalConfig `yaml:"terminal"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type AuthConfig struct {
	Username      string        `yaml:"username"`
	Password      string        `yaml:"password"`
	MaxAttempts   int           `yaml:"max_attempts"`
	BlockDuration time.Duration `yaml:"block_duration"`
}

type TerminalConfig struct {
	Shell string `yaml:"shell"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Auth.MaxAttempts == 0 {
		cfg.Auth.MaxAttempts = 5
	}
	if cfg.Auth.BlockDuration == 0 {
		cfg.Auth.BlockDuration = 30 * time.Minute
	}
	if cfg.Terminal.Shell == "" {
		cfg.Terminal.Shell = "cmd.exe"
	}
	return &cfg, nil
}
```

**Step 3: Install dependencies**

```bash
cd D:/Code/HongBin/Go/web-terminal
go get gopkg.in/yaml.v3@latest
go mod tidy
```

**Step 4: Write test for config loading**

Create `internal/config/config_test.go`:

```go
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
```

**Step 5: Run tests**

```bash
cd D:/Code/HongBin/Go/web-terminal
go test ./internal/config/ -v
```

Expected: PASS

**Step 6: Commit**

```bash
git init
git add .
git commit -m "feat: project scaffolding with config loader"
```

---

### Task 2: SQLite Database & Auth (Brute-Force Protection)

**Files:**
- Create: `internal/auth/auth.go`
- Create: `internal/auth/auth_test.go`

**Step 1: Install SQLite dependency**

```bash
cd D:/Code/HongBin/Go/web-terminal
go get github.com/mattn/go-sqlite3@latest
go mod tidy
```

Note: `go-sqlite3` requires CGO. Make sure GCC is available (`gcc --version`). If not available on Windows, use `github.com/glebarez/go-sqlite` (pure Go) as alternative.

**Step 2: Create auth module**

Create `internal/auth/auth.go`:

```go
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
		// Block expired, remove it
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

	// Count recent failures
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
		// Clean up old failure records for this IP
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

	// Generate session token
	token := uuid.New().String()
	s.tokenMu.Lock()
	s.tokens[token] = true
	s.tokenMu.Unlock()

	// Clear any failure records on success
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
```

**Step 3: Install uuid dependency**

```bash
go get github.com/google/uuid@latest
go mod tidy
```

**Step 4: Write tests**

Create `internal/auth/auth_test.go`:

```go
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

	// 3 failures should trigger block
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

	// Even correct credentials should fail when blocked
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
```

**Step 5: Run tests**

```bash
cd D:/Code/HongBin/Go/web-terminal
go test ./internal/auth/ -v
```

Expected: PASS (4 tests)

**Step 6: Commit**

```bash
git add .
git commit -m "feat: auth service with SQLite brute-force protection"
```

---

### Task 3: Terminal Manager (ConPTY)

**Files:**
- Create: `internal/terminal/terminal.go`

**Step 1: Create terminal manager**

Create `internal/terminal/terminal.go`:

```go
package terminal

import (
	"context"
	"io"
	"sync"

	"github.com/UserExistsError/conpty"
)

type Terminal struct {
	cpty   *conpty.ConPty
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
}

func New(shell string, cols, rows int) (*Terminal, error) {
	cpty, err := conpty.Start(
		shell,
		conpty.ConPtyDimensions(cols, rows),
	)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	t := &Terminal{
		cpty:   cpty,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	// Monitor process exit
	go func() {
		defer close(t.done)
		cpty.Wait(ctx)
	}()

	return t, nil
}

func (t *Terminal) Read(p []byte) (int, error) {
	return t.cpty.Read(p)
}

func (t *Terminal) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cpty.Write(p)
}

func (t *Terminal) Resize(cols, rows int) error {
	return t.cpty.Resize(cols, rows)
}

func (t *Terminal) Done() <-chan struct{} {
	return t.done
}

func (t *Terminal) Close() error {
	t.cancel()
	return t.cpty.Close()
}

// Pipe reads from terminal and writes to writer until terminal closes or error.
func (t *Terminal) Pipe(w io.Writer) error {
	buf := make([]byte, 8192)
	for {
		n, err := t.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			return err
		}
	}
}
```

**Step 2: Manual test (no automated test - ConPTY requires Windows terminal)**

This module wraps ConPTY which is OS-level. It will be integration-tested via the WebSocket handler in Task 5.

**Step 3: Commit**

```bash
git add .
git commit -m "feat: terminal manager wrapping ConPTY"
```

---

### Task 4: Frontend (Login + Terminal UI)

**Files:**
- Create: `web/index.html`
- Create: `web/style.css`
- Create: `web/terminal.js`

**Step 1: Create index.html**

Create `web/index.html`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Web Terminal</title>
    <link rel="stylesheet" href="/style.css">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.min.css">
</head>
<body>
    <!-- Login Page -->
    <div id="login-container">
        <div class="login-box">
            <h1>Web Terminal</h1>
            <form id="login-form">
                <input type="text" id="username" placeholder="Username" autocomplete="username" required>
                <input type="password" id="password" placeholder="Password" autocomplete="current-password" required>
                <button type="submit" id="login-btn">Login</button>
                <p id="login-error" class="error"></p>
            </form>
        </div>
    </div>

    <!-- Terminal Page -->
    <div id="terminal-container" style="display:none;">
        <div id="terminal"></div>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/@xterm/addon-web-links@0.11.0/lib/addon-web-links.min.js"></script>
    <script src="/terminal.js"></script>
</body>
</html>
```

**Step 2: Create style.css**

Create `web/style.css`:

```css
* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

body {
    background: #1e1e1e;
    color: #cccccc;
    font-family: 'Segoe UI', sans-serif;
    height: 100vh;
    overflow: hidden;
}

/* Login */
#login-container {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100vh;
}

.login-box {
    background: #2d2d2d;
    padding: 40px;
    border-radius: 8px;
    width: 360px;
}

.login-box h1 {
    text-align: center;
    margin-bottom: 24px;
    color: #ffffff;
    font-size: 24px;
}

.login-box input {
    width: 100%;
    padding: 12px;
    margin-bottom: 16px;
    background: #3c3c3c;
    border: 1px solid #555;
    border-radius: 4px;
    color: #fff;
    font-size: 14px;
}

.login-box input:focus {
    outline: none;
    border-color: #007acc;
}

.login-box button {
    width: 100%;
    padding: 12px;
    background: #007acc;
    border: none;
    border-radius: 4px;
    color: #fff;
    font-size: 16px;
    cursor: pointer;
}

.login-box button:hover {
    background: #005f9e;
}

.login-box button:disabled {
    background: #555;
    cursor: not-allowed;
}

.error {
    color: #f44;
    text-align: center;
    margin-top: 12px;
    min-height: 20px;
}

/* Terminal */
#terminal-container {
    height: 100vh;
    width: 100vw;
}

#terminal {
    height: 100%;
    width: 100%;
}
```

**Step 3: Create terminal.js**

Create `web/terminal.js`:

```javascript
(function () {
    const TOKEN_KEY = 'web-terminal-token';

    // Check existing session
    const savedToken = localStorage.getItem(TOKEN_KEY);
    if (savedToken) {
        showTerminal(savedToken);
    }

    // Login form handler
    document.getElementById('login-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const btn = document.getElementById('login-btn');
        const errorEl = document.getElementById('login-error');
        errorEl.textContent = '';
        btn.disabled = true;

        const username = document.getElementById('username').value;
        const password = document.getElementById('password').value;

        try {
            const resp = await fetch('/api/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password }),
            });
            const data = await resp.json();

            if (!resp.ok) {
                errorEl.textContent = data.error || 'Login failed';
                btn.disabled = false;
                return;
            }

            localStorage.setItem(TOKEN_KEY, data.token);
            showTerminal(data.token);
        } catch (err) {
            errorEl.textContent = 'Connection error';
            btn.disabled = false;
        }
    });

    function showTerminal(token) {
        document.getElementById('login-container').style.display = 'none';
        document.getElementById('terminal-container').style.display = 'block';
        initTerminal(token);
    }

    function initTerminal(token) {
        const term = new Terminal({
            cursorBlink: true,
            fontSize: 14,
            fontFamily: 'Consolas, "Courier New", monospace',
            theme: {
                background: '#0c0c0c',
                foreground: '#cccccc',
                cursor: '#ffffff',
            },
        });

        const fitAddon = new FitAddon.FitAddon();
        const webLinksAddon = new WebLinksAddon.WebLinksAddon();
        term.loadAddon(fitAddon);
        term.loadAddon(webLinksAddon);

        const container = document.getElementById('terminal');
        term.open(container);
        fitAddon.fit();

        // WebSocket connection
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${location.host}/ws/terminal?token=${encodeURIComponent(token)}&cols=${term.cols}&rows=${term.rows}`;
        const ws = new WebSocket(wsUrl);
        ws.binaryType = 'arraybuffer';

        ws.onopen = () => {
            term.focus();
        };

        ws.onmessage = (event) => {
            const data = typeof event.data === 'string' ? event.data : new TextDecoder().decode(event.data);
            term.write(data);
        };

        ws.onclose = () => {
            term.write('\r\n\x1b[31m[Connection closed]\x1b[0m\r\n');
        };

        ws.onerror = () => {
            term.write('\r\n\x1b[31m[Connection error]\x1b[0m\r\n');
            // Token might be invalid
            localStorage.removeItem(TOKEN_KEY);
        };

        // Send user input to server
        term.onData((data) => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(data);
            }
        });

        // Handle resize
        window.addEventListener('resize', () => {
            fitAddon.fit();
        });

        term.onResize(({ cols, rows }) => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({ type: 'resize', cols, rows }));
            }
        });
    }
})();
```

**Step 4: Commit**

```bash
git add .
git commit -m "feat: frontend with login page and xterm.js terminal"
```

---

### Task 5: HTTP Server & WebSocket Handler

**Files:**
- Create: `internal/server/server.go`

**Step 1: Install WebSocket dependency**

```bash
cd D:/Code/HongBin/Go/web-terminal
go get github.com/gorilla/websocket@latest
go mod tidy
```

**Step 2: Create server module**

Create `internal/server/server.go`:

```go
package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"

	"web-terminal/internal/auth"
	"web-terminal/internal/config"
	"web-terminal/internal/terminal"

	"github.com/gorilla/websocket"
)

type Server struct {
	cfg      *config.Config
	auth     *auth.AuthService
	upgrader websocket.Upgrader
	webFS    fs.FS
}

func New(cfg *config.Config, authSvc *auth.AuthService, webContent embed.FS) (*Server, error) {
	// Get the "web" subdirectory from the embedded FS
	webFS, err := fs.Sub(webContent, "web")
	if err != nil {
		return nil, fmt.Errorf("failed to get web subdirectory: %w", err)
	}

	return &Server{
		cfg:  cfg,
		auth: authSvc,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		webFS: webFS,
	}, nil
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/ws/terminal", s.handleTerminal)
	mux.Handle("/", http.FileServer(http.FS(s.webFS)))

	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For for reverse proxy
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token,omitempty"`
	Error string `json:"error,omitempty"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: "Invalid request"})
		return
	}

	ip := getClientIP(r)
	token, err := s.auth.Login(req.Username, req.Password, ip)
	if err != nil {
		status := http.StatusUnauthorized
		msg := "Invalid credentials"
		if strings.Contains(err.Error(), "blocked") {
			status = http.StatusTooManyRequests
			msg = "Too many failed attempts, IP blocked"
		}
		writeJSON(w, status, loginResponse{Error: msg})
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{Token: token})
}

func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if !s.auth.ValidateToken(token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	cols := queryInt(r, "cols", 80)
	rows := queryInt(r, "rows", 24)

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	term, err := terminal.New(s.cfg.Terminal.Shell, cols, rows)
	if err != nil {
		log.Printf("Terminal creation error: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n[Error: %v]\r\n", err)))
		return
	}
	defer term.Close()

	// Terminal output -> WebSocket
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := term.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// WebSocket input -> Terminal
	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		if msgType == websocket.TextMessage {
			// Check if it's a resize command
			var resize struct {
				Type string `json:"type"`
				Cols int    `json:"cols"`
				Rows int    `json:"rows"`
			}
			if json.Unmarshal(msg, &resize) == nil && resize.Type == "resize" {
				term.Resize(resize.Cols, resize.Rows)
				continue
			}
		}

		// Regular input
		term.Write(msg)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return defaultVal
	}
	return n
}
```

**Step 3: Commit**

```bash
git add .
git commit -m "feat: HTTP server with login and WebSocket terminal handler"
```

---

### Task 6: Main Entry Point & Build

**Files:**
- Create: `main.go`

**Step 1: Create main.go**

Create `main.go`:

```go
package main

import (
	"embed"
	"flag"
	"log"

	"web-terminal/internal/auth"
	"web-terminal/internal/config"
	"web-terminal/internal/server"
)

//go:embed web/*
var webContent embed.FS

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	authSvc, err := auth.New(&cfg.Auth, "data.db")
	if err != nil {
		log.Fatalf("Failed to init auth: %v", err)
	}
	defer authSvc.Close()

	srv, err := server.New(cfg, authSvc, webContent)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Printf("Web Terminal starting on port %d", cfg.Server.Port)
	log.Printf("Shell: %s", cfg.Terminal.Shell)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
```

**Step 2: Build and test**

```bash
cd D:/Code/HongBin/Go/web-terminal
go mod tidy
go build -o web-terminal.exe .
```

Expected: Builds successfully, produces `web-terminal.exe`

**Step 3: Manual smoke test**

```bash
./web-terminal.exe
```

Then open browser to `http://localhost:8080`:
1. Should see login page
2. Login with admin/changeme
3. Should see terminal with CMD prompt
4. Type `dir` - should see directory listing
5. Type `color 0a` - should see green text (verifies ANSI color support)

**Step 4: Commit**

```bash
git add .
git commit -m "feat: main entry point, project is functional"
```

---

### Task 7: Polish & Edge Cases

**Files:**
- Modify: `web/terminal.js`
- Modify: `internal/server/server.go`

**Step 1: Add reconnection hint on disconnect**

In `web/terminal.js`, update `ws.onclose` to offer a reload button:

```javascript
ws.onclose = () => {
    term.write('\r\n\x1b[31m[Connection closed. Press any key to reconnect...]\x1b[0m\r\n');
    term.onData(() => {
        location.reload();
    });
};
```

**Step 2: Add logout support**

In `web/terminal.js`, add a keyboard shortcut (Ctrl+Shift+L) to logout:

Add this after `term.onResize`:

```javascript
// Ctrl+Shift+L to logout
document.addEventListener('keydown', (e) => {
    if (e.ctrlKey && e.shiftKey && e.key === 'L') {
        localStorage.removeItem(TOKEN_KEY);
        location.reload();
    }
});
```

**Step 3: Build final binary**

```bash
cd D:/Code/HongBin/Go/web-terminal
go build -o web-terminal.exe .
```

**Step 4: Full manual test**

1. Start server: `./web-terminal.exe`
2. Open `http://localhost:8080`
3. Test wrong password multiple times -> should see "IP blocked" after 5 attempts
4. Login with correct credentials
5. Run commands: `dir`, `echo hello`, `cls` (clear screen)
6. Test color: `color 0a` then `color 07` to reset
7. Test resize: resize browser window, terminal should adapt
8. Test Ctrl+Shift+L logout
9. Test reconnection after closing/reopening tab

**Step 5: Commit**

```bash
git add .
git commit -m "feat: polish with reconnection and logout support"
```

---

### Task 8: Add .gitignore and Final Cleanup

**Files:**
- Create: `.gitignore`

**Step 1: Create .gitignore**

```
web-terminal.exe
data.db
*.db
```

**Step 2: Commit**

```bash
git add .
git commit -m "chore: add gitignore"
```

---

## Summary

| Task | Description | Estimated Effort |
|------|-------------|-----------------|
| 1 | Project scaffolding & config | Small |
| 2 | Auth service with SQLite | Medium |
| 3 | Terminal manager (ConPTY) | Small |
| 4 | Frontend (login + xterm.js) | Medium |
| 5 | HTTP server & WebSocket | Medium |
| 6 | Main entry point & build | Small |
| 7 | Polish & edge cases | Small |
| 8 | Gitignore & cleanup | Trivial |

**Dependencies:** Task 1 -> Task 2 -> Task 3 -> Task 4 -> Task 5 -> Task 6 -> Task 7 -> Task 8 (sequential, each builds on previous)
