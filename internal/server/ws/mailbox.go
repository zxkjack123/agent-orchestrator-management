package ws

import (
	"log"
	"net/http"
	"strings"
	"time"
)

const mailboxPollInterval = 2 * time.Second

// MailboxHandler streams new mailbox messages for a specific agent over WebSocket.
// It behaves like `aom message watch` — only sends messages that arrive after
// the connection is opened (cursor-based, no replay of old messages).
type MailboxHandler struct {
	// resolveMailboxPath maps (projectID, agentName) to the .aom/mailbox/<agent>.md path.
	resolveMailboxPath func(projectID, agentName string) (string, bool)
}

// NewMailboxHandler creates the handler.
func NewMailboxHandler(resolve func(string, string) (string, bool)) *MailboxHandler {
	return &MailboxHandler{resolveMailboxPath: resolve}
}

// ServeHTTP handles GET /ws/mailbox/{project}/{agent}.
func (h *MailboxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")
	agentName := r.PathValue("agent")
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(agentName) == "" {
		http.Error(w, "project and agent are required", http.StatusBadRequest)
		return
	}

	mailboxPath, ok := h.resolveMailboxPath(projectID, agentName)
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws/mailbox] upgrade error: %v", err)
		return
	}

	// Reuse streamFileLines — mailbox is just another append-only file.
	go streamFileLines(conn, mailboxPath)
}

// ensure mailboxPollInterval is used (avoids unused import if inlined later).
var _ = mailboxPollInterval
var _ = time.Second
