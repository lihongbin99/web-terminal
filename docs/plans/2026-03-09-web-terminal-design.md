# Web Terminal Design

## Overview

A web-based terminal emulator that allows operating Windows CMD from a browser with near-native terminal appearance (colors, text layout, cursor). Built with Go backend and xterm.js frontend, deployed on public internet with authentication.

## Architecture

```
Browser (xterm.js)
    |
    |  WebSocket (ws://)
    |
Go HTTP Server
    +-- /api/login       POST  username+password -> UUID session token
    +-- /ws/terminal     WebSocket (requires session token) -> terminal I/O
    +-- /                Static files (HTML/JS/CSS, embedded in binary)
    |
    |  ConPTY API
    |
cmd.exe (pseudo-terminal process)
```

### Data Flow

1. User types in xterm.js -> WebSocket sends keystrokes to Go backend
2. Go backend writes to ConPTY -> cmd.exe processes input
3. cmd.exe output (with ANSI escape codes) -> ConPTY read -> WebSocket sends to frontend
4. xterm.js parses ANSI sequences and renders (colors, cursor movement, etc.)

## Tech Stack

- **Backend**: Go
  - `github.com/UserExistsError/conpty` - Windows ConPTY API
  - `github.com/gorilla/websocket` - WebSocket
  - `github.com/mattn/go-sqlite3` - SQLite
  - `gopkg.in/yaml.v3` - Config parsing
  - `go:embed` - Embed static assets into binary
- **Frontend**: Vanilla HTML/CSS/JS
  - xterm.js + fit addon + web-links addon
- **Database**: SQLite (login failures, blocked IPs)

## Authentication & Security

- Username and password stored in plaintext in `config.yaml`
- Login success generates a UUID session token (no expiration)
- Token stored in localStorage on client, sent via WebSocket URL param
- Brute-force protection: 5 failed attempts per IP within 5 minutes -> IP blocked
- Blocked IP duration configurable (default 30 minutes)
- SQLite stores: failed login records, blocked IPs
- SSL/reverse proxy handled externally by user (nginx)

## Project Structure

```
web-terminal/
+-- main.go              # Entry point, start HTTP server
+-- config.yaml          # Configuration (port, credentials, etc.)
+-- internal/
|   +-- auth/            # Auth logic (login, session, brute-force protection)
|   +-- terminal/        # ConPTY terminal management
|   +-- server/          # HTTP/WebSocket routes and handlers
+-- web/                 # Frontend static assets (embedded)
|   +-- index.html       # Login page + terminal page
|   +-- terminal.js      # xterm.js init and WebSocket connection
|   +-- style.css        # Styles
+-- go.mod
+-- go.sum
```

## Configuration

```yaml
server:
  port: 8080
auth:
  username: admin
  password: your-password
  max_attempts: 5
  block_duration: 30m
terminal:
  shell: cmd.exe
```

## Frontend

- Login page: simple username + password form
- Terminal page: full-screen xterm.js terminal
- Theme: dark background (CMD-style black background, white text)
- Font: monospace (Consolas or Courier New)
- Auto-resize: fit addon + notify backend to resize ConPTY
- Session check: localStorage token -> validate -> show terminal or login

## Scope

### In Scope (v1)
- Single terminal session
- Username/password authentication
- Brute-force protection with IP blocking
- ConPTY for full terminal emulation (colors, cursor, interactive programs)
- Single binary deployment with embedded static assets
- Configurable via YAML

### Out of Scope (future)
- Multiple terminal tabs/sessions
- Multiple user accounts
- OAuth/third-party login
- Built-in TLS/HTTPS (use nginx)
- File upload/download
