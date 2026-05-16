package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/provider"
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
	if command != "sh -lc 'exec codex --sandbox workspace-write'" {
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
	// gemini is a named stub provider that returns "not yet implemented"
	if !strings.Contains(err.Error(), `"gemini"`) {
		t.Fatalf("error = %q, want error mentioning gemini runtime", err)
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

func TestBuilderBuildResumesClaudeSession(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		if name == "claude" {
			return "/usr/bin/claude", nil
		}
		return "", fmt.Errorf("unexpected lookup %q", name)
	})

	command, err := builder.Build(SessionSpec{
		Runtime:        "claude",
		AgentSessionID: "abc-123-def",
	}, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	want := "sh -lc 'exec claude --resume abc-123-def --dangerously-skip-permissions'"
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}

func TestBuilderBuildResumesCodexSession(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		if name == "codex" {
			return "/usr/bin/codex", nil
		}
		return "", fmt.Errorf("unexpected lookup %q", name)
	})

	command, err := builder.Build(SessionSpec{
		Runtime:        "codex",
		AgentSessionID: "sess-xyz-789",
	}, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	want := "sh -lc 'exec codex resume sess-xyz-789 --sandbox workspace-write'"
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}

func TestBuilderBuildFreshStartWhenNoAgentSessionID(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		return "/usr/bin/" + name, nil
	})

	for _, tc := range []struct {
		runtime string
		want    string
	}{
		{"claude", "sh -lc 'exec claude --dangerously-skip-permissions'"},
		{"codex", "sh -lc 'exec codex --sandbox workspace-write'"},
	} {
		command, err := builder.Build(SessionSpec{Runtime: tc.runtime}, LaunchModeReal)
		if err != nil {
			t.Fatalf("%s: Build failed: %v", tc.runtime, err)
		}
		if command != tc.want {
			t.Fatalf("%s: command = %q, want %q", tc.runtime, command, tc.want)
		}
	}
}

func TestBuilderBuildClaudeWithDenyCommands(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		if name == "claude" {
			return "/usr/bin/claude", nil
		}
		return "", fmt.Errorf("unexpected lookup %q", name)
	})

	command, err := builder.Build(SessionSpec{
		Runtime:      "claude",
		DenyCommands: []string{"rm -rf", "git push --force"},
	}, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !strings.Contains(command, "--disallowed-tools") {
		t.Fatalf("command = %q, want --disallowed-tools flag", command)
	}
	if !strings.Contains(command, "'Bash(rm -rf*)'") {
		t.Fatalf("command = %q, want Bash(rm -rf*) pattern", command)
	}
	if !strings.Contains(command, "'Bash(git push --force*)'") {
		t.Fatalf("command = %q, want Bash(git push --force*) pattern", command)
	}
}

func TestBuilderBuildCodexIgnoresDenyCommands(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		if name == "codex" {
			return "/usr/bin/codex", nil
		}
		return "", fmt.Errorf("unexpected lookup %q", name)
	})

	command, err := builder.Build(SessionSpec{
		Runtime:      "codex",
		DenyCommands: []string{"rm -rf", "git push --force"},
	}, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if strings.Contains(command, "--disallowed-tools") {
		t.Fatalf("command = %q, codex should not contain --disallowed-tools flag", command)
	}
}

func TestNewBuilderWithRegistryUsesCustomRegistry(t *testing.T) {
	// A custom provider that always returns a sentinel command.
	customProvider := &testProvider{name: "custom", launchCmd: "sh -lc 'exec custom-agent'"}
	reg := provider.Registry{"custom": customProvider}

	builder := NewBuilderWithRegistry(func(name string) (string, error) {
		return "/usr/bin/" + name, nil
	}, reg)

	command, err := builder.Build(SessionSpec{Runtime: "custom"}, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if command != "sh -lc 'exec custom-agent'" {
		t.Fatalf("command = %q, want custom-agent exec command", command)
	}
}

// testProvider is a minimal provider.Provider implementation for use in tests.
type testProvider struct {
	name      string
	launchCmd string
}

func (p *testProvider) Name() string            { return p.name }
func (p *testProvider) IdentityFilename() string { return "" }
func (p *testProvider) LaunchCommand(_ provider.LaunchSpec, _ func(string) (string, error)) (string, error) {
	return p.launchCmd, nil
}
func (p *testProvider) ResumeInfo() provider.ResumeInfo                         { return provider.ResumeInfo{} }
func (p *testProvider) MCPConfigStyle() provider.MCPStyle                       { return provider.MCPStyleNone }
func (p *testProvider) PolicyEnforcementLevel() provider.PolicyEnforcement      { return provider.PolicyEnforcementInstructionOnly }
func (p *testProvider) NativeSessionDetection() *provider.NativeSessionStrategy { return nil }
