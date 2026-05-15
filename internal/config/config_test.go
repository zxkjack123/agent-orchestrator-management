package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProjectConfig(t *testing.T) {
	root := t.TempDir()

	if err := os.Mkdir(filepath.Join(root, ".aom"), 0o755); err != nil {
		t.Fatalf("Mkdir(.aom) failed: %v", err)
	}

	writeConfigFile(t, filepath.Join(root, ".aom", "project.yaml"), `
name: my-app
repo: .
default_branch: main

runtime:
  terminal: tmux
  session_prefix: myapp

context:
  state_dir: tasks
  checkpoint_required: true
`)

	writeConfigFile(t, filepath.Join(root, ".aom", "agents.yaml"), `
roles:
  orchestrator:
    class: orchestrator
    worktree_mode: read-only
    checkpoint_expectation: required
    default_session_mode: interactive

  backend:
    class: builder
    worktree_mode: dedicated-writer
    checkpoint_expectation: required
    default_session_mode: interactive

agents:
  orchestrator-main:
    runtime: claude
    role: orchestrator
    enabled: true

  backend-main:
    runtime: codex
    role: backend
    enabled: true
`)

	writeConfigFile(t, filepath.Join(root, ".aom", "resources.yaml"), `
skills:
  api-patterns:
    path: .aom/skills/api-patterns.md
    shared: true
    runtimes: [codex, claude]

mcp_servers:
  repo-index:
    type: stdio
    command: uvx
    args: ["repo-index-server"]
    shared: true
    runtimes: [codex, claude]

role_bindings:
  backend:
    skills: [api-patterns]
    mcp_servers: [repo-index]
`)

	writeConfigFile(t, filepath.Join(root, ".aom", "policy.yaml"), `
policy:
  deny_commands:
    - "rm -rf"
  require_approval:
    - "network access"
  session_defaults:
    approval_scope: per-session
    yolo_mode: disabled
  owner_exceptions:
    enabled: true
    log_required: true
`)

	cfg, err := LoadProjectConfig(root)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	if cfg.Project.Name != "my-app" {
		t.Fatalf("Project.Name = %q, want %q", cfg.Project.Name, "my-app")
	}
	if cfg.Project.Runtime.Terminal != "tmux" {
		t.Fatalf("Project.Runtime.Terminal = %q, want %q", cfg.Project.Runtime.Terminal, "tmux")
	}
	if got := cfg.Agents.Agents["backend-main"].Runtime; got != "codex" {
		t.Fatalf("backend-main runtime = %q, want %q", got, "codex")
	}
	if got := cfg.Resources.RoleBindings["backend"].Skills[0]; got != "api-patterns" {
		t.Fatalf("backend skill binding = %q, want %q", got, "api-patterns")
	}
}

func TestValidateRejectsUnsupportedTerminal(t *testing.T) {
	cfg := validConfig(t.TempDir())
	cfg.Project.Runtime.Terminal = "screen"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate returned nil, want error")
	}
	if !strings.Contains(err.Error(), "runtime.terminal") {
		t.Fatalf("Validate error = %q, want runtime.terminal context", err)
	}
}

func TestValidateRejectsUnknownAgentRole(t *testing.T) {
	cfg := validConfig(t.TempDir())
	cfg.Agents.Agents["backend-main"] = AgentConfig{
		Runtime: "codex",
		Role:    "missing-role",
		Enabled: true,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate returned nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown role") {
		t.Fatalf("Validate error = %q, want unknown role", err)
	}
}

func TestValidateAllowsCustomRoleClass(t *testing.T) {
	cfg := validConfig(t.TempDir())
	cfg.Agents.Roles["backend"] = RoleConfig{
		Class:                 "playwright-qa-specialist",
		WorktreeMode:          "dedicated-writer",
		CheckpointExpectation: "required",
		DefaultSessionMode:    "interactive",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed for custom role class: %v", err)
	}
}

func TestValidateRejectsSkillPathOutsideProject(t *testing.T) {
	cfg := validConfig(t.TempDir())
	cfg.Resources.Skills["bad-skill"] = SkillConfig{
		Path:     "../outside.md",
		Shared:   true,
		Runtimes: []string{"codex"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate returned nil, want error")
	}
	if !strings.Contains(err.Error(), "inside the project") {
		t.Fatalf("Validate error = %q, want inside the project", err)
	}
}

func TestValidateRejectsInvalidPolicyDefaults(t *testing.T) {
	cfg := validConfig(t.TempDir())
	cfg.Policy.Policy.SessionDefaults.ApprovalScope = "global"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate returned nil, want error")
	}
	if !strings.Contains(err.Error(), "approval_scope") {
		t.Fatalf("Validate error = %q, want approval_scope", err)
	}
}

func validConfig(root string) *ProjectConfig {
	return &ProjectConfig{
		RootPath: root,
		AOMPath:  filepath.Join(root, ".aom"),
		Project: ProjectFile{
			Name:          "my-app",
			Repo:          root,
			DefaultBranch: "main",
			Runtime: RuntimeConfig{
				Terminal:      "tmux",
				SessionPrefix: "myapp",
			},
			Context: ContextConfig{
				StateDir:           "tasks",
				CheckpointRequired: true,
			},
		},
		Agents: AgentsFile{
			Roles: map[string]RoleConfig{
				"backend": {
					Class:                 "builder",
					WorktreeMode:          "dedicated-writer",
					CheckpointExpectation: "required",
					DefaultSessionMode:    "interactive",
				},
			},
			Agents: map[string]AgentConfig{
				"backend-main": {
					Runtime: "codex",
					Role:    "backend",
					Enabled: true,
				},
			},
		},
		Resources: ResourcesFile{
			Skills: map[string]SkillConfig{
				"api-patterns": {
					Path:     ".aom/skills/api-patterns.md",
					Shared:   true,
					Runtimes: []string{"codex"},
				},
			},
			MCPServers: map[string]MCPServerConfig{
				"repo-index": {
					Type:     "stdio",
					Command:  "uvx",
					Args:     []string{"repo-index-server"},
					Shared:   true,
					Runtimes: []string{"codex"},
				},
			},
			RoleBindings: map[string]RoleBindingConfig{
				"backend": {
					Skills:     []string{"api-patterns"},
					MCPServers: []string{"repo-index"},
				},
			},
		},
		Policy: PolicyFile{
			Policy: PolicyConfig{
				DenyCommands:    []string{"rm -rf"},
				RequireApproval: []string{"network access"},
				SessionDefaults: SessionDefaultsConfig{
					ApprovalScope: "per-session",
					YoloMode:      "disabled",
				},
				OwnerExceptions: OwnerExceptionsConfig{
					Enabled:     true,
					LogRequired: true,
				},
			},
		},
	}
}

func TestResourcesForRoleReturnsSkillsAndMCPForKnownRole(t *testing.T) {
	cfg := validConfig(t.TempDir())
	res := cfg.Resources.ResourcesForRole("backend", "codex")

	if len(res.Skills) != 1 || res.Skills[0].Name != "api-patterns" {
		t.Fatalf("Skills = %v, want [api-patterns]", res.Skills)
	}
	if len(res.MCPServers) != 1 || res.MCPServers[0].Name != "repo-index" {
		t.Fatalf("MCPServers = %v, want [repo-index]", res.MCPServers)
	}
}

func TestResourcesForRoleReturnsEmptyForUnknownRole(t *testing.T) {
	cfg := validConfig(t.TempDir())
	res := cfg.Resources.ResourcesForRole("nonexistent", "codex")

	if len(res.Skills) != 0 || len(res.MCPServers) != 0 {
		t.Fatalf("expected empty RoleResources for unknown role, got %+v", res)
	}
}

func TestResourcesForRoleFiltersIncompatibleRuntimes(t *testing.T) {
	cfg := validConfig(t.TempDir())
	// codex skills/MCP are not compatible with claude runtime
	res := cfg.Resources.ResourcesForRole("backend", "claude")

	if len(res.Skills) != 0 {
		t.Fatalf("Skills = %v, want empty (codex skills incompatible with claude)", res.Skills)
	}
	if len(res.MCPServers) != 0 {
		t.Fatalf("MCPServers = %v, want empty (codex MCP incompatible with claude)", res.MCPServers)
	}
}

func writeConfigFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", path, err)
	}
}
