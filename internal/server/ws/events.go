package ws

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const eventsPolInterval = 2 * time.Second

// EventsHandler streams new lines from a project's channel.md (team events log)
// over a WebSocket. The client receives one line per message, prefixed with
// the type "event".
type EventsHandler struct {
	// resolveChannelPath maps a project ID to the .aom/channel.md path.
	resolveChannelPath func(projectID string) (string, bool)
}

// NewEventsHandler creates the handler. resolveChannelPath must return the
// absolute path to channel.md and true, or ("", false) if the project is unknown.
func NewEventsHandler(resolve func(string) (string, bool)) *EventsHandler {
	return &EventsHandler{resolveChannelPath: resolve}
}

// ServeHTTP handles GET /ws/events/{project-id}.
func (h *EventsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")
	if strings.TrimSpace(projectID) == "" {
		http.Error(w, "project ID required", http.StatusBadRequest)
		return
	}

	channelPath, ok := h.resolveChannelPath(projectID)
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws/events] upgrade error: %v", err)
		return
	}

	go streamFileLines(conn, channelPath)
}

// streamFileLines tails a file and sends each new line to the WebSocket client.
// It exits when the connection closes or an error occurs.
func streamFileLines(conn *websocket.Conn, filePath string) {
	defer conn.Close()

	// Seed the cursor at the current file end so only new events are sent.
	offset := currentFileSize(filePath)

	ticker := time.NewTicker(eventsPolInterval)
	defer ticker.Stop()

	conn.SetReadDeadline(time.Now().Add(pongTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongTimeout))
	})

	// Drain read side so pong frames are processed.
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-readDone:
			return
		case <-ticker.C:
			data, err := os.ReadFile(filePath)
			if err != nil || len(data) <= offset {
				continue
			}
			newContent := string(data[offset:])
			offset = len(data)

			for _, line := range strings.Split(newContent, "\n") {
				if trimmed := strings.TrimSpace(line); trimmed != "" {
					conn.SetWriteDeadline(time.Now().Add(writeTimeout))
					if err := conn.WriteJSON(OutMessage{Type: "event", Data: trimmed}); err != nil {
						return
					}
				}
			}
		}
	}
}

func currentFileSize(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return len(data)
}
