package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfigFilesRendersTemplates(t *testing.T) {
	root := t.TempDir()
	aomPath := filepath.Join(root, ".aom")
	if err := os.MkdirAll(aomPath, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	err := writeConfigFiles(aomPath, "my-app", root, "main", "my-app", "")
	if err != nil {
		t.Fatalf("writeConfigFiles failed: %v", err)
	}

	projectData, err := os.ReadFile(filepath.Join(aomPath, "project.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(project.yaml) failed: %v", err)
	}
	if !strings.Contains(string(projectData), "name: my-app") {
		t.Fatalf("project.yaml = %q, want rendered project name", string(projectData))
	}

	agentsData, err := os.ReadFile(filepath.Join(aomPath, "agents.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(agents.yaml) failed: %v", err)
	}
	if !strings.Contains(string(agentsData), "backend-main:") {
		t.Fatalf("agents.yaml = %q, want baseline agent template", string(agentsData))
	}

	gitignoreData, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile(.gitignore) failed: %v", err)
	}
	if !strings.Contains(string(gitignoreData), ".agent/") {
		t.Fatalf(".gitignore = %q, want .agent entry", string(gitignoreData))
	}
}

func TestWriteConfigFilesUsesCustomTemplateDir(t *testing.T) {
	root := t.TempDir()
	aomPath := filepath.Join(root, ".aom")
	templateDir := filepath.Join(root, "templates")
	if err := os.MkdirAll(aomPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(.aom) failed: %v", err)
	}
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templates) failed: %v", err)
	}

	files := map[string]string{
		"project.yaml.tmpl":   "name: {{ .Name }}\nrepo: {{ .RepoPath }}\ndefault_branch: {{ .DefaultBranch }}\n\nruntime:\n  terminal: tmux\n  session_prefix: custom\n\ncontext:\n  state_dir: tasks\n  checkpoint_required: true\n",
		"agents.yaml.tmpl":    "roles: {}\nagents:\n  custom-main:\n    runtime: codex\n    role: custom\n    enabled: true\n",
		"resources.yaml.tmpl": "skills: {}\nmcp_servers: {}\nrole_bindings: {}\n",
		"policy.yaml.tmpl":    "policy:\n  deny_commands: []\n  require_approval: []\n  session_defaults:\n    approval_scope: per-session\n    yolo_mode: disabled\n  owner_exceptions:\n    enabled: true\n    log_required: true\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) failed: %v", name, err)
		}
	}

	err := writeConfigFiles(aomPath, "my-app", root, "main", "my-app", templateDir)
	if err != nil {
		t.Fatalf("writeConfigFiles failed: %v", err)
	}

	agentsData, err := os.ReadFile(filepath.Join(aomPath, "agents.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(agents.yaml) failed: %v", err)
	}
	if !strings.Contains(string(agentsData), "custom-main:") {
		t.Fatalf("agents.yaml = %q, want custom template content", string(agentsData))
	}
}

func TestWriteConfigFilesAppendsAgentIgnoreWithoutOverwritingExistingGitignore(t *testing.T) {
	root := t.TempDir()
	aomPath := filepath.Join(root, ".aom")
	if err := os.MkdirAll(aomPath, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.gitignore) failed: %v", err)
	}

	if err := writeConfigFiles(aomPath, "my-app", root, "main", "my-app", ""); err != nil {
		t.Fatalf("writeConfigFiles failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile(.gitignore) failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Fatalf(".gitignore = %q, want existing content preserved", content)
	}
	if !strings.Contains(content, ".agent/") {
		t.Fatalf(".gitignore = %q, want .agent entry", content)
	}
	if strings.Count(content, ".agent/") != 1 {
		t.Fatalf(".gitignore = %q, want one .agent entry", content)
	}
}

func TestResolvePresetTemplateDirFindsTopLevelTemplate(t *testing.T) {
	path, err := resolvePresetTemplateDir("minimal")
	if err != nil {
		t.Fatalf("resolvePresetTemplateDir failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(path, "agents.yaml.tmpl")); err != nil {
		t.Fatalf("preset agents template missing: %v", err)
	}
}
