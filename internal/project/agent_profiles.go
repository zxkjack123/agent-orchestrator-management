package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/config"
	"gopkg.in/yaml.v3"
)

// AgentProfilePath returns the path to an agent's profile.md file.
func AgentProfilePath(aomPath, agentName string) string {
	return filepath.Join(aomPath, "agents", agentName, "profile.md")
}

// ReadAgentProfile returns the content of an agent's profile.md, or empty string if not found.
func ReadAgentProfile(aomPath, agentName string) (string, error) {
	data, err := os.ReadFile(AgentProfilePath(aomPath, agentName))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read agent profile %q: %w", agentName, err)
	}
	return string(data), nil
}

// WriteAgentProfile overwrites an agent's profile.md with new content.
func WriteAgentProfile(aomPath, agentName, content string) error {
	path := AgentProfilePath(aomPath, agentName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create agent profile dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write agent profile %q: %w", agentName, err)
	}
	return nil
}

// AddAgentParams describes the parameters for adding a new agent.
type AddAgentParams struct {
	Name    string
	Role    string
	Runtime string
}

// AddAgentToConfig adds a new agent entry to agents.yaml and seeds its profile.
// If the role does not exist, it is auto-created with builder-class defaults.
func AddAgentToConfig(aomPath string, params AddAgentParams) error {
	agentsPath := filepath.Join(aomPath, "agents.yaml")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		return fmt.Errorf("read agents.yaml: %w", err)
	}

	var cfg config.AgentsFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse agents.yaml: %w", err)
	}
	if cfg.Agents == nil {
		cfg.Agents = make(map[string]config.AgentConfig)
	}
	if cfg.Roles == nil {
		cfg.Roles = make(map[string]config.RoleConfig)
	}

	if _, exists := cfg.Agents[params.Name]; exists {
		return fmt.Errorf("agent %q already exists in agents.yaml", params.Name)
	}

	if _, exists := cfg.Roles[params.Role]; !exists {
		cfg.Roles[params.Role] = defaultInlineRoleConfig()
	}

	cfg.Agents[params.Name] = config.AgentConfig{
		Runtime: params.Runtime,
		Role:    params.Role,
		Enabled: true,
	}

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal agents.yaml: %w", err)
	}
	if err := os.WriteFile(agentsPath, out, 0o644); err != nil {
		return fmt.Errorf("write agents.yaml: %w", err)
	}

	roleCfg := cfg.Roles[params.Role]
	content := renderAgentProfileMarkdown(params.Name, params.Role, params.Runtime, roleCfg.Class)
	return WriteAgentProfile(aomPath, params.Name, content)
}

// UpdateProfileSection replaces a named markdown section (## Heading) with new content.
// Returns the updated profile string. If the section is not found, it is appended.
func UpdateProfileSection(profile, section, newContent string) string {
	heading := "## " + section
	lines := strings.Split(profile, "\n")
	start := -1
	end := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i
			continue
		}
		if start >= 0 && i > start && strings.HasPrefix(strings.TrimSpace(line), "## ") {
			end = i
			break
		}
	}

	newLines := strings.Split(strings.TrimRight(newContent, "\n"), "\n")
	replacement := append([]string{heading}, newLines...)
	replacement = append(replacement, "")

	if start < 0 {
		// Section not found — append it
		return strings.Join(append(lines, replacement...), "\n")
	}

	updated := make([]string, 0, len(lines))
	updated = append(updated, lines[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, lines[end:]...)
	return strings.Join(updated, "\n")
}

func seedAgentProfiles(aomPath string, cfg *config.ProjectConfig) error {
	if cfg == nil {
		return fmt.Errorf("project config is required")
	}

	for agentName, agentCfg := range cfg.Agents.Agents {
		roleCfg, ok := cfg.Agents.Roles[agentCfg.Role]
		if !ok {
			return fmt.Errorf("agent %q references unknown role %q", agentName, agentCfg.Role)
		}

		profilePath := filepath.Join(aomPath, "agents", agentName, "profile.md")
		if _, err := os.Stat(profilePath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat agent profile %q: %w", profilePath, err)
		}

		if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
			return fmt.Errorf("create agent profile dir %q: %w", filepath.Dir(profilePath), err)
		}

		content := renderAgentProfileMarkdown(agentName, agentCfg.Role, agentCfg.Runtime, roleCfg.Class)
		if err := os.WriteFile(profilePath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write agent profile %q: %w", profilePath, err)
		}
	}

	return nil
}

func renderAgentProfileMarkdown(agentName, roleName, runtimeName, roleClass string) string {
	return fmt.Sprintf(`# Agent Identity

## Identity
- Agent: %s
- Role: %s
- Runtime: %s

## Responsibilities
- %s

## AOM Workflow

### Starting a session
1. Read .agent/task.md — understand goal, scope, and pipeline position
2. Read .agent/state.md — see current progress and remaining steps
3. Read .agent/log.md — review history and last checkpoint

### During work
- Update .agent/state.md as step progress changes
- Create checkpoints every ~30 minutes or after significant milestones

### Completing work
1. git add -A && git commit -m "..."  (always commit before signaling)
2. Choose the right signal:
   - Task fully done → append event "task.completed" to .agent/log.md
   - Handing off to another agent → append event "handoff.prepared" to .agent/log.md
3. Update .agent/state.md to reflect final status

### Valid log events
| Event             | Use when                                          |
|-------------------|---------------------------------------------------|
| task.completed    | Work is done; ready for operator to close         |
| handoff.prepared  | Passing work to another agent to continue         |
| checkpoint.created| Saving progress mid-task                          |
| step.completed    | A sub-step finished                               |

### Waiting for another session to finish
  aom session wait <session-id> --event task.completed
  aom session wait <session-id> --event handoff.prepared
  (default timeout: 30m — override with --timeout 2h)

### Seeing what tasks are ready to work on
  aom next                   # unblocked tasks sorted by priority
  aom next --format json     # machine-readable output for scripting

## Team Communication

Call AOM commands from your worktree shell to coordinate with teammates:
- Broadcast to team channel:       aom channel append "your message"
- Direct message a teammate:       aom message send <agent-name> "your message"
- Check your inbox:                aom message read <your-agent-name>
- Read a file from another worktree: aom worktree read-file <task-id> <relative-path>

NOTE: If your runtime sandbox restricts writes outside your worktree, "channel append"
and "message send" will stage to .agent/outbox.md instead. The operator runs
"aom outbox flush" to publish them. Seeing "Message staged to outbox" is expected.

## Constraints
- Stay within the current task scope
- Do not modify .agent/index.md or .agent/log.md directly — those are AOM-owned

## Model Configuration

This agent uses the provider default model unless model: is set in .aom/agents.yaml.

Configure per-agent model in .aom/agents.yaml:
  agents:
    <agent-name>:
      model: claude-sonnet-4-6   # claude aliases: sonnet, opus, haiku
      # codex: gpt-5.5, gpt-5.4, gpt-5.4-mini, gpt-5.3-codex
      # gemini: gemini-2.5-pro, gemini-2.5-flash

After changing model: re-run "aom session spawn" to apply.
`,
		agentName,
		roleName,
		runtimeName,
		defaultResponsibility(roleClass),
	)
}

func defaultResponsibility(roleClass string) string {
	switch strings.TrimSpace(roleClass) {
	case "reviewer":
		return "Review implementation work against the task artifacts and record actionable findings"
	case "orchestrator":
		return "Coordinate task flow, handoffs, and next actions according to the project artifacts"
	default:
		return "Implement the assigned task work according to the task artifacts and current session state"
	}
}
