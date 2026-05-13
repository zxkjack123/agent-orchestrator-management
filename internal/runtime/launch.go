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
	SessionID string
	AgentName string
	RoleName  string
	Runtime   string
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
		if _, err := b.lookPath("codex"); err != nil {
			return "", fmt.Errorf("real launch for runtime %q requires the %q CLI in PATH", runtimeName, runtimeName)
		}
		return "sh -lc 'exec codex'", nil
	default:
		return "", fmt.Errorf("real launch mode does not support runtime %q in the current milestone", runtimeName)
	}
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
	return fmt.Sprintf(
		"sh -lc 'printf \"AOM mock runtime boot\\nsession=%s\\nagent=%s\\nrole=%s\\nruntime=%s\\nstate=ready-for-operator\\n\"; exec ${SHELL:-sh}'",
		spec.SessionID,
		spec.AgentName,
		spec.RoleName,
		spec.Runtime,
	)
}
