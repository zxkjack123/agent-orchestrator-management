package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
)

// ProjectActionsHandler handles project-wide action endpoints.
type ProjectActionsHandler struct {
	app      *app.App
	registry ProjectLookup
}

// NewProjectActionsHandler creates the handler.
func NewProjectActionsHandler(a *app.App, reg ProjectLookup) *ProjectActionsHandler {
	return &ProjectActionsHandler{app: a, registry: reg}
}

// Channel handles POST /api/v1/projects/{id}/channel.
// Body: { "message": "..." }
func (h *ProjectActionsHandler) Channel(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	h.runCmd(w, r, proj.Path, 30*time.Second, "channel", "append", req.Message)
}

// Broadcast handles POST /api/v1/projects/{id}/broadcast.
// Body: { "message": "..." }
func (h *ProjectActionsHandler) Broadcast(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	h.runCmd(w, r, proj.Path, 60*time.Second, "broadcast", req.Message)
}

// ChannelHistory handles GET /api/v1/projects/{id}/channel.
// Returns the last N lines of channel.md as a JSON array.
func (h *ProjectActionsHandler) ChannelHistory(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	channelPath := proj.Path + "/.aom/channel.md"
	data, err := os.ReadFile(channelPath)
	if err != nil {
		// File doesn't exist yet — return empty
		writeJSON(w, map[string]any{"lines": []string{}})
		return
	}
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		if t := strings.TrimSpace(l); t != "" {
			lines = append(lines, t)
		}
	}
	// Return last 200 lines
	if len(lines) > 200 {
		lines = lines[len(lines)-200:]
	}
	writeJSON(w, map[string]any{"lines": lines})
}

// MailboxHistory handles GET /api/v1/projects/{id}/mailbox/{agent}.
// Returns the last 200 lines of the agent's mailbox file.
func (h *ProjectActionsHandler) MailboxHistory(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	agentName := r.PathValue("agent")
	if strings.TrimSpace(agentName) == "" {
		writeError(w, http.StatusBadRequest, "agent name required")
		return
	}
	mailboxPath := proj.Path + "/.aom/mailbox/" + agentName + ".md"
	data, err := os.ReadFile(mailboxPath)
	if err != nil {
		writeJSON(w, map[string]any{"lines": []string{}})
		return
	}
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		if t := strings.TrimSpace(l); t != "" {
			lines = append(lines, t)
		}
	}
	if len(lines) > 200 {
		lines = lines[len(lines)-200:]
	}
	writeJSON(w, map[string]any{"lines": lines})
}

// PauseAll handles POST /api/v1/projects/{id}/pause-all.
func (h *ProjectActionsHandler) PauseAll(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	h.runCmd(w, r, proj.Path, 30*time.Second, "pause-all")
}

// ResumeAll handles POST /api/v1/projects/{id}/resume-all.
func (h *ProjectActionsHandler) ResumeAll(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	h.runCmd(w, r, proj.Path, 30*time.Second, "resume-all")
}

func (h *ProjectActionsHandler) runCmd(w http.ResponseWriter, r *http.Request, projPath string, timeout time.Duration, args ...string) {
	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot locate aom binary")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = projPath
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "output": strings.TrimSpace(string(out))})
}
