package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
)

// RolesHandler handles role and class management endpoints.
type RolesHandler struct {
	app      *app.App
	registry ProjectLookup
}

// NewRolesHandler creates the handler.
func NewRolesHandler(a *app.App, reg ProjectLookup) *RolesHandler {
	return &RolesHandler{app: a, registry: reg}
}

// ListRoles handles GET /api/v1/projects/{id}/roles.
func (h *RolesHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	roles, err := project.ListRoles(result.AOMPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]dto.RoleDefinition, 0, len(roles))
	for _, ro := range roles {
		out = append(out, dto.RoleDefinition{
			Name:                  ro.Name,
			Class:                 ro.Class,
			WorktreeMode:          ro.WorktreeMode,
			CheckpointExpectation: ro.CheckpointExpectation,
			DefaultSessionMode:    ro.DefaultSessionMode,
			AgentsUsing:           ro.AgentsUsing,
			Description:           ro.Description,
		})
	}
	writeJSON(w, out)
}

// CreateRole handles POST /api/v1/projects/{id}/roles.
func (h *RolesHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req dto.CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	params := project.RoleCreateParams{
		Class:                 req.Class,
		WorktreeMode:          req.WorktreeMode,
		CheckpointExpectation: req.CheckpointExpectation,
		DefaultSessionMode:    req.DefaultSessionMode,
		Description:           req.Description,
	}
	if err := project.CreateRole(result.AOMPath, req.Name, params); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	ro, _ := project.GetRole(result.AOMPath, req.Name)
	writeJSON(w, dto.RoleDefinition{
		Name:                  ro.Name,
		Class:                 ro.Class,
		WorktreeMode:          ro.WorktreeMode,
		CheckpointExpectation: ro.CheckpointExpectation,
		DefaultSessionMode:    ro.DefaultSessionMode,
		AgentsUsing:           ro.AgentsUsing,
	})
}

// GetRole handles GET /api/v1/projects/{id}/roles/{name}.
func (h *RolesHandler) GetRole(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ro, err := project.GetRole(result.AOMPath, name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, dto.RoleDefinition{
		Name:                  ro.Name,
		Class:                 ro.Class,
		WorktreeMode:          ro.WorktreeMode,
		CheckpointExpectation: ro.CheckpointExpectation,
		DefaultSessionMode:    ro.DefaultSessionMode,
		AgentsUsing:           ro.AgentsUsing,
	})
}

// UpdateRole handles PUT /api/v1/projects/{id}/roles/{name}.
func (h *RolesHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	var req dto.UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	params := project.RoleUpdateParams{
		Class:                 req.Class,
		WorktreeMode:          req.WorktreeMode,
		CheckpointExpectation: req.CheckpointExpectation,
		DefaultSessionMode:    req.DefaultSessionMode,
		Description:           req.Description,
	}
	if err := project.UpdateRole(result.AOMPath, name, params); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	ro, _ := project.GetRole(result.AOMPath, name)
	writeJSON(w, dto.RoleDefinition{
		Name:                  ro.Name,
		Class:                 ro.Class,
		WorktreeMode:          ro.WorktreeMode,
		CheckpointExpectation: ro.CheckpointExpectation,
		DefaultSessionMode:    ro.DefaultSessionMode,
		AgentsUsing:           ro.AgentsUsing,
	})
}

// DeleteRole handles DELETE /api/v1/projects/{id}/roles/{name}.
func (h *RolesHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := project.DeleteRole(result.AOMPath, name); err != nil {
		code := http.StatusNotFound
		if strings.Contains(err.Error(), "agent") {
			code = http.StatusConflict
		}
		writeError(w, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PreviewRole handles GET /api/v1/projects/{id}/roles/{name}/preview.
func (h *RolesHandler) PreviewRole(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	runtime := r.URL.Query().Get("runtime")
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rendered, err := project.PreviewRoleProfile(result.AOMPath, name, runtime)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, dto.ClassPreviewResponse{Rendered: rendered})
}

// ListClasses handles GET /api/v1/projects/{id}/classes.
func (h *RolesHandler) ListClasses(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	classes, err := project.ListClasses(result.AOMPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]dto.ClassInfo, 0, len(classes))
	for _, c := range classes {
		out = append(out, dto.ClassInfo{
			Name:        c.Name,
			Source:      string(c.Source),
			RolesUsing:  c.RolesUsing,
			Description: c.Description,
		})
	}
	writeJSON(w, out)
}

// GetClass handles GET /api/v1/projects/{id}/classes/{name}.
func (h *RolesHandler) GetClass(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	detail, err := project.GetClassTemplate(result.AOMPath, name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, dto.ClassDetail{
		ClassInfo: dto.ClassInfo{
			Name:        detail.Name,
			Source:      string(detail.Source),
			RolesUsing:  detail.RolesUsing,
			Description: detail.Description,
		},
		Content:     detail.Content,
		IsProtected: detail.IsProtected,
	})
}

// SetClass handles PUT /api/v1/projects/{id}/classes/{name}.
func (h *RolesHandler) SetClass(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	var req dto.SetClassRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := project.SetClassTemplate(result.AOMPath, name, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail, err := project.GetClassTemplate(result.AOMPath, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, dto.ClassDetail{
		ClassInfo: dto.ClassInfo{
			Name:        detail.Name,
			Source:      string(detail.Source),
			RolesUsing:  detail.RolesUsing,
			Description: detail.Description,
		},
		Content:     detail.Content,
		IsProtected: detail.IsProtected,
	})
}

// DeleteClass handles DELETE /api/v1/projects/{id}/classes/{name}.
func (h *RolesHandler) DeleteClass(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := project.DeleteClassTemplate(result.AOMPath, name); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PreviewClass handles GET /api/v1/projects/{id}/classes/{name}/preview.
func (h *RolesHandler) PreviewClass(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	q := r.URL.Query()
	runtime := q.Get("runtime")
	roleName := q.Get("role")
	agentName := q.Get("agent")

	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rendered, err := project.PreviewClassProfile(result.AOMPath, name, roleName, agentName, runtime)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, dto.ClassPreviewResponse{Rendered: rendered})
}

// GetSystemTemplate handles GET /api/v1/system-template.
func GetSystemTemplate(w http.ResponseWriter, _ *http.Request) {
	content, err := project.GetSystemTemplate()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, dto.SystemTemplateResponse{Content: content})
}
