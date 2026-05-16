package runtime

import (
	"fmt"
	"os/exec"
	"strings"
)

// LaunchMode controls how AOM starts a pane process for one session.
type LaunchMode string

const (
	LaunchModePlaceholder LaunchMode = "placeholder"
	LaunchModeMock        LaunchMode = "mock"
	LaunchModeReal        LaunchMode = "real"
)

// SessionSpec contains the runtime details needed to build one launch command.
type SessionSpec struct {
	SessionID      string
	AgentName      string
	RoleName       string
	Runtime        string
	AgentSessionID string   // native vendor session ID; non-empty triggers resume mode
	DenyCommands   []string // commands to block at runtime (claude --disallowed-tools only)
}

// LookPathFunc resolves a runtime binary path.
type LookPathFunc func(string) (string, error)

// Builder owns narrow launch-mode validation and shell command construction.
type Builder struct {
	lookPath LookPathFunc
}

// NewBuilder creates a launch builder with OS-backed binary lookup.
func NewBuilder() *Builder {
	return NewBuilderWithLookPath(exec.LookPath)
}

// NewBuilderWithLookPath creates a launch builder with an injected binary lookup.
func NewBuilderWithLookPath(lookPath LookPathFunc) *Builder {
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	return &Builder{lookPath: lookPath}
}

// Build validates the requested launch mode and returns the pane shell command.
func (b *Builder) Build(spec SessionSpec, mode LaunchMode) (string, error) {
	switch mode {
	case LaunchModePlaceholder:
		return placeholderShellCommand(spec), nil
	case LaunchModeMock:
		return mockRuntimeShellCommand(spec), nil
	case LaunchModeReal:
		return b.realRuntimeShellCommand(spec)
	default:
		return "", fmt.Errorf("launch mode %q is not recognized", mode)
	}
}

func (b *Builder) realRuntimeShellCommand(spec SessionSpec) (string, error) {
	runtimeName := strings.TrimSpace(spec.Runtime)
	switch runtimeName {
	case "codex":
		return b.execRuntimeCommand(spec)
	case "claude":
		return b.execRuntimeCommand(spec)
	default:
		return "", fmt.Errorf("real launch mode does not support runtime %q in the current milestone", runtimeName)
	}
}

func (b *Builder) execRuntimeCommand(spec SessionSpec) (string, error) {
	runtimeName := strings.TrimSpace(spec.Runtime)
	agentSessionID := strings.TrimSpace(spec.AgentSessionID)
	if _, err := b.lookPath(runtimeName); err != nil {
		return "", fmt.Errorf("real launch for runtime %q requires the %q CLI in PATH", runtimeName, runtimeName)
	}
	switch runtimeName {
	case "claude":
		disallowedFlag := buildDisallowedToolsFlag(spec.DenyCommands)
		if agentSessionID != "" {
			if disallowedFlag != "" {
				return fmt.Sprintf("sh -lc 'exec claude --resume %s --dangerously-skip-permissions %s'", agentSessionID, disallowedFlag), nil
			}
			return fmt.Sprintf("sh -lc 'exec claude --resume %s --dangerously-skip-permissions'", agentSessionID), nil
		}
		if disallowedFlag != "" {
			return fmt.Sprintf("sh -lc 'exec claude --dangerously-skip-permissions %s'", disallowedFlag), nil
		}
		return "sh -lc 'exec claude --dangerously-skip-permissions'", nil
	case "codex":
		if agentSessionID != "" {
			return fmt.Sprintf("sh -lc 'exec codex --sandbox workspace-write resume %s'", agentSessionID), nil
		}
		return "sh -lc 'exec codex --sandbox workspace-write'", nil
	default:
		return fmt.Sprintf("sh -lc 'exec %s'", runtimeName), nil
	}
}

// buildDisallowedToolsFlag converts deny_commands into a --disallowed-tools flag string.
// Each command cmd becomes 'Bash(cmd*)'. Returns an empty string when no commands are given.
func buildDisallowedToolsFlag(denyCommands []string) string {
	if len(denyCommands) == 0 {
		return ""
	}
	patterns := make([]string, len(denyCommands))
	for i, cmd := range denyCommands {
		patterns[i] = fmt.Sprintf("'Bash(%s*)'", cmd)
	}
	return "--disallowed-tools " + strings.Join(patterns, " ")
}

func placeholderShellCommand(spec SessionSpec) string {
	return fmt.Sprintf(
		"sh -lc 'printf \"AOM session %s\\nagent=%s\\nrole=%s\\nruntime=%s\\n\"; exec ${SHELL:-sh}'",
		spec.SessionID,
		spec.AgentName,
		spec.RoleName,
		spec.Runtime,
	)
}

func mockRuntimeShellCommand(spec SessionSpec) string {
	continuity := "fresh-start"
	if strings.TrimSpace(spec.AgentSessionID) != "" {
		continuity = "resume=" + spec.AgentSessionID
	}
	return fmt.Sprintf(
		"sh -lc 'printf \"AOM mock runtime boot\\nsession=%s\\nagent=%s\\nrole=%s\\nruntime=%s\\ncontinuity=%s\\nstate=ready-for-operator\\n\"; exec ${SHELL:-sh}'",
		spec.SessionID,
		spec.AgentName,
		spec.RoleName,
		spec.Runtime,
		continuity,
	)
}
