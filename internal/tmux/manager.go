// Package tmux provides tmux-specific availability and naming helpers.
package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const defaultProjectSessionPrefix = "aom"

// LookPathFunc resolves a binary path.
type LookPathFunc func(string) (string, error)

// ExecFunc executes a command and returns its combined output.
type ExecFunc func(string, ...string) ([]byte, error)

// RunFunc executes a command interactively.
type RunFunc func(string, ...string) error

// Availability describes whether tmux is available to AOM.
type Availability struct {
	Available  bool
	BinaryPath string
}

// Workspace describes the project-level tmux workspace state.
type Workspace struct {
	Name    string
	Target  string
	Created bool
}

// PaneBinding describes a live tmux pane created for an AOM session.
type PaneBinding struct {
	WindowID string
	PaneID   string
}

// Manager owns tmux-specific checks and naming behavior.
type Manager struct {
	lookPath LookPathFunc
	exec     ExecFunc
	run      RunFunc
}

// NewManager creates a tmux manager with OS-backed binary lookup.
func NewManager() *Manager {
	return NewManagerWithDeps(exec.LookPath, combinedOutput, interactiveRun)
}

// NewManagerWithLookPath creates a tmux manager with an injected lookup function.
func NewManagerWithLookPath(lookPath LookPathFunc) *Manager {
	return NewManagerWithDeps(lookPath, combinedOutput, interactiveRun)
}

// NewManagerWithDeps creates a tmux manager with injected external dependencies.
func NewManagerWithDeps(lookPath LookPathFunc, execFunc ExecFunc, runFunc RunFunc) *Manager {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if execFunc == nil {
		execFunc = combinedOutput
	}
	if runFunc == nil {
		runFunc = interactiveRun
	}

	return &Manager{
		lookPath: lookPath,
		exec:     execFunc,
		run:      runFunc,
	}
}

// Availability reports whether tmux can be found in the current environment.
func (m *Manager) Availability() Availability {
	path, err := m.lookPath("tmux")
	if err != nil {
		return Availability{}
	}

	return Availability{
		Available:  true,
		BinaryPath: path,
	}
}

// ProjectSessionName builds a stable tmux session name for one AOM project.
func (m *Manager) ProjectSessionName(sessionPrefix string) string {
	sanitizedPrefix := sanitizeName(sessionPrefix)
	if sanitizedPrefix == "" {
		sanitizedPrefix = "project"
	}

	return defaultProjectSessionPrefix + "-" + sanitizedPrefix
}

// SessionTarget returns the exact tmux session target AOM should use for the project workspace.
func (m *Manager) SessionTarget(sessionPrefix string) string {
	return m.ProjectSessionName(sessionPrefix)
}

// EnsureWorkspace creates or reuses the project tmux session.
func (m *Manager) EnsureWorkspace(sessionPrefix, repoPath string) (*Workspace, error) {
	availability := m.Availability()
	if !availability.Available {
		return nil, fmt.Errorf("tmux is not available in the current environment")
	}

	name := m.ProjectSessionName(sessionPrefix)
	target := m.SessionTarget(sessionPrefix)

	if _, err := m.exec(availability.BinaryPath, "has-session", "-t", target); err == nil {
		return &Workspace{
			Name:    name,
			Target:  target,
			Created: false,
		}, nil
	}

	if _, err := m.exec(availability.BinaryPath, "new-session", "-d", "-s", name, "-c", repoPath); err != nil {
		return nil, fmt.Errorf("create tmux workspace %q: %w", target, err)
	}

	return &Workspace{
		Name:    name,
		Target:  target,
		Created: true,
	}, nil
}

// CreatePane creates one detached pane in the given tmux workspace and returns its binding IDs.
func (m *Manager) CreatePane(sessionTarget, repoPath, command string) (*PaneBinding, error) {
	availability := m.Availability()
	if !availability.Available {
		return nil, fmt.Errorf("tmux is not available in the current environment")
	}

	output, err := m.exec(
		availability.BinaryPath,
		"split-window",
		"-d",
		"-P",
		"-F",
		"#{window_id} #{pane_id}",
		"-t",
		sessionTarget,
		"-c",
		repoPath,
		command,
	)
	if err != nil {
		return nil, fmt.Errorf("create tmux pane in %q: %w", sessionTarget, err)
	}

	fields := strings.Fields(strings.TrimSpace(string(output)))
	if len(fields) != 2 {
		return nil, fmt.Errorf("parse tmux pane binding output %q", strings.TrimSpace(string(output)))
	}

	return &PaneBinding{
		WindowID: fields[0],
		PaneID:   fields[1],
	}, nil
}

// AnnotatePane stores AOM metadata on a pane using tmux user options.
func (m *Manager) AnnotatePane(paneID string, metadata map[string]string) error {
	availability := m.Availability()
	if !availability.Available {
		return fmt.Errorf("tmux is not available in the current environment")
	}

	for key, value := range metadata {
		if strings.TrimSpace(key) == "" {
			continue
		}

		if _, err := m.exec(
			availability.BinaryPath,
			"set-option",
			"-p",
			"-t",
			paneID,
			key,
			value,
		); err != nil {
			return fmt.Errorf("annotate pane %q with %q: %w", paneID, key, err)
		}
	}

	return nil
}

// AttachPane selects the pane and attaches the operator to the tmux workspace.
func (m *Manager) AttachPane(sessionTarget, paneID string) error {
	availability := m.Availability()
	if !availability.Available {
		return fmt.Errorf("tmux is not available in the current environment")
	}

	if _, err := m.exec(
		availability.BinaryPath,
		"select-pane",
		"-t",
		paneID,
	); err != nil {
		return fmt.Errorf("select tmux pane %q: %w", paneID, err)
	}

	if err := m.run(
		availability.BinaryPath,
		"attach-session",
		"-t",
		sessionTarget,
	); err != nil {
		return fmt.Errorf("attach to tmux workspace %q: %w", sessionTarget, err)
	}

	return nil
}

// CapturePane returns the visible pane output.
func (m *Manager) CapturePane(paneID string) (string, error) {
	availability := m.Availability()
	if !availability.Available {
		return "", fmt.Errorf("tmux is not available in the current environment")
	}

	output, err := m.exec(
		availability.BinaryPath,
		"capture-pane",
		"-p",
		"-t",
		paneID,
	)
	if err != nil {
		return "", fmt.Errorf("capture tmux pane %q: %w", paneID, err)
	}

	return string(output), nil
}

// SendKeys sends a literal message followed by Enter into a live tmux pane.
func (m *Manager) SendKeys(paneID, message string) error {
	availability := m.Availability()
	if !availability.Available {
		return fmt.Errorf("tmux is not available in the current environment")
	}
	if strings.TrimSpace(paneID) == "" {
		return fmt.Errorf("pane id is required")
	}
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("message is required")
	}

	if _, err := m.exec(
		availability.BinaryPath,
		"send-keys",
		"-t",
		paneID,
		"-l",
		message,
	); err != nil {
		return fmt.Errorf("send literal keys to tmux pane %q: %w", paneID, err)
	}

	// Brief pause so interactive TUI apps (codex, claude) finish buffering
	// the literal text before the Enter key arrives. Without this, complex
	// TUI input widgets may drop the submission on fast machines.
	time.Sleep(50 * time.Millisecond)

	if _, err := m.exec(
		availability.BinaryPath,
		"send-keys",
		"-t",
		paneID,
		"Enter",
	); err != nil {
		return fmt.Errorf("send enter to tmux pane %q: %w", paneID, err)
	}

	return nil
}

// PaneExists reports whether the given pane target is still live in tmux.
func (m *Manager) PaneExists(paneID string) (bool, error) {
	availability := m.Availability()
	if !availability.Available {
		return false, fmt.Errorf("tmux is not available in the current environment")
	}
	if strings.TrimSpace(paneID) == "" {
		return false, nil
	}

	output, err := m.exec(
		availability.BinaryPath,
		"display-message",
		"-p",
		"-t",
		paneID,
		"#{pane_id}",
	)
	if err != nil {
		return false, nil
	}

	return strings.TrimSpace(string(output)) == strings.TrimSpace(paneID), nil
}

// KillPane intentionally removes a pane from the tmux workspace.
func (m *Manager) KillPane(paneID string) error {
	availability := m.Availability()
	if !availability.Available {
		return fmt.Errorf("tmux is not available in the current environment")
	}
	if strings.TrimSpace(paneID) == "" {
		return fmt.Errorf("pane id is required")
	}

	if _, err := m.exec(
		availability.BinaryPath,
		"kill-pane",
		"-t",
		paneID,
	); err != nil {
		return fmt.Errorf("kill tmux pane %q: %w", paneID, err)
	}

	return nil
}

func combinedOutput(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func interactiveRun(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")

	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			builder.WriteRune(r)
		}
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return ""
	}

	return result
}
