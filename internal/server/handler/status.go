package handler

import (
	"net/http"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
)

// StatusHandler handles GET /api/v1/projects/{id}/status.
type StatusHandler struct {
	app      *app.App
	registry ProjectLookup
}

// NewStatusHandler creates the handler.
func NewStatusHandler(a *app.App, reg ProjectLookup) *StatusHandler {
	return &StatusHandler{app: a, registry: reg}
}

// Get handles GET /api/v1/projects/{id}/status.
// Returns a dashboard-friendly summary: agents list + active/idle counts.
func (h *StatusHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	paneByAgent := make(map[string]string)
	statusByAgent := make(map[string]string)
	for _, s := range sessions {
		if s.TmuxPane != "" {
			paneByAgent[s.AgentName] = s.TmuxPane
			statusByAgent[s.AgentName] = s.Status
		}
	}

	agents := make([]dto.Agent, 0, len(result.Agents))
	var activeCount, idleCount int
	for _, a := range result.Agents {
		st := statusByAgent[a.Name]
		agents = append(agents, dto.Agent{
			Name:     a.Name,
			Role:     a.Role,
			Runtime:  a.Runtime,
			Enabled:  a.Enabled,
			TmuxPane: paneByAgent[a.Name],
			Status:   st,
		})
		switch st {
		case "Working", "WaitingApproval", "WaitingHandoff", "Booting":
			activeCount++
		case "Idle":
			idleCount++
		}
	}

	writeJSON(w, dto.ProjectStatus{
		ProjectID:   result.Project.ID,
		ProjectName: result.Project.Name,
		Agents:      agents,
		ActiveCount: activeCount,
		IdleCount:   idleCount,
	})
}
