package project

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/config"
	"gopkg.in/yaml.v3"
)

//go:embed templates/project-init/*.tmpl templates/project-init/profiles/*.md.tmpl
var projectInitTemplates embed.FS

type projectTemplateData struct {
	Name          string
	RepoPath      string
	DefaultBranch string
	SessionPrefix string
}

type initAgentsConfig struct {
	Roles  map[string]config.RoleConfig  `yaml:"roles"`
	Agents map[string]config.AgentConfig `yaml:"agents"`
}

func writeConfigFiles(aomPath, name, repoPath, defaultBranch, sessionPrefix, templateDir string, agentSelections []InitAgentSelection) error {
	data := projectTemplateData{
		Name:          name,
		RepoPath:      repoPath,
		DefaultBranch: defaultBranch,
		SessionPrefix: sessionPrefix,
	}

	files := map[string]string{
		"project.yaml":   "templates/project-init/project.yaml.tmpl",
		"agents.yaml":    "templates/project-init/agents.yaml.tmpl",
		"resources.yaml": "templates/project-init/resources.yaml.tmpl",
		"policy.yaml":    "templates/project-init/policy.yaml.tmpl",
	}

	for outputName, templatePath := range files {
		var (
			rendered []byte
			err      error
		)
		if outputName == "agents.yaml" {
			rendered, err = renderAgentsConfig(data, templateDir, agentSelections)
			if err != nil {
				return fmt.Errorf("render %s: %w", outputName, err)
			}
		} else {
			rendered, err = renderTemplate(templatePath, templateDir, data)
			if err != nil {
				return fmt.Errorf("render %s: %w", outputName, err)
			}
		}

		path := filepath.Join(aomPath, outputName)
		if err := os.WriteFile(path, rendered, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outputName, err)
		}
	}

	if err := ensureRootGitignore(repoPath); err != nil {
		return err
	}

	if err := ensureHooksDir(aomPath); err != nil {
		return err
	}

	return nil
}

func renderAgentsConfig(data projectTemplateData, templateDir string, agentSelections []InitAgentSelection) ([]byte, error) {
	rendered, err := renderTemplate("templates/project-init/agents.yaml.tmpl", templateDir, data)
	if err != nil {
		return nil, err
	}
	return filterAgentsConfig(rendered, agentSelections)
}

var defaultGitignoreEntries = []string{
	// AOM runtime directories — must be first; agents run git add -A and these
	// must be excluded before any commit to prevent binary SQLite DBs and large
	// channel logs from being staged and slowing down git operations.
	".aom/",
	".agent/",
	// Common build artifacts and secrets
	"node_modules/",
	"dist/",
	"build/",
	"*.env",
	".env.*",
	".DS_Store",
}

func ensureRootGitignore(repoPath string) error {
	path := filepath.Join(repoPath, ".gitignore")

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read .gitignore: %w", err)
	}

	content := string(data)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	changed := false
	for _, entry := range defaultGitignoreEntries {
		if strings.Contains(content, entry) {
			continue
		}
		content += entry + "\n"
		changed = true
	}

	if !changed {
		return nil
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}
	return nil
}

func ensureHooksDir(aomPath string) error {
	hooksDir := filepath.Join(aomPath, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	// on-task-done.sh — live by default so the operator sees hook output immediately.
	taskDonePath := filepath.Join(hooksDir, "on-task-done.sh")
	if _, err := os.Stat(taskDonePath); err != nil {
		const content = `#!/bin/bash
# on-task-done.sh — triggered when a task is closed or accepted
# Args: $1=task_id  $2=task_title  $3=final_status
# Env:  AOM_REPO, AOM_HOOK
#
# Example: notify a reviewer session when a backend task completes
# REVIEW_SESS=$(aom session list 2>/dev/null | grep reviewer | awk '{print $1}' | head -1)
# if [ -n "$REVIEW_SESS" ]; then
#   aom session send "$REVIEW_SESS" "Task '$2' ($1) is done. Begin review."
# fi
echo "[aom hook] on-task-done: $1 ($2) -> $3"
`
		if err := os.WriteFile(taskDonePath, []byte(content), 0o755); err != nil {
			return fmt.Errorf("write on-task-done hook: %w", err)
		}
	}

	// Additional hooks shipped as .sh.example — operator activates by removing .example suffix.
	examples := []struct {
		name    string
		content string
	}{
		{"on-task-created.sh.example", `#!/bin/bash
# on-task-created.sh — triggered when a new task is created
# Args: $1=task_id  $2=task_title  $3=initial_status
# Env:  AOM_REPO, AOM_HOOK
echo "[aom hook] on-task-created: $1 ($2)"
`},
		{"on-blocked.sh.example", `#!/bin/bash
# on-blocked.sh — triggered when a task or plan is blocked
# Args: $1=task_id  $2=task_title  $3=status
# Env:  AOM_REPO, AOM_HOOK
# Exit code 2 blocks the originating operation and returns the script output as an error.
echo "[aom hook] on-blocked: $1 ($2)"
`},
		{"on-needs-attention.sh.example", `#!/bin/bash
# on-needs-attention.sh — triggered when a task transitions to NeedsAttention
# (e.g. review findings detected, QA failure recorded)
# Args: $1=task_id  $2=task_title  $3=status
# Env:  AOM_REPO, AOM_HOOK
echo "[aom hook] on-needs-attention: $1 ($2)"
`},
		{"on-approval-required.sh.example", `#!/bin/bash
# on-approval-required.sh — triggered when a session requests operator approval
# Args: $1=task_id  $2=agent_name  $3=status
# Env:  AOM_REPO, AOM_HOOK
echo "[aom hook] on-approval-required: task=$1 agent=$2"
`},
		{"on-plan-proposed.sh.example", `#!/bin/bash
# on-plan-proposed.sh — triggered when a worker proposes a plan (task → PendingApproval)
# Args: $1=task_id  $2=task_title  $3=status
# Env:  AOM_REPO, AOM_HOOK
# Run 'aom task plan-approve $1' or 'aom task plan-reject $1 --reason <text>' to act on it.
echo "[aom hook] on-plan-proposed: $1 ($2)"
`},
		{"on-plan-approved.sh.example", `#!/bin/bash
# on-plan-approved.sh — triggered when a proposed plan is approved (task → Ready)
# Args: $1=task_id  $2=task_title  $3=status
# Env:  AOM_REPO, AOM_HOOK
echo "[aom hook] on-plan-approved: $1 ($2)"
`},
		{"on-plan-rejected.sh.example", `#!/bin/bash
# on-plan-rejected.sh — triggered when a proposed plan is rejected (task → Blocked)
# Args: $1=task_id  $2=task_title  $3=status
# Env:  AOM_REPO, AOM_HOOK
echo "[aom hook] on-plan-rejected: $1 ($2)"
`},
	}

	for _, ex := range examples {
		p := filepath.Join(hooksDir, ex.name)
		if _, err := os.Stat(p); err != nil {
			if err := os.WriteFile(p, []byte(ex.content), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", ex.name, err)
			}
		}
	}
	return nil
}

func filterAgentsConfig(rendered []byte, agentSelections []InitAgentSelection) ([]byte, error) {
	normalizedSelections, err := normalizeInitAgentSelections(agentSelections)
	if err != nil {
		return nil, err
	}
	if len(normalizedSelections) == 0 {
		return rendered, nil
	}

	var agentsFile initAgentsConfig
	if err := yaml.Unmarshal(rendered, &agentsFile); err != nil {
		return nil, fmt.Errorf("unmarshal agents config: %w", err)
	}

	filteredAgents := make(map[string]config.AgentConfig, len(normalizedSelections))
	referencedRoles := make(map[string]struct{}, len(normalizedSelections))
	for _, selection := range normalizedSelections {
		if selection.Inline {
			filteredAgents[selection.Name] = config.AgentConfig{
				Runtime: selection.Runtime,
				Role:    selection.Role,
				Enabled: true,
			}
			referencedRoles[selection.Role] = struct{}{}
			continue
		}

		agentCfg, ok := agentsFile.Agents[selection.Name]
		if !ok {
			return nil, fmt.Errorf("agent %q was not found in the selected template", selection.Name)
		}
		filteredAgents[selection.Name] = agentCfg
		referencedRoles[agentCfg.Role] = struct{}{}
	}

	filteredRoles := make(map[string]config.RoleConfig, len(referencedRoles))
	for roleName := range referencedRoles {
		roleCfg, ok := agentsFile.Roles[roleName]
		if ok {
			filteredRoles[roleName] = roleCfg
			continue
		}
		if roleName == "" {
			return nil, fmt.Errorf("role %q required by selected agents was not found in the selected template", roleName)
		}
		filteredRoles[roleName] = defaultInlineRoleConfig()
	}

	agentsFile.Agents = filteredAgents
	agentsFile.Roles = filteredRoles
	filtered, err := yaml.Marshal(&agentsFile)
	if err != nil {
		return nil, fmt.Errorf("marshal filtered agents config: %w", err)
	}
	return filtered, nil
}

func ParseInitAgentSelections(values []string) ([]InitAgentSelection, error) {
	if len(values) == 0 {
		return nil, nil
	}

	selections := make([]InitAgentSelection, 0, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}

		selection, err := parseInitAgentSelection(item)
		if err != nil {
			return nil, err
		}
		selections = append(selections, selection)
	}
	return normalizeInitAgentSelections(selections)
}

func parseInitAgentSelection(value string) (InitAgentSelection, error) {
	parts := strings.Split(value, ":")
	switch len(parts) {
	case 1:
		name := strings.TrimSpace(parts[0])
		if err := validateInitAgentIdentifier(name, "agent"); err != nil {
			return InitAgentSelection{}, err
		}
		return InitAgentSelection{Name: name}, nil
	case 3:
		name := strings.TrimSpace(parts[0])
		role := strings.TrimSpace(parts[1])
		runtimeName := strings.ToLower(strings.TrimSpace(parts[2]))
		if err := validateInitAgentIdentifier(name, "agent"); err != nil {
			return InitAgentSelection{}, err
		}
		if strings.TrimSpace(role) == "" {
			return InitAgentSelection{}, fmt.Errorf("agent %q role is required", name)
		}
		if _, ok := knownInitAgentRuntimes[runtimeName]; !ok {
			return InitAgentSelection{}, fmt.Errorf("agent %q runtime %q is not supported", name, parts[2])
		}
		return InitAgentSelection{
			Name:    name,
			Role:    role,
			Runtime: runtimeName,
			Inline:  true,
		}, nil
	default:
		return InitAgentSelection{}, fmt.Errorf("agent selection %q must use name or name:role:runtime", value)
	}
}

var knownInitAgentRuntimes = map[string]struct{}{}

// RegisterKnownInitRuntime adds a runtime name to the set of known runtimes
// used when parsing and validating --agent flags during `aom init`. Call this
// at startup (e.g., from internal/app) for each provider in the registry.
func RegisterKnownInitRuntime(name string) {
	knownInitAgentRuntimes[name] = struct{}{}
}

func normalizeInitAgentSelections(selections []InitAgentSelection) ([]InitAgentSelection, error) {
	if len(selections) == 0 {
		return nil, nil
	}

	seen := make(map[string]InitAgentSelection, len(selections))
	normalized := make([]InitAgentSelection, 0, len(selections))
	for _, selection := range selections {
		current := InitAgentSelection{
			Name:   strings.TrimSpace(selection.Name),
			Role:   strings.TrimSpace(selection.Role),
			Inline: selection.Inline,
		}
		if err := validateInitAgentIdentifier(current.Name, "agent"); err != nil {
			return nil, err
		}
		if current.Inline {
			if current.Role == "" {
				return nil, fmt.Errorf("agent %q role is required", current.Name)
			}
			current.Runtime = strings.ToLower(strings.TrimSpace(selection.Runtime))
			if _, ok := knownInitAgentRuntimes[current.Runtime]; !ok {
				return nil, fmt.Errorf("agent %q runtime %q is not supported", current.Name, selection.Runtime)
			}
		}
		if existing, ok := seen[current.Name]; ok {
			if existing == current {
				continue
			}
			return nil, fmt.Errorf("agent %q was selected more than once", current.Name)
		}
		seen[current.Name] = current
		normalized = append(normalized, current)
	}
	return normalized, nil
}

func validateInitAgentIdentifier(value, label string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s name is required", label)
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return fmt.Errorf("%s name %q must be alphanumeric with hyphens only", label, value)
	}
	return nil
}

func defaultInlineRoleConfig() config.RoleConfig {
	return config.RoleConfig{
		Class:                 "builder",
		WorktreeMode:          "dedicated-writer",
		CheckpointExpectation: "required",
		DefaultSessionMode:    "interactive",
	}
}

func renderTemplate(templatePath, templateDir string, data projectTemplateData) ([]byte, error) {
	source, err := readTemplateSource(templatePath, templateDir)
	if err != nil {
		return nil, err
	}

	source = bytes.ReplaceAll(source, []byte("\r\n"), []byte("\n"))

	tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(source))
	if err != nil {
		return nil, err
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return nil, err
	}

	return rendered.Bytes(), nil
}

func readTemplateSource(templatePath, templateDir string) ([]byte, error) {
	if templateDir == "" {
		return projectInitTemplates.ReadFile(templatePath)
	}

	customPath := filepath.Join(templateDir, filepath.Base(templatePath))
	data, err := os.ReadFile(customPath)
	if err != nil {
		return nil, fmt.Errorf("read custom template %q: %w", customPath, err)
	}

	return data, nil
}

func resolvePresetTemplateDir(name string) (string, error) {
	name = filepath.Clean(name)
	if name == "." || name == "" {
		return "", fmt.Errorf("template preset is required")
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve preset template path: runtime caller is unavailable")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	templateDir := filepath.Join(repoRoot, "templates", "project-init", name)
	info, err := os.Stat(templateDir)
	if err != nil {
		return "", fmt.Errorf("stat preset template dir %q: %w", templateDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("preset template dir %q is not a directory", templateDir)
	}

	return templateDir, nil
}
