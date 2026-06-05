package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/tmux"
)

const (
	writeTimeout = 10 * time.Second
	pongTimeout  = 60 * time.Second
	pingInterval = 30 * time.Second
	defaultCols  = 220
	defaultRows  = 50
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // localhost-only server; CORS handled separately
}

// InMessage is a message sent from the browser to the server.
type InMessage struct {
	Type string `json:"type"` // "resize" | "input"
	Data string `json:"data"` // keystrokes for "input"
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// OutMessage is a JSON control message sent from the server to the browser.
// Terminal output is sent as binary WebSocket frames (raw PTY bytes), not as OutMessage.
type OutMessage struct {
	Type string `json:"type"` // "ready" | "error"
	Data string `json:"data,omitempty"`
}

// TerminalHandler manages WebSocket connections to tmux panes.
//
// Two streaming modes are selected automatically based on the pane's window layout:
//
//   - PTY attach (single-pane window): tmux renders at browser dimensions and streams
//     output in real-time via a PTY. This is the smooth path used when each agent has
//     its own dedicated tmux session (aom session spawn without --grid).
//
//   - Viewer script (multi-pane window): a shell script polls capture-pane every 50ms
//     inside a PTY. Used as a fallback for --grid / aom orchestrate sessions where
//     multiple agents share one tmux window.
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

// terminalSession holds state for one active WebSocket ↔ terminal connection.
type terminalSession struct {
	conn   *websocket.Conn
	tmux   *tmux.Manager
	paneID string

	mu          sync.Mutex
	ptmx        *os.File // PTY master fd
	tempSession string   // grouped temp session to kill on disconnect (PTY path only)
}

func (s *terminalSession) run() {
	defer s.conn.Close()
	defer s.cleanup()

	if ok, _ := s.tmux.PaneExists(s.paneID); !ok {
		_ = s.sendError("pane not found: " + s.paneID)
		return
	}

	if err := s.writeJSON(OutMessage{Type: "ready"}); err != nil {
		return
	}

	// Wait up to 10s for browser dimensions.
	cols, rows := defaultCols, defaultRows
	s.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, raw, err := s.conn.ReadMessage(); err == nil {
		var msg InMessage
		if json.Unmarshal(raw, &msg) == nil && msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
			cols, rows = msg.Cols, msg.Rows
		}
	}
	s.conn.SetReadDeadline(time.Time{})

	// Choose streaming mode based on window layout.
	sessionName, windowID, err := s.tmux.PaneSessionInfo(s.paneID)
	if err == nil {
		panes, listErr := s.tmux.ListPanesInWindow(sessionName + ":" + windowID)
		if listErr == nil && len(panes) == 1 {
			// Single-pane window (dedicated agent session): PTY attach for real-time streaming.
			s.runPTYAttach(sessionName, windowID, cols, rows)
			return
		}
	}

	// Multi-pane window (--grid / aom orchestrate): viewer script fallback.
	// Agents in shared sessions should be migrated to dedicated sessions via
	// "Isolate Session" in the War Room UI or by re-spawning after provisioning.
	s.runViewerScript(cols, rows)
}

// runPTYAttach creates a temporary grouped tmux session, attaches via PTY, and
// bridges raw bytes ↔ WebSocket. tmux renders output at browser dimensions so
// there is no dimension mismatch and no polling delay — purely real-time.
// Falls back to runViewerScript on any failure (e.g. race with cleanup of a
// previous grouped session).
func (s *terminalSession) runPTYAttach(sessionName, windowID string, cols, rows int) {
	tempName, err := s.tmux.NewGroupedSession(sessionName, windowID, cols, rows)
	if err != nil {
		log.Printf("[ws/terminal] grouped-session fallback for pane %s: %v", s.paneID, err)
		s.runViewerScript(cols, rows)
		return
	}
	s.mu.Lock()
	s.tempSession = tempName
	s.mu.Unlock()

	tmuxBin := s.tmux.BinaryPath()
	cmd := exec.Command(tmuxBin, "attach-session", "-t", tempName)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		_ = s.sendError("cannot attach: " + err.Error())
		return
	}
	defer ptmx.Close()
	s.mu.Lock()
	s.ptmx = ptmx
	s.mu.Unlock()

	s.bridgePTY(ptmx, true)
}

// runViewerScript runs a shell inside a PTY that polls capture-pane every 50ms.
// Uses alternate screen buffer to eliminate flicker: \033[H overwrites in-place
// without a clear-screen blank flash. Used when the pane is in a multi-pane window.
func (s *terminalSession) runViewerScript(cols, rows int) {
	script := fmt.Sprintf(
		`printf '\033[?1049h';`+
			` trap 'printf "\033[?1049l"' EXIT;`+
			` while true; do`+
			` cap=$(tmux capture-pane -p -e -J -t '%s' 2>/dev/null) || break;`+
			` printf '\033[H\033[?25l';`+
			` printf '%%s\n' "$cap";`+
			` printf '\033[J\033[?25h';`+
			` sleep 0.05;`+
			` done;`+
			` printf '\033[?1049l\n\033[33m[aom] pane closed\033[0m\n'`,
		s.paneID,
	)

	cmd := exec.Command("sh", "-c", script)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		_ = s.sendError("viewer failed: " + err.Error())
		return
	}
	defer ptmx.Close()
	s.mu.Lock()
	s.ptmx = ptmx
	s.mu.Unlock()

	// In viewer mode keystrokes route to the original pane, not the sh process.
	s.bridgePTY(ptmx, false)
}

// bridgePTY streams PTY output to the WebSocket and routes incoming messages.
// directInput: true  → binary frames go to the PTY (PTY attach mode, interactive)
// directInput: false → binary frames go to the original pane via send-keys (viewer mode)
func (s *terminalSession) bridgePTY(ptmx *os.File, directInput bool) {
	// PTY → WebSocket
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
				if werr := s.conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	s.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	s.conn.SetPongHandler(func(string) error {
		return s.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	})

	// WebSocket → PTY / pane
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			msgType, raw, err := s.conn.ReadMessage()
			if err != nil {
				return
			}
			switch msgType {
			case websocket.BinaryMessage:
				if directInput {
					s.mu.Lock()
					p := s.ptmx
					s.mu.Unlock()
					if p != nil {
						_, _ = p.Write(raw)
					}
				} else {
					_ = s.tmux.SendRawInput(s.paneID, string(raw))
				}
			case websocket.TextMessage:
				var msg InMessage
				if json.Unmarshal(raw, &msg) != nil {
					continue
				}
				switch msg.Type {
				case "input":
					if msg.Data != "" {
						if directInput {
							s.mu.Lock()
							p := s.ptmx
							s.mu.Unlock()
							if p != nil {
								_, _ = p.Write([]byte(msg.Data))
							}
						} else {
							_ = s.tmux.SendRawInput(s.paneID, msg.Data)
						}
					}
				case "resize":
					if msg.Cols > 0 && msg.Rows > 0 {
						s.mu.Lock()
						p := s.ptmx
						s.mu.Unlock()
						if p != nil {
							_ = pty.Setsize(p, &pty.Winsize{
								Rows: uint16(msg.Rows),
								Cols: uint16(msg.Cols),
							})
						}
					}
				}
			}
		}
	}()

	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-readDone:
			return
		case <-writeDone:
			return
		case <-pingTicker.C:
			s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// cleanup kills the temporary grouped session created for PTY attach mode.
func (s *terminalSession) cleanup() {
	s.mu.Lock()
	tempName := s.tempSession
	s.tempSession = ""
	s.mu.Unlock()
	if tempName != "" {
		_ = s.tmux.KillSession(tempName)
	}
}

func (s *terminalSession) sendError(msg string) error {
	return s.writeJSON(OutMessage{Type: "error", Data: msg})
}

func (s *terminalSession) writeJSON(v any) error {
	s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return s.conn.WriteJSON(v)
}
