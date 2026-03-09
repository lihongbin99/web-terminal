# Web Terminal

A browser-based terminal emulator for Windows, built with Go and [xterm.js](https://xtermjs.org/).

Through a web interface, you can log in and access a fully functional Windows terminal (cmd.exe, PowerShell, etc.) from any browser.

## Features

- Browser-based terminal with full xterm.js emulation
- Username/password authentication
- IP-based brute-force protection (configurable max attempts and block duration)
- Directory selection with history tracking
- Terminal resizing support
- WebSocket real-time communication
- Static assets embedded in binary, single-file deployment

## Quick Start

### Prerequisites

- Go 1.25+
- Windows (uses Windows ConPty API)
- GCC toolchain (required by go-sqlite3, e.g. [MinGW-w64](https://www.mingw-w64.org/))

### Build & Run

```bash
go build -o web-terminal.exe
./web-terminal.exe
```

Open http://localhost:8080 in your browser.

### Custom Config

```bash
./web-terminal.exe -config /path/to/config.yaml
```

## Configuration

Create a `config.yaml` in the working directory:

```yaml
server:
  port: 8080

auth:
  username: admin
  password: changeme        # Change this!
  max_attempts: 5           # Failed attempts before IP block
  block_duration: 30m       # Block duration

terminal:
  shell: cmd.exe            # cmd.exe, powershell.exe, etc.
```

All fields have defaults and are optional.

## Project Structure

```
web-terminal/
├── main.go                  # Entry point
├── config.yaml              # Configuration
├── internal/
│   ├── config/              # YAML config loading
│   ├── auth/                # Authentication, session tokens, brute-force protection
│   ├── server/              # HTTP/WebSocket server
│   └── terminal/            # Windows ConPty terminal wrapper
└── web/
    ├── index.html           # Frontend UI
    ├── terminal.js          # Terminal logic
    └── style.css            # Styling
```

## How It Works

1. User opens the web page and logs in
2. After authentication, a session token is issued
3. User selects a working directory (with history support)
4. A WebSocket connection is established, spawning a shell process via Windows ConPty
5. Terminal I/O is streamed over WebSocket in real-time

## API

| Endpoint | Method | Description |
|---|---|---|
| `/api/login` | POST | Authenticate and get session token |
| `/ws/terminal` | WebSocket | Terminal I/O stream (token required) |
| `/api/dirs` | GET | Get directory history (token required) |
| `/api/dirs` | POST | Record directory access (token required) |

## Keyboard Shortcuts

- `Ctrl+Shift+L` — Logout

## License

MIT
