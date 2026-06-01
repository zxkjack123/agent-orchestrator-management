package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestEventsHandlerProjectNotFound verifies 404 for unknown project.
func TestEventsHandlerProjectNotFound(t *testing.T) {
	h := NewEventsHandler(func(id string) (string, bool) { return "", false })

	req := httptest.NewRequest(http.MethodGet, "/ws/events/unknown", nil)
	req.SetPathValue("project", "unknown")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// TestEventsHandlerEmptyProjectID verifies 400 for missing project param.
func TestEventsHandlerEmptyProjectID(t *testing.T) {
	h := NewEventsHandler(func(id string) (string, bool) { return "", true })

	req := httptest.NewRequest(http.MethodGet, "/ws/events/", nil)
	req.SetPathValue("project", "")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestEventsHandlerStreamsNewLines verifies that new lines appended to the
// channel file are forwarded to connected WebSocket clients.
func TestEventsHandlerStreamsNewLines(t *testing.T) {
	dir := t.TempDir()
	channelPath := filepath.Join(dir, "channel.md")
	// Create with existing content — should NOT be re-sent.
	_ = os.WriteFile(channelPath, []byte("old content\n"), 0o644)

	h := NewEventsHandler(func(id string) (string, bool) {
		if id == "proj1" {
			return channelPath, true
		}
		return "", false
	})

	mux := http.NewServeMux()
	mux.Handle("/ws/events/{project}", h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn := dialWS(t, srv, "/ws/events/proj1")

	// Append a new line after connection is established.
	go func() {
		time.Sleep(200 * time.Millisecond)
		f, _ := os.OpenFile(channelPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if f != nil {
			defer f.Close()
			_, _ = f.WriteString("### new event from agent-x\n")
		}
	}()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg OutMessage
	_ = json.Unmarshal(raw, &msg)

	if msg.Type != "event" {
		t.Errorf("type = %q, want event", msg.Type)
	}
	if !strings.Contains(msg.Data, "new event") {
		t.Errorf("data = %q, should contain 'new event'", msg.Data)
	}
	if strings.Contains(msg.Data, "old content") {
		t.Errorf("old content should not be re-sent, got: %q", msg.Data)
	}
}

// TestEventsHandlerNoReplayOnConnect verifies pre-existing content is skipped.
func TestEventsHandlerNoReplayOnConnect(t *testing.T) {
	dir := t.TempDir()
	channelPath := filepath.Join(dir, "channel.md")
	_ = os.WriteFile(channelPath, []byte("existing line 1\nexisting line 2\n"), 0o644)

	h := NewEventsHandler(func(id string) (string, bool) {
		return channelPath, true
	})

	mux := http.NewServeMux()
	mux.Handle("/ws/events/{project}", h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn := dialWS(t, srv, "/ws/events/any")

	// Append new content after a brief delay.
	go func() {
		time.Sleep(150 * time.Millisecond)
		f, _ := os.OpenFile(channelPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if f != nil {
			defer f.Close()
			_, _ = f.WriteString("fresh line\n")
		}
	}()

	conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg OutMessage
	_ = json.Unmarshal(raw, &msg)

	if strings.Contains(msg.Data, "existing") {
		t.Errorf("existing content was replayed: %q", msg.Data)
	}
	if !strings.Contains(msg.Data, "fresh") {
		t.Errorf("fresh line not received: %q", msg.Data)
	}
}

// TestCurrentFileSize verifies the helper returns 0 for a missing file.
func TestCurrentFileSize(t *testing.T) {
	if got := currentFileSize("/does/not/exist"); got != 0 {
		t.Errorf("currentFileSize(missing) = %d, want 0", got)
	}
}

// TestCurrentFileSizeWithContent verifies the helper returns the correct byte count.
func TestCurrentFileSizeWithContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	content := "hello world"
	_ = os.WriteFile(path, []byte(content), 0o644)

	if got := currentFileSize(path); got != len(content) {
		t.Errorf("currentFileSize = %d, want %d", got, len(content))
	}
}

// TestMailboxHandlerProjectNotFound mirrors the events test for mailbox.
func TestMailboxHandlerProjectNotFound(t *testing.T) {
	h := NewMailboxHandler(func(p, a string) (string, bool) { return "", false })

	req := httptest.NewRequest(http.MethodGet, "/ws/mailbox/x/y", nil)
	req.SetPathValue("project", "x")
	req.SetPathValue("agent", "y")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// TestMailboxHandlerStreamsNewMessages verifies that new mailbox entries are forwarded.
func TestMailboxHandlerStreamsNewMessages(t *testing.T) {
	dir := t.TempDir()
	mailboxPath := filepath.Join(dir, "agent-a.md")
	_ = os.WriteFile(mailboxPath, []byte("# Mailbox\n\n"), 0o644)

	h := NewMailboxHandler(func(p, a string) (string, bool) {
		return mailboxPath, true
	})

	mux := http.NewServeMux()
	mux.Handle("/ws/mailbox/{project}/{agent}", h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn := dialWS(t, srv, "/ws/mailbox/proj/agent-a")

	go func() {
		time.Sleep(200 * time.Millisecond)
		f, _ := os.OpenFile(mailboxPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if f != nil {
			defer f.Close()
			_, _ = f.WriteString("### 2026-01-01 | MSG-1 | from: orchestrator\nhello agent\n\n")
		}
	}()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var msg OutMessage
		_ = json.Unmarshal(raw, &msg)
		if strings.Contains(msg.Data, "hello agent") {
			return // success
		}
	}
}

// Ensure websocket package is used (avoids unused import if tests are skipped).
var _ = websocket.DefaultDialer
