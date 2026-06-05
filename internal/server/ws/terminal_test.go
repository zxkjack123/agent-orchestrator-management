package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/tmux"
)

// dialWS upgrades a test server connection to WebSocket and returns the client conn.
func dialWS(t *testing.T, srv *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial WebSocket %s: %v", url, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// TestTerminalHandlerMissingPane verifies that connecting with a pane that
// doesn't exist causes the server to send an error message and close.
func TestTerminalHandlerMissingPane(t *testing.T) {
	m := tmux.NewManager()
	h := NewTerminalHandler(m)

	mux := http.NewServeMux()
	mux.Handle("/ws/terminal/{pane}", h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn := dialWS(t, srv, "/ws/terminal/nonexistent-pane-id")

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	var msg OutMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "error" {
		t.Errorf("msg.Type = %q, want error", msg.Type)
	}
	if !strings.Contains(msg.Data, "pane not found") {
		t.Errorf("msg.Data = %q, want 'pane not found'", msg.Data)
	}
}

// TestTerminalHandlerEmptyPaneID verifies that a request with no pane ID
// returns HTTP 400 before upgrading to WebSocket.
func TestTerminalHandlerEmptyPaneID(t *testing.T) {
	m := tmux.NewManager()
	h := NewTerminalHandler(m)

	req := httptest.NewRequest(http.MethodGet, "/ws/terminal/", nil)
	req.SetPathValue("pane", "")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestOutMessageJSON verifies that OutMessage serialises correctly.
func TestOutMessageJSON(t *testing.T) {
	msg := OutMessage{Type: "output", Data: "hello world"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"type":"output"`) {
		t.Errorf("serialised = %s, want type:output", data)
	}
	if !strings.Contains(string(data), `"data":"hello world"`) {
		t.Errorf("serialised = %s, want data:hello world", data)
	}
}

// TestInMessageJSON verifies that InMessage deserialises correctly.
func TestInMessageJSON(t *testing.T) {
	raw := `{"type":"input","data":"ls -la\n"}`
	var msg InMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "input" {
		t.Errorf("type = %q, want input", msg.Type)
	}
	if msg.Data != "ls -la\n" {
		t.Errorf("data = %q, want ls -la\\n", msg.Data)
	}
}
