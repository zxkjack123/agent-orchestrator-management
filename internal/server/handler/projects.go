package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
)

// ProjectRegistry is the full interface the projects handler needs.
type ProjectRegistry interface {
	ProjectLookup
	List() []dto.Project
	Add(path string) (dto.Project, error)
	Remove(id string) error
}

// ProjectsHandler handles /api/v1/projects endpoints.
type ProjectsHandler struct {
	registry ProjectRegistry
}

// NewProjectsHandler creates a handler backed by the given registry.
func NewProjectsHandler(reg ProjectRegistry) *ProjectsHandler {
	return &ProjectsHandler{registry: reg}
}

// List handles GET /api/v1/projects.
func (h *ProjectsHandler) List(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.registry.List())
}

// Add handles POST /api/v1/projects.
func (h *ProjectsHandler) Add(w http.ResponseWriter, r *http.Request) {
	var req dto.AddProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	proj, err := h.registry.Add(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, proj)
}

// Remove handles DELETE /api/v1/projects/{id}.
func (h *ProjectsHandler) Remove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.registry.Remove(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// InitProject handles POST /api/v1/projects/init
// Runs `aom project init <name> --repo .` in the given directory (non-interactive).
func (h *ProjectsHandler) InitProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Path = strings.TrimSpace(req.Path)
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	if !filepath.IsAbs(req.Path) {
		writeError(w, http.StatusBadRequest, "path must be absolute")
		return
	}

	// Derive project name from the last path component.
	name := filepath.Base(req.Path)
	if name == "." || name == "/" {
		name = "project"
	}

	exe, err := os.Executable()
	if err != nil {
		exe = "aom"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, exe, "project", "init", name, "--repo", ".")
	cmd.Dir = req.Path
	cmd.Stdin = nil // non-interactive: skips agent selection prompt

	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		writeJSON(w, map[string]string{"status": "error", "output": output})
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "output": output})
}
