package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceOpenSyncsConfigToDB(t *testing.T) {
	repoRoot := t.TempDir()

	service := NewService()
	if _, err := service.Init(InitParams{
		Name:     "my-app",
		RepoPath: repoRoot,
	}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	result, err := service.Open(repoRoot)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if result.Project.Name != "my-app" {
		t.Fatalf("project name = %q, want %q", result.Project.Name, "my-app")
	}
	if len(result.Agents) != 4 {
		t.Fatalf("agent count = %d, want 4", len(result.Agents))
	}
	if result.DBPath != filepath.Join(repoRoot, ".aom", "sessions.db") {
		t.Fatalf("db path = %q, want %q", result.DBPath, filepath.Join(repoRoot, ".aom", "sessions.db"))
	}
	if result.TerminalDriver != "tmux" {
		t.Fatalf("terminal driver = %q, want %q", result.TerminalDriver, "tmux")
	}
	if result.SessionPrefix != "my-app" {
		t.Fatalf("session prefix = %q, want %q", result.SessionPrefix, "my-app")
	}
	if result.StateDir != "tasks" {
		t.Fatalf("state dir = %q, want %q", result.StateDir, "tasks")
	}

	for _, agentName := range []string{"orchestrator-main", "backend-main", "frontend-main", "reviewer-main"} {
		path := filepath.Join(repoRoot, ".aom", "agents", agentName, "profile.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) failed: %v", path, err)
		}
		if !strings.Contains(string(data), "Agent: "+agentName) {
			t.Fatalf("profile %q = %q, want agent identity", path, string(data))
		}
	}
}

func TestServiceInitUsesCustomTemplateDir(t *testing.T) {
	repoRoot := t.TempDir()
	templateDir := filepath.Join(repoRoot, "project-init-templates")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	files := map[string]string{
		"project.yaml.tmpl":   "name: {{ .Name }}\nrepo: {{ .RepoPath }}\ndefault_branch: {{ .DefaultBranch }}\n\nruntime:\n  terminal: tmux\n  session_prefix: {{ .SessionPrefix }}\n\ncontext:\n  state_dir: tasks\n  checkpoint_required: true\n",
		"agents.yaml.tmpl":    "roles:\n  custom:\n    class: builder\n    worktree_mode: dedicated-writer\n    checkpoint_expectation: required\n    default_session_mode: interactive\n\nagents:\n  custom-main:\n    runtime: codex\n    role: custom\n    enabled: true\n",
		"resources.yaml.tmpl": "skills: {}\nmcp_servers: {}\nrole_bindings: {}\n",
		"policy.yaml.tmpl":    "policy:\n  deny_commands: []\n  require_approval: []\n  session_defaults:\n    approval_scope: per-session\n    yolo_mode: disabled\n  owner_exceptions:\n    enabled: true\n    log_required: true\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) failed: %v", name, err)
		}
	}

	service := NewService()
	if _, err := service.Init(InitParams{
		Name:        "my-app",
		RepoPath:    repoRoot,
		TemplateDir: templateDir,
	}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	result, err := service.Open(repoRoot)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if len(result.Agents) != 1 {
		t.Fatalf("agent count = %d, want 1", len(result.Agents))
	}
	if result.Agents[0].Name != "custom-main" {
		t.Fatalf("agent name = %q, want custom-main", result.Agents[0].Name)
	}
}

func TestServiceInitUsesPresetTemplate(t *testing.T) {
	repoRoot := t.TempDir()

	service := NewService()
	if _, err := service.Init(InitParams{
		Name:         "my-app",
		RepoPath:     repoRoot,
		TemplateName: "minimal",
	}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	result, err := service.Open(repoRoot)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if len(result.Agents) != 1 {
		t.Fatalf("agent count = %d, want 1", len(result.Agents))
	}
	if result.Agents[0].Name != "backend-main" {
		t.Fatalf("agent name = %q, want backend-main", result.Agents[0].Name)
	}
}

func TestServiceInitFiltersSelectedAgents(t *testing.T) {
	repoRoot := t.TempDir()

	service := NewService()
	if _, err := service.Init(InitParams{
		Name:     "my-app",
		RepoPath: repoRoot,
		AgentSelections: []InitAgentSelection{
			{Name: "backend-main"},
			{Name: "reviewer-main"},
		},
	}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	result, err := service.Open(repoRoot)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if len(result.Agents) != 2 {
		t.Fatalf("agent count = %d, want 2", len(result.Agents))
	}
	for _, agentRecord := range result.Agents {
		if agentRecord.Name == "frontend-main" {
			t.Fatalf("unexpected filtered-out agent still present: %q", agentRecord.Name)
		}
	}
}

func TestServicePreviewInitAgentsReturnsTemplateAgents(t *testing.T) {
	repoRoot := t.TempDir()

	service := NewService()
	options, err := service.PreviewInitAgents(InitParams{
		Name:     "my-app",
		RepoPath: repoRoot,
	})
	if err != nil {
		t.Fatalf("PreviewInitAgents failed: %v", err)
	}

	if len(options) != 4 {
		t.Fatalf("option count = %d, want 4", len(options))
	}
	// Sorted alphabetically by name.
	if options[0].Name != "backend-main" {
		t.Fatalf("first option = %q, want backend-main", options[0].Name)
	}
	if options[1].Name != "frontend-main" {
		t.Fatalf("second option = %q, want frontend-main", options[1].Name)
	}
	if options[2].Name != "orchestrator-main" {
		t.Fatalf("third option = %q, want orchestrator-main", options[2].Name)
	}
	if options[3].Name != "reviewer-main" {
		t.Fatalf("fourth option = %q, want reviewer-main", options[3].Name)
	}
}

func TestServiceInitSupportsInlineAgentSelection(t *testing.T) {
	repoRoot := t.TempDir()

	service := NewService()
	if _, err := service.Init(InitParams{
		Name:     "my-app",
		RepoPath: repoRoot,
		AgentSelections: []InitAgentSelection{
			{Name: "backend-main"},
			{Name: "frontend-main", Role: "builder", Runtime: "claude", Inline: true},
		},
	}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	result, err := service.Open(repoRoot)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if len(result.Agents) != 2 {
		t.Fatalf("agent count = %d, want 2", len(result.Agents))
	}

	found := false
	for _, agentRecord := range result.Agents {
		if agentRecord.Name != "frontend-main" {
			continue
		}
		found = true
		if agentRecord.Role != "builder" {
			t.Fatalf("frontend role = %q, want builder", agentRecord.Role)
		}
		if agentRecord.Runtime != "claude" {
			t.Fatalf("frontend runtime = %q, want claude", agentRecord.Runtime)
		}
	}
	if !found {
		t.Fatal("frontend-main was not created")
	}
}

func TestServiceInitDoesNotOverwriteExistingAgentProfile(t *testing.T) {
	repoRoot := t.TempDir()

	service := NewService()
	if _, err := service.Init(InitParams{
		Name:     "my-app",
		RepoPath: repoRoot,
	}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	profilePath := filepath.Join(repoRoot, ".aom", "agents", "backend-main", "profile.md")
	original := "# Agent Identity\n\ncustom backend profile\n"
	if err := os.WriteFile(profilePath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile(profile.md) failed: %v", err)
	}

	if _, err := service.Init(InitParams{
		Name:     "my-app",
		RepoPath: repoRoot,
	}); err != nil {
		t.Fatalf("second Init failed: %v", err)
	}

	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("ReadFile(profile.md) failed: %v", err)
	}
	if string(data) != original {
		t.Fatalf("profile.md was overwritten: %q", string(data))
	}
}
