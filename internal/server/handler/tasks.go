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
	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
)

// TasksHandler handles /api/v1/projects/{id}/tasks endpoints.
type TasksHandler struct {
	app      *app.App
	registry ProjectLookup
}

// NewTasksHandler creates the handler.
func NewTasksHandler(a *app.App, reg ProjectLookup) *TasksHandler {
	return &TasksHandler{app: a, registry: reg}
}

// List handles GET /api/v1/projects/{id}/tasks.
func (h *TasksHandler) List(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	svc, sqlDB, err := h.app.OpenTaskService(result.DBPath)
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

	out := make([]dto.Task, 0, len(records))
	for _, t := range records {
		out = append(out, dto.Task{
			ID:             t.ID,
			Title:          t.Title,
			Description:    t.Description,
			Status:         t.Status,
			Mode:           t.Mode,
			Priority:       t.Priority,
			PreferredAgent: t.PreferredAgent,
			PreferredRole:  t.PreferredRole,
			CreatedAt:      t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:      t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeJSON(w, out)
}

// GetOne handles GET /api/v1/projects/{id}/tasks/{tid}.
func (h *TasksHandler) GetOne(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	svc, sqlDB, err := h.app.OpenTaskService(result.DBPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer sqlDB.Close()

	t, err := svc.Get(r.PathValue("tid"))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	writeJSON(w, dto.Task{
		ID:             t.ID,
		Title:          t.Title,
		Description:    t.Description,
		Status:         t.Status,
		Mode:           t.Mode,
		Priority:       t.Priority,
		PreferredAgent: t.PreferredAgent,
		PreferredRole:  t.PreferredRole,
		CreatedAt:      t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Signal handles POST /api/v1/projects/{id}/tasks/{tid}/signal.
// Body: { "signal": "task.completed" }
func (h *TasksHandler) Signal(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req struct {
		Signal string `json:"signal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Signal) == "" {
		writeError(w, http.StatusBadRequest, "signal is required")
		return
	}

	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot locate aom binary")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "task", "signal", r.PathValue("tid"), req.Signal)
	cmd.Dir = proj.Path
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "output": strings.TrimSpace(string(out))})
}

// Accept handles POST /api/v1/projects/{id}/tasks/{tid}/accept.
// Body: { "force": false }
func (h *TasksHandler) Accept(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req struct {
		Force bool `json:"force"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot locate aom binary")
		return
	}

	args := []string{"task", "accept", r.PathValue("tid")}
	if req.Force {
		args = append(args, "--force")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = proj.Path
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "output": strings.TrimSpace(string(out))})
}

// Create handles POST /api/v1/projects/{id}/tasks.
// Body: { "title": "...", "description": "...", "mode": "Direct", "agent": "...", "role": "..." }
func (h *TasksHandler) Create(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Mode        string `json:"mode"`
		Agent       string `json:"agent"`
		Role        string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot locate aom binary")
		return
	}

	args := []string{"task", "create", req.Title}
	if req.Description != "" {
		args = append(args, "--description", req.Description)
	}
	if req.Mode != "" {
		args = append(args, "--mode", req.Mode)
	}
	if req.Agent != "" {
		args = append(args, "--agent", req.Agent)
	}
	if req.Role != "" {
		args = append(args, "--role", req.Role)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = proj.Path
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, map[string]string{"status": "created", "output": strings.TrimSpace(string(out))})
}

// Close handles POST /api/v1/projects/{id}/tasks/{tid}/close.
func (h *TasksHandler) Close(w http.ResponseWriter, r *http.Request) {
	h.runTaskSubCmd(w, r, "close")
}

// Cancel handles POST /api/v1/projects/{id}/tasks/{tid}/cancel.
func (h *TasksHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	h.runTaskSubCmd(w, r, "cancel")
}

func (h *TasksHandler) runTaskSubCmd(w http.ResponseWriter, r *http.Request, subCmd string) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot locate aom binary")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "task", subCmd, r.PathValue("tid"))
	cmd.Dir = proj.Path
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "output": strings.TrimSpace(string(out))})
}
