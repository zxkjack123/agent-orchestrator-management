package handler

import (
	"net/http"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
)

// SessionsHandler handles /api/v1/projects/{id}/sessions endpoints.
type SessionsHandler struct {
	app      *app.App
	registry ProjectLookup
}

// NewSessionsHandler creates the handler.
func NewSessionsHandler(a *app.App, reg ProjectLookup) *SessionsHandler {
	return &SessionsHandler{app: a, registry: reg}
}

// List handles GET /api/v1/projects/{id}/sessions.
func (h *SessionsHandler) List(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}

	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	svc, sqlDB, err := h.app.OpenSessionService(result.DBPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer sqlDB.Close()

	records, err := svc.ListByProject(result.Project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Only show the active=true query param filter when requested.
	activeOnly := r.URL.Query().Get("active") == "true"

	out := make([]dto.Session, 0, len(records))
	for _, s := range records {
		if activeOnly && !isActiveStatus(s.Status) {
			continue
		}
		out = append(out, dto.Session{
			ID:        s.ID,
			AgentName: s.AgentName,
			Status:    s.Status,
			TaskID:    s.TaskID,
			TmuxPane:  s.TmuxPane,
			CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeJSON(w, out)
}

// Stop handles DELETE /api/v1/projects/{id}/sessions/{sid}.
func (h *SessionsHandler) Stop(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}

	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	svc, sqlDB, err := h.app.OpenSessionService(result.DBPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer sqlDB.Close()

	sid := r.PathValue("sid")
	record, err := svc.Get(sid)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	if record.TmuxPane != "" {
		_ = h.app.Tmux.KillPaneAndDescendants(record.TmuxPane)
	}
	if _, err := svc.Stop(*record); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isActiveStatus(status string) bool {
	switch status {
	case "Working", "Idle", "WaitingApproval", "WaitingHandoff", "Booting":
		return true
	}
	return false
}
