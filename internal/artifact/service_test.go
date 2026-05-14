package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/step"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/task"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/worktree"
)

func TestServiceSeedTaskArtifactsCreatesCoreFiles(t *testing.T) {
	repoRoot := t.TempDir()
	service := NewService(repoRoot, "tasks")
	service.now = func() time.Time { return time.Date(2026, 5, 11, 21, 0, 0, 0, time.FixedZone("ICT", 7*60*60)) }
	service.eventIDGen = func() string { return "EVT-001" }

	err := service.SeedTaskArtifacts(SyncParams{
		Task: task.Record{
			ID:             "TASK-001",
			Title:          "Fix login validation",
			Mode:           "Direct",
			Status:         "Planned",
			PreferredRole:  "backend",
			PreferredAgent: "backend-main",
		},
		Steps: []step.Record{
			{
				ID:        "STEP-001",
				TaskID:    "TASK-001",
				StepType:  "implementation",
				Title:     "Fix login validation",
				Status:    "Proposed",
				RoleName:  "backend",
				AgentName: "backend-main",
			},
		},
		Worktree: &worktree.Record{
			TaskID:       "TASK-001",
			ProjectID:    "proj-1",
			Status:       "Planned",
			BaseBranch:   "main",
			BranchName:   "aom/task-001-fix-login-validation",
			WorktreePath: filepath.Join(repoRoot, ".aom", "worktrees", "task-001-fix-login-validation"),
		},
		CreatedBy:             "operator",
		UpdatedBy:             "operator",
		RecommendedNextAction: "confirm the proposed step and move the task to Ready",
	})
	if err != nil {
		t.Fatalf("SeedTaskArtifacts failed: %v", err)
	}

	taskDir := filepath.Join(repoRoot, ".aom", "tasks", "TASK-001")
	for _, name := range []string{"task.md", "state.md", "index.md", "log.md"} {
		if _, err := os.Stat(filepath.Join(taskDir, name)); err != nil {
			t.Fatalf("artifact %s missing: %v", name, err)
		}
	}

	taskData, err := os.ReadFile(filepath.Join(taskDir, "task.md"))
	if err != nil {
		t.Fatalf("ReadFile(task.md) failed: %v", err)
	}
	if !strings.Contains(string(taskData), "Worktree: "+filepath.Join(repoRoot, ".aom", "worktrees", "task-001-fix-login-validation")) {
		t.Fatalf("task.md = %q, want mapped worktree path", string(taskData))
	}

	indexData, err := os.ReadFile(filepath.Join(taskDir, "index.md"))
	if err != nil {
		t.Fatalf("ReadFile(index.md) failed: %v", err)
	}
	if !strings.Contains(string(indexData), "Worktree Status: Planned") {
		t.Fatalf("index.md = %q, want planned worktree status", string(indexData))
	}

	logData, err := os.ReadFile(filepath.Join(taskDir, "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "task.created") {
		t.Fatalf("log.md = %q, want task.created event", string(logData))
	}
	if !strings.Contains(string(logData), "step.proposed") {
		t.Fatalf("log.md = %q, want step.proposed event", string(logData))
	}
}

func TestServiceSeedTaskArtifactsWritesIntoReadyWorktreeAgentDir(t *testing.T) {
	repoRoot := t.TempDir()
	worktreePath := filepath.Join(repoRoot, ".aom", "worktrees", "task-003-fix-login-validation")
	service := NewService(repoRoot, "tasks")

	err := service.SeedTaskArtifacts(SyncParams{
		Task: task.Record{
			ID:             "TASK-003",
			Title:          "Fix login validation",
			Mode:           "Direct",
			Status:         "Planned",
			PreferredRole:  "backend",
			PreferredAgent: "backend-main",
		},
		Steps: []step.Record{
			{ID: "STEP-001", TaskID: "TASK-003", StepType: "implementation", Title: "Fix login validation", Status: "Proposed"},
		},
		Worktree: &worktree.Record{
			TaskID:       "TASK-003",
			ProjectID:    "proj-1",
			Status:       "Ready",
			BaseBranch:   "main",
			BranchName:   "aom/task-003-fix-login-validation",
			WorktreePath: worktreePath,
		},
		CreatedBy:             "operator",
		UpdatedBy:             "operator",
		RecommendedNextAction: "confirm the proposed step and move the task to Ready",
	})
	if err != nil {
		t.Fatalf("SeedTaskArtifacts failed: %v", err)
	}

	agentDir := filepath.Join(worktreePath, ".agent")
	for _, name := range []string{"task.md", "state.md", "index.md", "log.md"} {
		if _, err := os.Stat(filepath.Join(agentDir, name)); err != nil {
			t.Fatalf("artifact %s missing: %v", name, err)
		}
	}

	if _, err := os.Stat(filepath.Join(repoRoot, ".aom", "tasks", "TASK-003", "task.md")); !os.IsNotExist(err) {
		t.Fatalf("fallback artifact unexpectedly exists: %v", err)
	}

	taskData, err := os.ReadFile(filepath.Join(agentDir, "task.md"))
	if err != nil {
		t.Fatalf("ReadFile(task.md) failed: %v", err)
	}
	if !strings.Contains(string(taskData), "Artifact Root: "+filepath.Join(worktreePath, ".agent")) {
		t.Fatalf("task.md = %q, want .agent artifact root", string(taskData))
	}
}

func TestServiceSeedTaskArtifactsCreatesModeSpecificFiles(t *testing.T) {
	repoRoot := t.TempDir()
	service := NewService(repoRoot, "tasks")

	err := service.SeedTaskArtifacts(SyncParams{
		Task: task.Record{
			ID:             "TASK-002",
			Title:          "Capture checkout requirements",
			Mode:           "Requirements-first",
			Status:         "Planned",
			PreferredRole:  "backend",
			PreferredAgent: "backend-main",
		},
		Steps: []step.Record{
			{ID: "STEP-001", TaskID: "TASK-002", StepType: "research", Title: "Capture requirements", Status: "Proposed"},
			{ID: "STEP-002", TaskID: "TASK-002", StepType: "coordination", Title: "Turn accepted requirements into implementation steps", Status: "Proposed", Dependencies: []string{"STEP-001"}},
		},
		CreatedBy:             "operator",
		UpdatedBy:             "operator",
		RecommendedNextAction: "confirm the requirements-first mode, then create the task and capture the first requirement step",
	})
	if err != nil {
		t.Fatalf("SeedTaskArtifacts failed: %v", err)
	}

	taskDir := filepath.Join(repoRoot, ".aom", "tasks", "TASK-002")
	for _, name := range []string{"requirements.md", "tasks.md"} {
		if _, err := os.Stat(filepath.Join(taskDir, name)); err != nil {
			t.Fatalf("mode artifact %s missing: %v", name, err)
		}
	}
}

func TestCountUnresolvedReviewItemsCountsOnlyOpenStatuses(t *testing.T) {
	repoRoot := t.TempDir()
	path := filepath.Join(repoRoot, "review-notes.md")
	content := `# Review Notes

## Summary
- Status: Needs fixes

## Items

### RVW-001
- Severity: high
- Path: internal/auth/handler.go
- Issue: inconsistent error payload
- Expected Fix: use shared envelope helper
- Status: open
- Owner: backend

### RVW-002
- Severity: medium
- Path: internal/auth/handler_test.go
- Issue: missing malformed email test
- Expected Fix: add focused negative test
- Status: resolved
- Owner: backend

### RVW-003
- Severity: low
- Path: internal/auth/validation.go
- Issue: follow-up cleanup
- Expected Fix: simplify duplicated branch
- Status: in-progress
- Owner: backend
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(review-notes.md) failed: %v", err)
	}

	if got := CountUnresolvedReviewItems(path); got != 2 {
		t.Fatalf("CountUnresolvedReviewItems() = %d, want 2", got)
	}
}

func TestEnsureReviewNotesTemplateDoesNotOverwriteExistingFindings(t *testing.T) {
	repoRoot := t.TempDir()
	service := NewService(repoRoot, "tasks")
	params := SyncParams{
		Task: task.Record{
			ID:    "TASK-010",
			Title: "Review findings preservation",
			Mode:  "Direct",
		},
	}

	dir := filepath.Join(repoRoot, ".aom", "tasks", "TASK-010")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	original := `# Review Notes

## Summary
- Status: Needs fixes

## Items

### RVW-001
- Status: open
`
	path := filepath.Join(dir, "review-notes.md")
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile(review-notes.md) failed: %v", err)
	}

	if err := service.EnsureReviewNotesTemplate(params, "reviewer-main", "SESS-001"); err != nil {
		t.Fatalf("EnsureReviewNotesTemplate failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(review-notes.md) failed: %v", err)
	}
	if string(data) != original {
		t.Fatalf("review-notes.md was overwritten: %q", string(data))
	}
}

func TestSuggestedReviewOwnerReturnsSharedUnresolvedOwnerOnly(t *testing.T) {
	repoRoot := t.TempDir()
	path := filepath.Join(repoRoot, "review-notes.md")
	content := `# Review Notes

## Items

### RVW-001
- Status: open
- Owner: backend

### RVW-002
- Status: resolved
- Owner: reviewer

### RVW-003
- Status: in-progress
- Owner: backend
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(review-notes.md) failed: %v", err)
	}
	if got := SuggestedReviewOwner(path); got != "backend" {
		t.Fatalf("SuggestedReviewOwner() = %q, want backend", got)
	}

	mixedPath := filepath.Join(repoRoot, "review-notes-mixed.md")
	mixed := `# Review Notes

## Items

### RVW-001
- Status: open
- Owner: backend

### RVW-002
- Status: open
- Owner: qa
`
	if err := os.WriteFile(mixedPath, []byte(mixed), 0o644); err != nil {
		t.Fatalf("WriteFile(review-notes-mixed.md) failed: %v", err)
	}
	if got := SuggestedReviewOwner(mixedPath); got != "" {
		t.Fatalf("SuggestedReviewOwner() = %q, want empty for mixed owners", got)
	}
}
