package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/config"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/session"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/step"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/task"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/worktree"
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

func TestEnsureHandoffTemplateCreatesStructuredTemplate(t *testing.T) {
	repoRoot := t.TempDir()
	service := NewService(repoRoot, "tasks")
	params := SyncParams{
		Task: task.Record{
			ID:             "TASK-011",
			Title:          "Seed handoff template",
			Mode:           "Direct",
			PreferredRole:  "backend",
			PreferredAgent: "backend-main",
		},
		Steps: []step.Record{
			{ID: "STEP-001", TaskID: "TASK-011", StepType: "implementation", Title: "Implement slice", Status: "InProgress", RoleName: "backend"},
		},
	}

	err := service.EnsureHandoffTemplate(params, session.Record{
		ID:        "SESS-001",
		AgentName: "backend-main",
		RoleName:  "backend",
		Runtime:   "codex",
	})
	if err != nil {
		t.Fatalf("EnsureHandoffTemplate failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, ".aom", "tasks", "TASK-011", "handoff.md"))
	if err != nil {
		t.Fatalf("ReadFile(handoff.md) failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "From Session: SESS-001") {
		t.Fatalf("handoff.md = %q, want session id", content)
	}
	if !strings.Contains(content, "From Runtime: codex") {
		t.Fatalf("handoff.md = %q, want runtime", content)
	}
	if !strings.Contains(content, "To Role: backend") {
		t.Fatalf("handoff.md = %q, want role", content)
	}
	if !strings.Contains(content, "Step: STEP-001 Implement slice") {
		t.Fatalf("handoff.md = %q, want active step", content)
	}
}

func TestEnsureHandoffTemplateDoesNotOverwriteExistingContent(t *testing.T) {
	repoRoot := t.TempDir()
	service := NewService(repoRoot, "tasks")
	params := SyncParams{
		Task: task.Record{
			ID:    "TASK-012",
			Title: "Preserve handoff content",
			Mode:  "Direct",
		},
	}

	dir := filepath.Join(repoRoot, ".aom", "tasks", "TASK-012")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	original := "# Handoff\n\ncustom content\n"
	path := filepath.Join(dir, "handoff.md")
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile(handoff.md) failed: %v", err)
	}

	if err := service.EnsureHandoffTemplate(params, session.Record{ID: "SESS-002"}); err != nil {
		t.Fatalf("EnsureHandoffTemplate failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(handoff.md) failed: %v", err)
	}
	if string(data) != original {
		t.Fatalf("handoff.md was overwritten: %q", string(data))
	}
}

func TestMaterializeIdentityFileWritesRuntimeSpecificFilename(t *testing.T) {
	root := t.TempDir()
	profilePath := filepath.Join(root, "profile.md")
	worktreePath := filepath.Join(root, "worktree")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll(worktree) failed: %v", err)
	}
	if err := os.WriteFile(profilePath, []byte("# Agent Identity\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(profile.md) failed: %v", err)
	}

	if err := MaterializeIdentityFile("backend-main", "codex", worktreePath, profilePath); err != nil {
		t.Fatalf("MaterializeIdentityFile failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(worktreePath, "AGENTS.md"))
	if err != nil {
		t.Fatalf("ReadFile(AGENTS.md) failed: %v", err)
	}
	if string(data) != "# Agent Identity\n" {
		t.Fatalf("AGENTS.md = %q, want copied profile", string(data))
	}
}

func TestMaterializeIdentityFileSkipsMissingProfileAndUnsupportedRuntime(t *testing.T) {
	root := t.TempDir()
	worktreePath := filepath.Join(root, "worktree")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll(worktree) failed: %v", err)
	}

	if err := MaterializeIdentityFile("backend-main", "codex", worktreePath, filepath.Join(root, "missing-profile.md")); err != nil {
		t.Fatalf("MaterializeIdentityFile missing profile returned error: %v", err)
	}
	if err := MaterializeIdentityFile("backend-main", "kiro", worktreePath, filepath.Join(root, "missing-profile.md")); err != nil {
		t.Fatalf("MaterializeIdentityFile unsupported runtime returned error: %v", err)
	}
	if err := MaterializeIdentityFile("backend-main", "codex", "", filepath.Join(root, "missing-profile.md")); err != nil {
		t.Fatalf("MaterializeIdentityFile empty worktree returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(worktreePath, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("AGENTS.md unexpectedly exists: %v", err)
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

func TestMaterializeSkillFilesCopiesSkillToWorktree(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	worktreePath := filepath.Join(root, "worktree")
	skillSrc := filepath.Join(repoPath, ".aom", "skills", "api-patterns.md")

	if err := os.MkdirAll(filepath.Dir(skillSrc), 0o755); err != nil {
		t.Fatalf("MkdirAll skill src dir: %v", err)
	}
	if err := os.WriteFile(skillSrc, []byte("# API Patterns\n"), 0o644); err != nil {
		t.Fatalf("WriteFile skill src: %v", err)
	}
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll worktree: %v", err)
	}

	skills := []config.ResolvedSkill{
		{Name: "api-patterns", SkillConfig: config.SkillConfig{Path: ".aom/skills/api-patterns.md", Runtimes: []string{"codex"}}},
	}
	if err := MaterializeSkillFiles("backend-main", skills, repoPath, worktreePath); err != nil {
		t.Fatalf("MaterializeSkillFiles: %v", err)
	}

	dst := filepath.Join(worktreePath, "api-patterns.md")
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile(api-patterns.md): %v", err)
	}
	if string(data) != "# API Patterns\n" {
		t.Fatalf("api-patterns.md = %q, want original content", string(data))
	}
}

func TestMaterializeSkillFilesSkipsMissingSource(t *testing.T) {
	root := t.TempDir()
	worktreePath := filepath.Join(root, "worktree")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll worktree: %v", err)
	}

	skills := []config.ResolvedSkill{
		{Name: "missing", SkillConfig: config.SkillConfig{Path: ".aom/skills/missing.md", Runtimes: []string{"codex"}}},
	}
	if err := MaterializeSkillFiles("backend-main", skills, root, worktreePath); err != nil {
		t.Fatalf("MaterializeSkillFiles with missing source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(worktreePath, "missing.md")); !os.IsNotExist(err) {
		t.Fatal("missing.md unexpectedly created")
	}
}

func TestMaterializeMCPConfigAppendsClaudeSection(t *testing.T) {
	root := t.TempDir()
	claudeMD := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte("# Identity\n"), 0o644); err != nil {
		t.Fatalf("WriteFile CLAUDE.md: %v", err)
	}

	servers := []config.ResolvedMCPServer{
		{Name: "repo-index", MCPServerConfig: config.MCPServerConfig{Type: "stdio", Command: "uvx", Args: []string{"repo-index-server"}}},
	}
	if err := MaterializeMCPConfig("backend-main", "claude", servers, root); err != nil {
		t.Fatalf("MaterializeMCPConfig(claude): %v", err)
	}

	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("ReadFile CLAUDE.md: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "## MCP Servers") {
		t.Fatalf("CLAUDE.md missing MCP Servers section: %q", out)
	}
	if !strings.Contains(out, "repo-index") {
		t.Fatalf("CLAUDE.md missing repo-index entry: %q", out)
	}
	if !strings.Contains(out, "# Identity") {
		t.Fatalf("CLAUDE.md original content lost: %q", out)
	}
}

func TestMaterializeMCPConfigWritesCodexJSON(t *testing.T) {
	root := t.TempDir()

	servers := []config.ResolvedMCPServer{
		{Name: "repo-index", MCPServerConfig: config.MCPServerConfig{Type: "stdio", Command: "uvx", Args: []string{"repo-index-server"}}},
	}
	if err := MaterializeMCPConfig("backend-main", "codex", servers, root); err != nil {
		t.Fatalf("MaterializeMCPConfig(codex): %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".codex", "mcp.json"))
	if err != nil {
		t.Fatalf("ReadFile .codex/mcp.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal mcp.json: %v", err)
	}
	mcpServers, ok := parsed["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcp.json missing mcpServers key: %v", parsed)
	}
	if _, ok := mcpServers["repo-index"]; !ok {
		t.Fatalf("mcp.json missing repo-index entry: %v", mcpServers)
	}
}

func TestMaterializeMCPConfigNoopForEmptyServers(t *testing.T) {
	root := t.TempDir()
	if err := MaterializeMCPConfig("backend-main", "claude", nil, root); err != nil {
		t.Fatalf("MaterializeMCPConfig(empty): %v", err)
	}
	if err := MaterializeMCPConfig("backend-main", "codex", []config.ResolvedMCPServer{}, root); err != nil {
		t.Fatalf("MaterializeMCPConfig(empty slice): %v", err)
	}
}

func TestMaterializeCodexConfigWritesConfigTOML(t *testing.T) {
	root := t.TempDir()
	if err := MaterializeCodexConfig("bot-codex", "codex", root); err != nil {
		t.Fatalf("MaterializeCodexConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("ReadFile .codex/config.toml: %v", err)
	}
	content := string(data)

	// Confirmed-valid keys (verified against codex binary string table).
	// Top-level keys must appear before any [section] header in the TOML file.
	for _, want := range []string{
		"model_auto_compact_token_limit",
		"tool_output_token_limit",
		"background_terminal_max_timeout",
		"[agents]",
		"max_threads = 1",
		"job_max_runtime_seconds",
	} {
		if !strings.Contains(content, want) {
			t.Errorf(".codex/config.toml missing %q; content:\n%s", want, content)
		}
	}
	// Keys that were previously written but are NOT valid in codex's config schema.
	for _, bad := range []string{
		"tool_timeout_sec", // incorrect name; correct is agents.job_max_runtime_seconds
		"max_bytes",        // [history] section does not accept this key
	} {
		if strings.Contains(content, bad) {
			t.Errorf(".codex/config.toml contains invalid key %q; content:\n%s", bad, content)
		}
	}
	// Top-level keys must NOT appear after a [section] header — verify structural ordering.
	agentsIdx := strings.Index(content, "[agents]")
	for _, topKey := range []string{"model_auto_compact_token_limit", "tool_output_token_limit", "background_terminal_max_timeout"} {
		keyIdx := strings.Index(content, topKey)
		if keyIdx > agentsIdx {
			t.Errorf(".codex/config.toml key %q appears after [agents] section — it will be treated as agents.%s", topKey, topKey)
		}
	}
}

func TestMaterializeCodexConfigNoopForNonCodex(t *testing.T) {
	for _, runtime := range []string{"claude", "gemini", "kiro", ""} {
		root := t.TempDir()
		if err := MaterializeCodexConfig("agent", runtime, root); err != nil {
			t.Fatalf("MaterializeCodexConfig(%q): unexpected error: %v", runtime, err)
		}
		if _, err := os.Stat(filepath.Join(root, ".codex", "config.toml")); !os.IsNotExist(err) {
			t.Errorf("runtime %q: config.toml should not exist, got stat err: %v", runtime, err)
		}
	}
}

func TestMaterializeCodexConfigNoopForEmptyWorktree(t *testing.T) {
	if err := MaterializeCodexConfig("agent", "codex", ""); err != nil {
		t.Fatalf("MaterializeCodexConfig(empty path): %v", err)
	}
}

func TestMaterializeCodexConfigOverwritesExistingFile(t *testing.T) {
	root := t.TempDir()
	codexDir := filepath.Join(root, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write stale content first.
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte("stale = true\n"), 0o644); err != nil {
		t.Fatalf("seed config.toml: %v", err)
	}

	if err := MaterializeCodexConfig("bot", "codex", root); err != nil {
		t.Fatalf("MaterializeCodexConfig: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(codexDir, "config.toml"))
	content := string(data)
	if strings.Contains(content, "stale = true") {
		t.Error("config.toml was not overwritten — stale content still present")
	}
	if !strings.Contains(content, "max_threads = 1") {
		t.Error("config.toml missing expected AOM content after overwrite")
	}
}

func TestMaterializePolicyConstraintsAppendsToClaudeMD(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# Agent\n"), 0o644); err != nil {
		t.Fatalf("setup CLAUDE.md: %v", err)
	}

	if err := MaterializePolicyConstraints("backend-main", "claude", []string{"rm -rf", "git push --force"}, root); err != nil {
		t.Fatalf("MaterializePolicyConstraints: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "## Policy Constraints") {
		t.Fatalf("CLAUDE.md missing policy section: %q", got)
	}
	if !strings.Contains(got, "`rm -rf`") {
		t.Fatalf("CLAUDE.md missing deny command: %q", got)
	}
	if !strings.Contains(got, "`git push --force`") {
		t.Fatalf("CLAUDE.md missing deny command: %q", got)
	}
}

func TestMaterializePolicyConstraintsNoopForEmptyList(t *testing.T) {
	root := t.TempDir()
	if err := MaterializePolicyConstraints("backend-main", "claude", nil, root); err != nil {
		t.Fatalf("MaterializePolicyConstraints(nil): %v", err)
	}
	if err := MaterializePolicyConstraints("backend-main", "claude", []string{}, root); err != nil {
		t.Fatalf("MaterializePolicyConstraints(empty): %v", err)
	}
	// no CLAUDE.md should be created
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatal("CLAUDE.md should not be created for empty deny list")
	}
}

func TestComputeContinuityReadinessHigh(t *testing.T) {
	sess := &session.Record{ID: "S1", Status: "Working"}
	wt := &worktree.Record{Status: worktree.StatusActive}
	params := SyncParams{
		Task:          task.Record{Status: "InProgress"},
		ActiveSession: sess,
		Worktree:      wt,
	}
	if got := computeContinuityReadiness(params, 0); got != "High" {
		t.Fatalf("want High, got %q", got)
	}
}

func TestComputeContinuityReadinessLowOnBlockedTask(t *testing.T) {
	params := SyncParams{Task: task.Record{Status: "Blocked"}}
	if got := computeContinuityReadiness(params, 0); got != "Low" {
		t.Fatalf("want Low, got %q", got)
	}
}

func TestComputeContinuityReadinessLowOnUnresolvedReviews(t *testing.T) {
	sess := &session.Record{ID: "S1"}
	params := SyncParams{
		Task:          task.Record{Status: "InProgress"},
		ActiveSession: sess,
	}
	if got := computeContinuityReadiness(params, 2); got != "Low" {
		t.Fatalf("want Low, got %q", got)
	}
}

func TestComputeContinuityReadinessMediumWithNoSession(t *testing.T) {
	params := SyncParams{Task: task.Record{Status: "InProgress"}}
	if got := computeContinuityReadiness(params, 0); got != "Medium" {
		t.Fatalf("want Medium, got %q", got)
	}
}
