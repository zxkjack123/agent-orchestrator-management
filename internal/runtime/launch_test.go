package runtime

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuilderBuildReturnsPlaceholderCommand(t *testing.T) {
	builder := NewBuilderWithLookPath(func(string) (string, error) { return "", nil })

	command, err := builder.Build(SessionSpec{
		SessionID: "SESS-001",
		AgentName: "backend-main",
		RoleName:  "backend",
		Runtime:   "codex",
	}, LaunchModePlaceholder)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !strings.Contains(command, "AOM session SESS-001") {
		t.Fatalf("command = %q, want placeholder transcript", command)
	}
}

func TestBuilderBuildReturnsMockCommand(t *testing.T) {
	builder := NewBuilderWithLookPath(func(string) (string, error) { return "", nil })

	command, err := builder.Build(SessionSpec{
		SessionID: "SESS-001",
		AgentName: "backend-main",
		RoleName:  "backend",
		Runtime:   "codex",
	}, LaunchModeMock)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !strings.Contains(command, "AOM mock runtime boot") {
		t.Fatalf("command = %q, want mock runtime transcript", command)
	}
}

func TestBuilderBuildReturnsRealCodexCommand(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		if name == "codex" {
			return "/opt/homebrew/bin/codex", nil
		}
		return "", fmt.Errorf("unexpected lookup %q", name)
	})

	command, err := builder.Build(SessionSpec{Runtime: "codex"}, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if command != "sh -lc 'exec codex'" {
		t.Fatalf("command = %q, want codex exec command", command)
	}
}

func TestBuilderBuildReturnsRealClaudeCommand(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		if name == "claude" {
			return "/opt/homebrew/bin/claude", nil
		}
		return "", fmt.Errorf("unexpected lookup %q", name)
	})

	command, err := builder.Build(SessionSpec{Runtime: "claude"}, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if command != "sh -lc 'exec claude --dangerously-skip-permissions'" {
		t.Fatalf("command = %q, want claude exec command", command)
	}
}

func TestBuilderBuildRejectsUnsupportedRealRuntime(t *testing.T) {
	builder := NewBuilderWithLookPath(func(string) (string, error) { return "", nil })

	_, err := builder.Build(SessionSpec{Runtime: "gemini"}, LaunchModeReal)
	if err == nil {
		t.Fatal("Build returned nil error, want unsupported runtime failure")
	}
	if !strings.Contains(err.Error(), `does not support runtime "gemini"`) {
		t.Fatalf("error = %q, want unsupported runtime message", err)
	}
}

func TestBuilderBuildRejectsMissingCodexBinary(t *testing.T) {
	builder := NewBuilderWithLookPath(func(string) (string, error) { return "", fmt.Errorf("missing") })

	_, err := builder.Build(SessionSpec{Runtime: "codex"}, LaunchModeReal)
	if err == nil {
		t.Fatal("Build returned nil error, want missing codex failure")
	}
	if !strings.Contains(err.Error(), `requires the "codex" CLI in PATH`) {
		t.Fatalf("error = %q, want codex PATH message", err)
	}
}

func TestBuilderBuildRejectsMissingClaudeBinary(t *testing.T) {
	builder := NewBuilderWithLookPath(func(string) (string, error) { return "", fmt.Errorf("missing") })

	_, err := builder.Build(SessionSpec{Runtime: "claude"}, LaunchModeReal)
	if err == nil {
		t.Fatal("Build returned nil error, want missing claude failure")
	}
	if !strings.Contains(err.Error(), `requires the "claude" CLI in PATH`) {
		t.Fatalf("error = %q, want claude PATH message", err)
	}
}
