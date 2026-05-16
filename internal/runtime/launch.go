package runtime

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/provider"
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
	registry provider.Registry
}

// NewBuilder creates a launch builder with OS-backed binary lookup and the default provider registry.
func NewBuilder() *Builder {
	return NewBuilderWithLookPath(exec.LookPath)
}

// NewBuilderWithLookPath creates a launch builder with an injected binary lookup
// and the default provider registry.
func NewBuilderWithLookPath(lookPath LookPathFunc) *Builder {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	return &Builder{
		lookPath: lookPath,
		registry: provider.DefaultRegistry(),
	}
}

// NewBuilderWithRegistry creates a launch builder with an injected binary lookup
// and a custom provider registry. Intended for tests that need to inject a
// specific registry without relying on DefaultRegistry.
func NewBuilderWithRegistry(lookPath LookPathFunc, reg provider.Registry) *Builder {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	return &Builder{
		lookPath: lookPath,
		registry: reg,
	}
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
	p := b.registry.Lookup(spec.Runtime)
	return p.LaunchCommand(provider.LaunchSpec{
		SessionID:      spec.SessionID,
		AgentName:      spec.AgentName,
		RoleName:       spec.RoleName,
		AgentSessionID: spec.AgentSessionID,
		DenyCommands:   spec.DenyCommands,
	}, b.lookPath)
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
