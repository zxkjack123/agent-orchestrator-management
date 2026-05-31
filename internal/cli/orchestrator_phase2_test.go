package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/tmux"
)

// ── Spawn budget guard ────────────────────────────────────────────────────────

func TestIsTerminalSessionStatus(t *testing.T) {
	terminal := []string{"Stopped", "Archived", "Failed"}
	nonTerminal := []string{"Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked", "Detached", "Created"}

	for _, s := range terminal {
		if !isTerminalSessionStatus(s) {
			t.Errorf("%q should be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if isTerminalSessionStatus(s) {
			t.Errorf("%q should NOT be terminal", s)
		}
	}
}

// ── aom task signal — escalation.required ────────────────────────────────────

func TestTaskSignalEscalationRequired(t *testing.T) {
	// Without a project, we should get a missing-arg or project-not-found error,
	// not an "unknown event type" error — confirming the event type is accepted.
	err := runCLI(t, "task", "signal", "escalation.required")
	if err == nil {
		t.Fatal("expected an error (no --task)")
	}
	if strings.Contains(err.Error(), "unknown event type") {
		t.Errorf("escalation.required should be a valid event type, got: %v", err)
	}
}

func TestTaskSignalUnknownTypeStillRejected(t *testing.T) {
	err := runCLI(t, "task", "signal", "bogus.event")
	if err == nil {
		t.Fatal("expected error for unknown event type")
	}
	if !strings.Contains(err.Error(), "unknown event type") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── aom orchestrator status — no project ─────────────────────────────────────

func TestOrchestratorStatusNoProject(t *testing.T) {
	err := runCLI(t, "orchestrator", "status")
	if err == nil {
		t.Fatal("expected error when no project found")
	}
	if !strings.Contains(err.Error(), "project") && !strings.Contains(err.Error(), "no AOM") {
		t.Errorf("expected project-not-found error, got: %v", err)
	}
}

// ── Spawn budget guard integration ───────────────────────────────────────────

func TestSpawnBudgetGuardBlocksOverLimit(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	firstHasSession := true
	restore := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(_ string, args ...string) ([]byte, error) {
			if len(args) == 0 {
				return nil, nil
			}
			switch args[0] {
			case "has-session":
				if firstHasSession {
					firstHasSession = false
					return nil, errors.New("session not found")
				}
				return nil, nil
			case "new-session":
				return nil, nil
			case "split-window":
				return []byte("@1 %5\n"), nil
			case "set-option":
				return nil, nil
			case "display-message":
				return []byte("%5\n"), nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restore()

	var stdout, stderr bytes.Buffer

	// Init project.
	if err := Execute([]string{"project", "init", "budget-test", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init: %v", err)
	}

	// Set max_concurrent_sessions: 1 in policy.yaml.
	policyPath := filepath.Join(repoRoot, ".aom", "policy.yaml")
	policyContent := `policy:
  deny_commands: []
  session_defaults:
    approval_scope: per-session
    yolo_mode: disabled
  owner_exceptions:
    enabled: true
    log_required: true
  max_concurrent_sessions: 1
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("write policy.yaml: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	// Spawn first session — should succeed.
	if err := Execute([]string{"session", "spawn", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("first spawn failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Session spawned") {
		t.Fatalf("expected first spawn to succeed, got: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()

	// Spawn second session — should be blocked by budget guard.
	// --allow-collision is needed here so the collision guard doesn't fire first;
	// we are specifically testing the budget cap, not the collision check.
	err2 := Execute([]string{"session", "spawn", "frontend-main", "--allow-collision"}, &stdout, &stderr)
	if err2 == nil {
		t.Fatal("expected budget guard error for second spawn")
	}
	if !strings.Contains(err2.Error(), "max_concurrent_sessions") {
		t.Errorf("expected budget guard error, got: %v", err2)
	}
}
