package handler

import (
	"net/http"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
)

// AgentsHandler handles GET /api/v1/projects/{id}/agents.
type AgentsHandler struct {
	app      *app.App
	registry ProjectLookup
}

// NewAgentsHandler creates the handler.
func NewAgentsHandler(a *app.App, reg ProjectLookup) *AgentsHandler {
	return &AgentsHandler{app: a, registry: reg}
}

// List handles GET /api/v1/projects/{id}/agents.
// Returns each agent plus its current tmux pane and session status if active.
func (h *AgentsHandler) List(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}

	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot open project: "+err.Error())
		return
	}

	svc, sqlDB, err := h.app.OpenSessionService(result.DBPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer sqlDB.Close()

	sessions, _ := svc.ListByProject(result.Project.ID)

	// Index the most-recent live pane per agent (newest session wins).
	paneByAgent := make(map[string]string)
	statusByAgent := make(map[string]string)
	for _, s := range sessions {
		if strings.TrimSpace(s.TmuxPane) != "" {
			paneByAgent[s.AgentName] = s.TmuxPane
			statusByAgent[s.AgentName] = s.Status
		}
	}

	out := make([]dto.Agent, 0, len(result.Agents))
	for _, a := range result.Agents {
		out = append(out, dto.Agent{
			Name:     a.Name,
			Role:     a.Role,
			Runtime:  a.Runtime,
			Enabled:  a.Enabled,
			TmuxPane: paneByAgent[a.Name],
			Status:   statusByAgent[a.Name],
		})
	}
	writeJSON(w, out)
}
