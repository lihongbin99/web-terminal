package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
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
	mux.HandleFunc("/api/dirs", s.handleDirs)
	mux.HandleFunc("/api/browse", s.handleBrowse)
	mux.Handle("/", http.FileServer(http.FS(s.webFS)))

	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func getClientIP(r *http.Request) string {
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
	workDir := r.URL.Query().Get("workDir")

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	term, err := terminal.New(s.cfg.Terminal.Shell, cols, rows, workDir)
	if err != nil {
		log.Printf("Terminal creation error: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n[Error: %v]\r\n", err)))
		return
	}
	defer term.Close()

	if workDir != "" {
		s.auth.RecordDir(workDir)
	}

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

		term.Write(msg)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) handleDirs(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if !s.auth.ValidateToken(token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		dirs, err := s.auth.GetDirs(20)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get dirs"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"dirs": dirs})
	case http.MethodPost:
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
			return
		}
		if err := s.auth.RecordDir(req.Path); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to record dir"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if !s.auth.ValidateToken(token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	dirPath := r.URL.Query().Get("path")

	// If path is empty, return drive letters on Windows
	if dirPath == "" {
		var drives []string
		for c := 'A'; c <= 'Z'; c++ {
			drive := string(c) + `:\`
			if _, err := os.Stat(drive); err == nil {
				drives = append(drives, drive)
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": drives})
		return
	}

	// Clean path and list subdirectories
	dirPath = filepath.Clean(dirPath)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Cannot read directory"})
		return
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": dirs})
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
