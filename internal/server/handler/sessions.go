package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/session"
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
			ID:         s.ID,
			AgentName:  s.AgentName,
			Status:     s.Status,
			TaskID:     s.TaskID,
			TmuxPane:   s.TmuxPane,
			CreatedAt:  s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Persistent: s.Persistent,
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

// Spawn handles POST /api/v1/projects/{id}/sessions.
// It shells out to the current aom binary with CWD = project path.
func (h *SessionsHandler) Spawn(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req dto.SpawnSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Agent) == "" {
		writeError(w, http.StatusBadRequest, "agent is required")
		return
	}

	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot locate aom binary")
		return
	}

	args := []string{"session", "spawn", req.Agent}
	if req.TaskID != "" {
		args = append(args, "--task", req.TaskID)
	}
	if req.Mode == "real" {
		args = append(args, "--real")
	} else {
		args = append(args, "--mock")
	}
	if req.Persistent {
		args = append(args, "--persistent")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = proj.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, dto.SpawnSessionResponse{Status: "spawned", Output: strings.TrimSpace(string(out))})
}

// Get handles GET /api/v1/projects/{id}/sessions/{sid}.
func (h *SessionsHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	record, err := svc.Get(r.PathValue("sid"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, dto.Session{
		ID:         record.ID,
		AgentName:  record.AgentName,
		Status:     record.Status,
		TaskID:     record.TaskID,
		TmuxPane:   record.TmuxPane,
		CreatedAt:  record.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Persistent: record.Persistent,
	})
}

// Archive handles POST /api/v1/projects/{id}/sessions/{sid}/archive.
func (h *SessionsHandler) Archive(w http.ResponseWriter, r *http.Request) {
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

	record, err := svc.Get(r.PathValue("sid"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	updated, err := svc.Archive(*record)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, dto.Session{
		ID:         updated.ID,
		AgentName:  updated.AgentName,
		Status:     updated.Status,
		TaskID:     updated.TaskID,
		TmuxPane:   updated.TmuxPane,
		CreatedAt:  updated.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Persistent: updated.Persistent,
	})
}

// Send handles POST /api/v1/projects/{id}/sessions/{sid}/send.
// Writes a message to the target agent's mailbox file.
func (h *SessionsHandler) Send(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req dto.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
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

	record, err := svc.Get(r.PathValue("sid"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	from := strings.TrimSpace(req.From)
	if from == "" {
		from = "operator"
	}

	if err := appendMailbox(proj.Path, record.AgentName, req.Message, from); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Notify the agent in-pane so they don't have to poll their inbox.
	notifyAgentInboxPane(h.app, result, record.AgentName, from, req.Message)

	w.WriteHeader(http.StatusNoContent)
}

// Resume handles POST /api/v1/projects/{id}/sessions/{sid}/resume.
// Shells out to `aom session resume <agent-name>`.
func (h *SessionsHandler) Resume(w http.ResponseWriter, r *http.Request) {
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

	record, err := svc.Get(r.PathValue("sid"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot locate aom binary")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	// Pass the session ID directly — not AgentName — so the CLI resumes exactly
	// the session the caller selected. Using AgentName causes loadSessionByIdentifier
	// to pick the newest session for that agent, ignoring which one was requested.
	cmd := exec.CommandContext(ctx, exe, "session", "resume", record.ID)
	cmd.Dir = proj.Path
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, dto.SpawnSessionResponse{Status: "resumed", Output: strings.TrimSpace(string(out))})
}

// Approve handles POST /api/v1/projects/{id}/sessions/{sid}/approve.
func (h *SessionsHandler) Approve(w http.ResponseWriter, r *http.Request) {
	h.runApprovalCmd(w, r, "approve")
}

// Deny handles POST /api/v1/projects/{id}/sessions/{sid}/deny.
func (h *SessionsHandler) Deny(w http.ResponseWriter, r *http.Request) {
	h.runApprovalCmd(w, r, "deny")
}

func (h *SessionsHandler) runApprovalCmd(w http.ResponseWriter, r *http.Request, action string) {
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

	record, err := svc.Get(r.PathValue("sid"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot locate aom binary")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, action, record.AgentName)
	cmd.Dir = proj.Path
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "output": strings.TrimSpace(string(out))})
}

// Recover handles POST /api/v1/projects/{id}/sessions/{sid}/recover.
func (h *SessionsHandler) Recover(w http.ResponseWriter, r *http.Request) {
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

	record, err := svc.Get(r.PathValue("sid"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot locate aom binary")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "session", "recover", record.AgentName)
	cmd.Dir = proj.Path
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, dto.SpawnSessionResponse{Status: "recovered", Output: strings.TrimSpace(string(out))})
}

// Isolate handles POST /api/v1/projects/{id}/sessions/isolate.
//
// If the agent's pane is in the team session (aom-team-*) it was placed there
// by "aom team view". In that case we move the pane via join-pane so the agent
// process keeps running — no restart, no lost context.
//
// Otherwise (agent was in a shared --grid window) we fall back to the original
// behaviour: provision a dedicated workspace and re-spawn the session.
func (h *SessionsHandler) Isolate(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req struct {
		Agent string `json:"agent"`
		Mode  string `json:"mode"` // "real" | "mock"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Agent) == "" {
		writeError(w, http.StatusBadRequest, "agent is required")
		return
	}
	mode := req.Mode
	if mode != "real" {
		mode = "mock"
	}

	result, err := h.app.Projects.Open(proj.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	teamSession := "aom-team-" + result.SessionPrefix

	// Try join-pane isolation when the pane lives in the team session.
	if moved, msg := h.isolatePaneFromTeam(req.Agent, teamSession, result.DBPath, result.Project.ID); moved {
		writeJSON(w, dto.SpawnSessionResponse{Status: "isolated", Output: msg})
		return
	}

	// Fallback: re-spawn the agent in its own dedicated session (--grid / shared window case).
	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot locate aom binary")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	provCmd := exec.CommandContext(ctx, exe, "agent", "provision", req.Agent)
	provCmd.Dir = proj.Path
	provOut, provErr := provCmd.CombinedOutput()

	spawnArgs := []string{"session", "spawn", req.Agent, "--" + mode}
	spawnCmd := exec.CommandContext(ctx, exe, spawnArgs...)
	spawnCmd.Dir = proj.Path
	spawnOut, spawnErr := spawnCmd.CombinedOutput()
	if spawnErr != nil {
		writeError(w, http.StatusInternalServerError,
			strings.TrimSpace(string(provOut))+"\n"+strings.TrimSpace(string(spawnOut)))
		return
	}

	combined := strings.TrimSpace(string(provOut)) + "\n" + strings.TrimSpace(string(spawnOut))
	_ = provErr
	writeJSON(w, dto.SpawnSessionResponse{Status: "isolated", Output: strings.TrimSpace(combined)})
}

// isolatePaneFromTeam moves agentName's pane out of teamSession into its own session.
func (h *SessionsHandler) isolatePaneFromTeam(agentName, teamSession, dbPath, projectID string) (bool, string) {
	svc, sqlDB, err := h.app.OpenSessionService(dbPath)
	if err != nil {
		return false, ""
	}
	defer sqlDB.Close()

	sessions, _ := svc.ListByProject(projectID)
	activeStatuses := map[string]bool{"Idle": true, "Working": true, "Booting": true, "WaitingApproval": true, "WaitingHandoff": true}

	// Find the agent's most-recent live pane.
	var activePane string
	for i := len(sessions) - 1; i >= 0; i-- {
		s := sessions[i]
		if s.AgentName != agentName || strings.TrimSpace(s.TmuxPane) == "" {
			continue
		}
		if !activeStatuses[s.Status] {
			continue
		}
		if alive, _ := h.app.Tmux.PaneExists(s.TmuxPane); alive {
			activePane = s.TmuxPane
			break
		}
	}
	if activePane == "" {
		return false, ""
	}

	// Confirm the pane is in the team session.
	teamPanes, _ := h.app.Tmux.ListPanesInSession(teamSession)
	inTeam := false
	for _, p := range teamPanes {
		if p == activePane {
			inTeam = true
			break
		}
	}
	if !inTeam {
		return false, ""
	}

	// Create a dedicated session and move the pane there.
	sessionPrefix := strings.TrimPrefix(teamSession, "aom-team-")
	dedicatedName := "aom-iso-" + sanitizeForSession(agentName) + "-" + sessionPrefix
	if ex, _ := h.app.Tmux.SessionExists(dedicatedName); ex {
		dedicatedName = fmt.Sprintf("%s-%d", dedicatedName, time.Now().UnixMilli())
	}
	if err := h.app.Tmux.NewDetachedSession(dedicatedName); err != nil {
		return false, ""
	}
	blankPanes, _ := h.app.Tmux.ListPanesInSession(dedicatedName)
	wins, _ := h.app.Tmux.ListWindowsInSession(dedicatedName)
	winTarget := dedicatedName
	if len(wins) > 0 {
		winTarget = dedicatedName + ":" + wins[0].ID
	}
	if err := h.app.Tmux.JoinPane(activePane, winTarget); err != nil {
		_ = h.app.Tmux.KillSession(dedicatedName)
		return false, ""
	}
	if len(blankPanes) > 0 {
		_ = h.app.Tmux.KillPane(blankPanes[0])
	}
	return true, agentName + " moved to " + dedicatedName + " (process kept running)"
}

func sanitizeForSession(name string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(name) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			b.WriteRune(c)
		} else {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// notifyAgentInboxPane sends a tmux notification to the agent's live pane
// so they are alerted about a new DM without having to poll their inbox.
func notifyAgentInboxPane(a *app.App, result *project.OpenResult, agentName, from, message string) {
	svc, sqlDB, err := a.OpenSessionService(result.DBPath)
	if err != nil {
		return
	}
	defer sqlDB.Close()

	all, err := svc.ListByProject(result.Project.ID)
	if err != nil {
		return
	}

	notification := fmt.Sprintf("[DM from %s] %s", from, message)
	for i := len(all) - 1; i >= 0; i-- {
		s := all[i]
		if s.AgentName != agentName {
			continue
		}
		if strings.TrimSpace(s.TmuxPane) == "" {
			continue
		}
		switch s.Status {
		case "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Booting":
		default:
			continue
		}
		alive, _ := a.Tmux.PaneExists(s.TmuxPane)
		if !alive {
			continue
		}
		_ = a.Tmux.SendKeys(s.TmuxPane, notification)
		return
	}
}

// appendMailbox appends a message to the agent's .aom/mailbox/<agent>.md file.
func appendMailbox(repoPath, agentName, message, fromSender string) error {
	now := time.Now()
	path := filepath.Join(repoPath, ".aom", "mailbox", agentName+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create mailbox dir: %w", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		header := fmt.Sprintf("# Mailbox: %s\n\n## Messages\n\n", agentName)
		if err := os.WriteFile(path, []byte(header), 0o644); err != nil {
			return fmt.Errorf("create mailbox file: %w", err)
		}
	}
	msgID := "MSG-" + strconv.FormatInt(now.UnixNano(), 10)
	entry := fmt.Sprintf("### %s | %s | from: %s\n%s\n\n",
		now.Format(time.RFC3339), msgID, fromSender, message)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open mailbox: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}

// isActiveStatus delegates to the canonical implementation in the session package.
func isActiveStatus(status string) bool { return session.IsActiveStatus(status) }
