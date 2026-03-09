package session

import (
	"fmt"
	"log"
	"sync"
	"time"

	"web-terminal/internal/terminal"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// RingBuffer is a circular buffer that keeps the last N bytes of output.
type RingBuffer struct {
	buf  []byte
	w    int
	full bool
	mu   sync.Mutex
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{buf: make([]byte, size)}
}

// Write appends data to the ring buffer.
func (rb *RingBuffer) Write(p []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	n := len(p)
	if n >= len(rb.buf) {
		// Data larger than buffer — keep only the tail
		copy(rb.buf, p[n-len(rb.buf):])
		rb.w = 0
		rb.full = true
		return n, nil
	}

	// How much fits before wrap
	space := len(rb.buf) - rb.w
	if n <= space {
		copy(rb.buf[rb.w:], p)
		rb.w += n
		if rb.w == len(rb.buf) {
			rb.w = 0
			rb.full = true
		}
	} else {
		copy(rb.buf[rb.w:], p[:space])
		copy(rb.buf, p[space:])
		rb.w = n - space
		rb.full = true
	}
	return n, nil
}

// Bytes returns all buffered data in chronological order.
func (rb *RingBuffer) Bytes() []byte {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if !rb.full {
		out := make([]byte, rb.w)
		copy(out, rb.buf[:rb.w])
		return out
	}
	out := make([]byte, len(rb.buf))
	copy(out, rb.buf[rb.w:])
	copy(out[len(rb.buf)-rb.w:], rb.buf[:rb.w])
	return out
}

// Session represents a persistent terminal session.
type Session struct {
	ID        string
	Name      string
	Term      *terminal.Terminal
	Buffer    *RingBuffer
	CreatedAt time.Time

	listeners map[*websocket.Conn]struct{}
	mu        sync.Mutex
	done      chan struct{}
}

// SessionInfo is the JSON-serializable info about a session.
type SessionInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

// Attach adds a WebSocket connection as a listener.
// It first sends the buffered history to the connection.
func (s *Session) Attach(conn *websocket.Conn) {
	history := s.Buffer.Bytes()
	if len(history) > 0 {
		conn.WriteMessage(websocket.BinaryMessage, history)
	}

	s.mu.Lock()
	s.listeners[conn] = struct{}{}
	s.mu.Unlock()
}

// Detach removes a WebSocket connection from listeners.
func (s *Session) Detach(conn *websocket.Conn) {
	s.mu.Lock()
	delete(s.listeners, conn)
	s.mu.Unlock()
}

// broadcast sends data to all attached listeners.
func (s *Session) broadcast(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for conn := range s.listeners {
		if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
			delete(s.listeners, conn)
		}
	}
}

// SessionManager manages persistent terminal sessions.
type SessionManager struct {
	sessions map[string]*Session
	shell    string
	mu       sync.RWMutex
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(shell string) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
		shell:    shell,
	}
}

const ringBufferSize = 1024 * 1024 // 1MB

// Create creates a new persistent terminal session.
func (m *SessionManager) Create(name, workDir string, cols, rows int) (*Session, error) {
	term, err := terminal.New(m.shell, cols, rows, workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create terminal: %w", err)
	}

	s := &Session{
		ID:        uuid.New().String(),
		Name:      name,
		Term:      term,
		Buffer:    NewRingBuffer(ringBufferSize),
		CreatedAt: time.Now(),
		listeners: make(map[*websocket.Conn]struct{}),
		done:      make(chan struct{}),
	}

	// Background reader: reads terminal output, writes to buffer, broadcasts to listeners
	go func() {
		defer close(s.done)
		buf := make([]byte, 8192)
		for {
			n, err := s.Term.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				s.Buffer.Write(data)
				s.broadcast(data)
			}
			if err != nil {
				return
			}
		}
	}()

	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()

	log.Printf("Session created: %s (%s)", s.ID, name)
	return s, nil
}

// Get returns a session by ID.
func (m *SessionManager) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// Delete closes and removes a session.
func (m *SessionManager) Delete(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session not found: %s", id)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	log.Printf("Session deleted: %s (%s)", s.ID, s.Name)
	return s.Term.Close()
}

// List returns info about all sessions.
func (m *SessionManager) List() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, SessionInfo{
			ID:        s.ID,
			Name:      s.Name,
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		})
	}
	return list
}
