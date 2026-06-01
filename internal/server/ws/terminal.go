package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/tmux"
)

const (
	pollInterval    = 500 * time.Millisecond
	writeTimeout    = 10 * time.Second
	pongTimeout     = 60 * time.Second
	pingInterval    = 30 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // localhost-only server; CORS handled separately
}

// InMessage is a message sent from the browser to the server.
type InMessage struct {
	Type string `json:"type"` // "input" | "resize"
	Data string `json:"data"` // keystrokes for "input"
	Cols int    `json:"cols"` // for "resize"
	Rows int    `json:"rows"` // for "resize"
}

// OutMessage is a message sent from the server to the browser.
type OutMessage struct {
	Type string `json:"type"` // "output" | "error"
	Data string `json:"data"`
}

// TerminalHandler manages WebSocket connections to tmux panes.
// Each connection streams one pane: the server polls tmux capture-pane and
// forwards new content; keystrokes from the client are forwarded via send-keys.
type TerminalHandler struct {
	tmux *tmux.Manager
}

// NewTerminalHandler creates the handler.
func NewTerminalHandler(m *tmux.Manager) *TerminalHandler {
	return &TerminalHandler{tmux: m}
}

// ServeHTTP handles GET /ws/terminal/{pane-id}.
func (h *TerminalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	paneID := r.PathValue("pane")
	if strings.TrimSpace(paneID) == "" {
		http.Error(w, "pane ID required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws/terminal] upgrade error: %v", err)
		return
	}

	s := &terminalSession{
		conn:   conn,
		tmux:   h.tmux,
		paneID: paneID,
	}
	go s.run()
}

// terminalSession holds the state for one active WebSocket ↔ tmux connection.
type terminalSession struct {
	conn   *websocket.Conn
	tmux   *tmux.Manager
	paneID string

	mu       sync.Mutex
	lastSnap string // last content sent; used to suppress no-change polls
}

func (s *terminalSession) run() {
	defer s.conn.Close()

	// Verify the pane exists before starting the loop.
	if ok, _ := s.tmux.PaneExists(s.paneID); !ok {
		_ = s.sendError("pane not found: " + s.paneID)
		return
	}

	// Send the initial snapshot immediately so the browser shows something fast.
	if snap, err := s.tmux.CapturePane(s.paneID); err == nil {
		s.lastSnap = snap
		_ = s.sendOutput(snap)
	}

	// Read loop (client → server): runs in a goroutine so it doesn't block polling.
	readDone := make(chan struct{})
	go s.readLoop(readDone)

	// Poll loop (tmux → client): captures pane every pollInterval and sends diffs.
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-readDone:
			return
		case <-ticker.C:
			snap, err := s.tmux.CapturePane(s.paneID)
			if err != nil {
				// Pane likely died; close gracefully.
				_ = s.sendError("pane closed")
				return
			}
			s.mu.Lock()
			changed := snap != s.lastSnap
			if changed {
				s.lastSnap = snap
			}
			s.mu.Unlock()
			if changed {
				if err := s.sendOutput(snap); err != nil {
					return
				}
			}
		case <-pingTicker.C:
			s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readLoop processes messages from the browser (keystrokes / resize).
func (s *terminalSession) readLoop(done chan<- struct{}) {
	defer close(done)
	s.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	s.conn.SetPongHandler(func(string) error {
		return s.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	})

	for {
		_, raw, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg InMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "input":
			if msg.Data != "" {
				_ = s.tmux.SendKeys(s.paneID, msg.Data)
			}
		}
	}
}

func (s *terminalSession) sendOutput(data string) error {
	return s.writeJSON(OutMessage{Type: "output", Data: data})
}

func (s *terminalSession) sendError(msg string) error {
	return s.writeJSON(OutMessage{Type: "error", Data: msg})
}

func (s *terminalSession) writeJSON(v any) error {
	s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return s.conn.WriteJSON(v)
}
