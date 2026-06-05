package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/config"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/session"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/tmux"
	"gopkg.in/yaml.v3"
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

	// Index the most-recent LIVE pane per agent: walk newest→oldest, skip dead panes.
	paneByAgent := make(map[string]string)
	statusByAgent := make(map[string]string)
	for i := len(sessions) - 1; i >= 0; i-- {
		s := sessions[i]
		if paneByAgent[s.AgentName] != "" {
			continue // already found a live pane for this agent
		}
		if strings.TrimSpace(s.TmuxPane) == "" {
			continue
		}
		if alive, _ := h.app.Tmux.PaneExists(s.TmuxPane); alive {
			paneByAgent[s.AgentName] = s.TmuxPane
			statusByAgent[s.AgentName] = s.Status
		}
	}

	// Fetch workspace paths from DB (best-effort — empty map on error).
	workspaceByAgent := make(map[string]string)
	if agentRepo, agentDB, err := h.app.OpenAgentRepository(result.DBPath); err == nil {
		defer agentDB.Close()
		if records, err := agentRepo.ListByProjectID(result.Project.ID); err == nil {
			for _, rec := range records {
				workspaceByAgent[rec.Name] = rec.WorkspacePath
			}
		}
	}

	out := make([]dto.Agent, 0, len(result.Agents))
	for _, a := range result.Agents {
		pane := paneByAgent[a.Name]
		shared := isSharedPane(h.app.Tmux, pane)
		out = append(out, dto.Agent{
			Name:            a.Name,
			Role:            a.Role,
			Runtime:         a.Runtime,
			Enabled:         a.Enabled,
			Model:           a.Model,
			TmuxPane:        pane,
			Status:          statusByAgent[a.Name],
			IsSharedSession: shared,
			WorkspacePath:   workspaceByAgent[a.Name],
		})
	}
	writeJSON(w, out)
}

// Add handles POST /api/v1/projects/{id}/agents.
func (h *AgentsHandler) Add(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req dto.AddAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name, role, and runtime are required")
		return
	}

	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	af, err := loadAgentsFile(result.AOMPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if af.Agents == nil {
		af.Agents = make(map[string]config.AgentConfig)
	}
	af.Agents[req.Name] = config.AgentConfig{
		Runtime: req.Runtime,
		Role:    req.Role,
		Enabled: req.Enabled,
		Model:   req.Model,
	}
	if err := config.SaveAgentsFile(result.AOMPath, af); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, dto.Agent{
		Name:    req.Name,
		Role:    req.Role,
		Runtime: req.Runtime,
		Enabled: req.Enabled,
		Model:   req.Model,
	})
}

// Update handles PUT /api/v1/projects/{id}/agents/{name}.
func (h *AgentsHandler) Update(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	var req dto.UpdateAgentRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	af, err := loadAgentsFile(result.AOMPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ac, exists := af.Agents[name]
	if !exists {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if req.Model != nil {
		ac.Model = *req.Model
	}
	if req.Enabled != nil {
		ac.Enabled = *req.Enabled
	}
	af.Agents[name] = ac
	if err := config.SaveAgentsFile(result.AOMPath, af); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, dto.Agent{
		Name:    name,
		Role:    ac.Role,
		Runtime: ac.Runtime,
		Enabled: ac.Enabled,
		Model:   ac.Model,
	})
}

// Remove handles DELETE /api/v1/projects/{id}/agents/{name}.
func (h *AgentsHandler) Remove(w http.ResponseWriter, r *http.Request) {
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

	af, err := loadAgentsFile(result.AOMPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, exists := af.Agents[name]; !exists {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	delete(af.Agents, name)
	if err := config.SaveAgentsFile(result.AOMPath, af); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Provision handles POST /api/v1/projects/{id}/agents/{name}/provision.
// Shells out to `aom agent provision <name>` to create a permanent workspace.
func (h *AgentsHandler) Provision(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "agent name is required")
		return
	}

	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot locate aom binary")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "agent", "provision", name)
	cmd.Dir = proj.Path
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, map[string]string{"status": "provisioned", "output": strings.TrimSpace(string(out))})
}

// GetProfile handles GET /api/v1/projects/{id}/agents/{name}/profile.
// Returns the full content of the agent's profile.md.
func (h *AgentsHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "agent name is required")
		return
	}
	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	profile, err := project.ReadAgentProfile(result.AOMPath, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if profile == "" {
		writeError(w, http.StatusNotFound, "profile not found for agent "+name)
		return
	}
	writeJSON(w, map[string]string{"profile": profile})
}

// GetInstructions handles GET /api/v1/projects/{id}/agents/{name}/instructions.
// Returns the content of the "## Custom Instructions" section from the agent's profile.md.
func (h *AgentsHandler) GetInstructions(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "agent name is required")
		return
	}

	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot open project: "+err.Error())
		return
	}

	profile, err := project.ReadAgentProfile(result.AOMPath, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if profile == "" {
		writeError(w, http.StatusNotFound, "profile not found for agent "+name)
		return
	}

	instructions := project.GetCustomInstructions(profile)
	writeJSON(w, map[string]string{"instructions": instructions})
}

// SetInstructions handles PUT /api/v1/projects/{id}/agents/{name}/instructions.
// Replaces the "## Custom Instructions" section in the agent's profile.md.
// An empty instructions string clears the section (resets to placeholder comment).
func (h *AgentsHandler) SetInstructions(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "agent name is required")
		return
	}

	var req struct {
		Instructions string `json:"instructions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot open project: "+err.Error())
		return
	}

	profile, err := project.ReadAgentProfile(result.AOMPath, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if profile == "" {
		writeError(w, http.StatusNotFound, "profile not found for agent "+name)
		return
	}

	updated := project.SetCustomInstructions(profile, req.Instructions)
	if err := project.WriteAgentProfile(result.AOMPath, name, updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Check whether the agent currently has a live session so the caller can warn
	// the user that changes won't take effect until the next spawn.
	activeSession := agentHasLiveSession(h.app, result.DBPath, result.Project.ID, name)
	writeJSON(w, dto.SetInstructionsResponse{Status: "ok", ActiveSession: activeSession})
}

// agentHasLiveSession returns true when the agent has at least one DB session
// with an active status AND a confirmed-live tmux pane.
// Errors are treated as false — a failed check never blocks the profile save.
func agentHasLiveSession(a *app.App, dbPath, projectID, agentName string) bool {
	svc, sessDB, err := a.OpenSessionService(dbPath)
	if err != nil {
		return false
	}
	defer sessDB.Close()
	records, err := svc.ActiveByAgent(projectID, agentName)
	if err != nil || len(records) == 0 {
		return false
	}
	for i := len(records) - 1; i >= 0; i-- {
		s := records[i]
		if strings.TrimSpace(s.TmuxPane) == "" || !session.IsActiveStatus(s.Status) {
			continue
		}
		if alive, _ := a.Tmux.PaneExists(s.TmuxPane); alive {
			return true
		}
	}
	return false
}

// isSharedPane returns true when the pane lives in a tmux window that has more
// than one pane — i.e. the agent shares a window with other agents. These agents
// should be migrated to dedicated sessions via "Isolate Session".
func isSharedPane(m *tmux.Manager, paneID string) bool {
	if strings.TrimSpace(paneID) == "" {
		return false
	}
	sessionName, windowID, err := m.PaneSessionInfo(paneID)
	if err != nil {
		return false
	}
	panes, err := m.ListPanesInWindow(sessionName + ":" + windowID)
	if err != nil {
		return false
	}
	return len(panes) > 1
}

// loadAgentsFile reads and parses .aom/agents.yaml from the given aomPath.
func loadAgentsFile(aomPath string) (config.AgentsFile, error) {
	data, err := os.ReadFile(filepath.Join(aomPath, "agents.yaml"))
	if err != nil {
		return config.AgentsFile{}, fmt.Errorf("read agents.yaml: %w", err)
	}
	var af config.AgentsFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return config.AgentsFile{}, fmt.Errorf("parse agents.yaml: %w", err)
	}
	return af, nil
}
