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
)

// ExtrasHandler handles miscellaneous project-level endpoints.
type ExtrasHandler struct {
	registry ProjectLookup
}

// NewExtrasHandler creates the handler.
func NewExtrasHandler(reg ProjectLookup) *ExtrasHandler {
	return &ExtrasHandler{registry: reg}
}

// runCmd shells out to the aom binary with the given args in projPath.
func (h *ExtrasHandler) runCmd(w http.ResponseWriter, r *http.Request, projPath string, timeout time.Duration, args ...string) {
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

// ─── Task artifact ────────────────────────────────────────────────────────────

// TaskArtifact handles GET /api/v1/projects/{id}/tasks/{tid}/artifact.
// Returns content of task.md, handoff.md and state.md from .aom/tasks/{tid}/.
func (h *ExtrasHandler) TaskArtifact(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	tid := r.PathValue("tid")
	if strings.TrimSpace(tid) == "" {
		writeError(w, http.StatusBadRequest, "task id required")
		return
	}
	base := filepath.Join(proj.Path, ".aom", "tasks", tid)
	result := map[string]string{}
	for _, name := range []string{"task.md", "handoff.md", "state.md"} {
		if data, err := os.ReadFile(filepath.Join(base, name)); err == nil {
			result[name] = string(data)
		}
	}
	writeJSON(w, result)
}

// ─── Task requests ────────────────────────────────────────────────────────────

type requestRecord struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	RequestedBy string `json:"requested_by"`
	ParentTask  string `json:"parent_task"`
	Priority    string `json:"priority"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
}

// ListRequests handles GET /api/v1/projects/{id}/requests.
func (h *ExtrasHandler) ListRequests(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	dir := filepath.Join(proj.Path, ".aom", "requests")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, []requestRecord{})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var out []requestRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.HasSuffix(e.Name(), ".archive.md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, parseRequestFile(string(data)))
	}
	if out == nil {
		out = []requestRecord{}
	}
	writeJSON(w, out)
}

// ApproveRequest handles POST /api/v1/projects/{id}/requests/{rid}/approve.
func (h *ExtrasHandler) ApproveRequest(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	h.runCmd(w, r, proj.Path, 60*time.Second, "task", "approve-request", r.PathValue("rid"))
}

// RejectRequest handles POST /api/v1/projects/{id}/requests/{rid}/reject.
func (h *ExtrasHandler) RejectRequest(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	h.runCmd(w, r, proj.Path, 30*time.Second, "task", "reject-request", r.PathValue("rid"))
}

func parseRequestFile(content string) requestRecord {
	var rec requestRecord
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "# Task Request: "):
			rec.Title = strings.TrimPrefix(line, "# Task Request: ")
		case strings.HasPrefix(line, "- ID: "):
			rec.ID = strings.TrimPrefix(line, "- ID: ")
		case strings.HasPrefix(line, "- Requested by: "):
			rec.RequestedBy = strings.TrimPrefix(line, "- Requested by: ")
		case strings.HasPrefix(line, "- Parent task: "):
			rec.ParentTask = strings.TrimPrefix(line, "- Parent task: ")
		case strings.HasPrefix(line, "- Priority: "):
			rec.Priority = strings.TrimPrefix(line, "- Priority: ")
		case strings.HasPrefix(line, "- Status: "):
			rec.Status = strings.TrimPrefix(line, "- Status: ")
		case strings.HasPrefix(line, "- Reason: "):
			rec.Reason = strings.TrimPrefix(line, "- Reason: ")
		}
	}
	return rec
}

// ─── Metrics ──────────────────────────────────────────────────────────────────

// Metrics handles GET /api/v1/projects/{id}/metrics.
func (h *ExtrasHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	h.runCmd(w, r, proj.Path, 30*time.Second, "metrics")
}

// ─── Doctor ───────────────────────────────────────────────────────────────────

type doctorCheck struct {
	Result  string `json:"result"` // pass | warn | fail | info
	Message string `json:"message"`
}

// Doctor handles POST /api/v1/projects/{id}/doctor.
func (h *ExtrasHandler) Doctor(w http.ResponseWriter, r *http.Request) {
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
	cmd := exec.CommandContext(ctx, exe, "doctor")
	cmd.Dir = proj.Path
	out, _ := cmd.CombinedOutput()
	raw := strings.TrimSpace(string(out))
	writeJSON(w, map[string]any{
		"output": raw,
		"checks": parseDoctorOutput(raw),
	})
}

func parseDoctorOutput(output string) []doctorCheck {
	var checks []doctorCheck
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "[PASS]"):
			checks = append(checks, doctorCheck{Result: "pass", Message: strings.TrimSpace(strings.TrimPrefix(line, "[PASS]"))})
		case strings.HasPrefix(line, "[WARN]"):
			checks = append(checks, doctorCheck{Result: "warn", Message: strings.TrimSpace(strings.TrimPrefix(line, "[WARN]"))})
		case strings.HasPrefix(line, "[FAIL]"):
			checks = append(checks, doctorCheck{Result: "fail", Message: strings.TrimSpace(strings.TrimPrefix(line, "[FAIL]"))})
		case strings.HasPrefix(line, "[INFO]"):
			checks = append(checks, doctorCheck{Result: "info", Message: strings.TrimSpace(strings.TrimPrefix(line, "[INFO]"))})
		}
	}
	if checks == nil {
		checks = []doctorCheck{}
	}
	return checks
}

// ─── Team Brief ───────────────────────────────────────────────────────────────

// TeamBriefGet handles GET /api/v1/projects/{id}/team-brief.
func (h *ExtrasHandler) TeamBriefGet(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	path := filepath.Join(proj.Path, ".aom", "team-brief.md")
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, map[string]string{"content": ""})
		return
	}
	writeJSON(w, map[string]string{"content": string(data)})
}

// TeamBriefPut handles PUT /api/v1/projects/{id}/team-brief.
// Body: { "content": "..." }
func (h *ExtrasHandler) TeamBriefPut(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	path := filepath.Join(proj.Path, ".aom", "team-brief.md")
	if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// TeamBriefGenerate handles POST /api/v1/projects/{id}/team-brief/generate.
// Runs `aom team brief` in the project directory to regenerate from current state.
func (h *ExtrasHandler) TeamBriefGenerate(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	aomBin, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot find aom binary")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, aomBin, "team", "brief")
	cmd.Dir = proj.Path
	if out, err := cmd.CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	// Return updated content.
	path := filepath.Join(proj.Path, ".aom", "team-brief.md")
	data, _ := os.ReadFile(path)
	writeJSON(w, map[string]string{"status": "ok", "content": string(data)})
}

// TeamBriefPush handles POST /api/v1/projects/{id}/team-brief/push.
// Runs `aom team brief --push` to broadcast to the team.
func (h *ExtrasHandler) TeamBriefPush(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	aomBin, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot find aom binary")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, aomBin, "team", "brief", "--push")
	cmd.Dir = proj.Path
	if out, err := cmd.CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// ─── Merge ────────────────────────────────────────────────────────────────────

// MergeCheck handles POST /api/v1/projects/{id}/merge/check.
// Body: { "task_id": "TASK-xxx" }
func (h *ExtrasHandler) MergeCheck(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TaskID) == "" {
		writeError(w, http.StatusBadRequest, "task_id is required")
		return
	}
	h.runCmd(w, r, proj.Path, 60*time.Second, "merge", "check", req.TaskID)
}

// MergePrepare handles POST /api/v1/projects/{id}/merge/prepare.
// Body: { "task_id": "TASK-xxx" }
func (h *ExtrasHandler) MergePrepare(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TaskID) == "" {
		writeError(w, http.StatusBadRequest, "task_id is required")
		return
	}
	h.runCmd(w, r, proj.Path, 60*time.Second, "merge", "prepare", req.TaskID)
}

// MergeCommit handles POST /api/v1/projects/{id}/merge/commit.
// Body: { "task_id": "TASK-xxx" }
func (h *ExtrasHandler) MergeCommit(w http.ResponseWriter, r *http.Request) {
	proj, ok := resolveProject(h.registry, w, r)
	if !ok {
		return
	}
	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TaskID) == "" {
		writeError(w, http.StatusBadRequest, "task_id is required")
		return
	}
	h.runCmd(w, r, proj.Path, 120*time.Second, "merge", "commit", req.TaskID)
}
