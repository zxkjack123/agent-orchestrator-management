package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/provider"
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
	if !strings.Contains(command, "exec codex --sandbox workspace-write -a never") {
		t.Fatalf("command = %q, want codex exec command with -a never", command)
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
	if !strings.Contains(command, "exec claude --dangerously-skip-permissions") || !strings.Contains(command, "unset CLAUDECODE") {
		t.Fatalf("command = %q, want claude exec command with env reset", command)
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
	if !strings.Contains(command, "exec claude --resume abc-123-def --dangerously-skip-permissions") || !strings.Contains(command, "unset CLAUDECODE") {
		t.Fatalf("command = %q, want claude resume command with env reset", command)
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
	want := "sh -lc 'export AOM_RUNTIME=codex; export PYTHONDONTWRITEBYTECODE=1; [ -f \"$HOME/.codex/version.json\" ] || { mkdir -p \"$HOME/.codex\" && printf '{\"dismissed_version\":\"9999.0.0\"}\\n' > \"$HOME/.codex/version.json\"; }; exec codex resume sess-xyz-789 --sandbox workspace-write -a never'"
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}

func TestBuilderBuildFreshStartWhenNoAgentSessionID(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		return "/usr/bin/" + name, nil
	})

	for _, tc := range []struct {
		runtime     string
		wantContain string
	}{
		{"claude", "exec claude --dangerously-skip-permissions"},
		{"codex", "exec codex --sandbox workspace-write -a never"},
	} {
		command, err := builder.Build(SessionSpec{Runtime: tc.runtime}, LaunchModeReal)
		if err != nil {
			t.Fatalf("%s: Build failed: %v", tc.runtime, err)
		}
		if !strings.Contains(command, tc.wantContain) {
			t.Fatalf("%s: command = %q, want to contain %q", tc.runtime, command, tc.wantContain)
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
	if !strings.Contains(command, `"Bash(rm -rf*)"`) {
		t.Fatalf("command = %q, want Bash(rm -rf*) pattern", command)
	}
	if !strings.Contains(command, `"Bash(git push --force*)"`) {
		t.Fatalf("command = %q, want Bash(git push --force*) pattern", command)
	}
}

func TestBuilderBuildCodexWrapsDenyCommands(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		if name == "codex" {
			return "/usr/bin/codex", nil
		}
		return "", fmt.Errorf("unexpected lookup %q", name)
	})

	spec := SessionSpec{
		SessionID:    "SESS-test-123",
		Runtime:      "codex",
		DenyCommands: []string{"rm -rf", "git push --force"},
	}
	command, err := builder.Build(spec, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	// Codex uses PATH wrappers, never --disallowed-tools (that's claude's mechanism).
	if strings.Contains(command, "--disallowed-tools") {
		t.Fatalf("command = %q, codex should not use --disallowed-tools", command)
	}
	// Wrapper bin dir is created for the session.
	wantBinDir := "/tmp/aom-policy-SESS-test-123/bin"
	if !strings.Contains(command, "mkdir -p") {
		t.Fatalf("command = %q, want mkdir -p for wrapper bin dir", command)
	}
	if !strings.Contains(command, wantBinDir) {
		t.Fatalf("command = %q, want wrapper bin dir %q", command, wantBinDir)
	}
	// Wrapper scripts are created for base commands (first word of deny entry).
	for _, wantCmd := range []string{"rm", "git"} {
		wantScript := fmt.Sprintf("%s/%s", wantBinDir, wantCmd)
		if !strings.Contains(command, wantScript) {
			t.Fatalf("command = %q, want wrapper script for %q at %q", command, wantCmd, wantScript)
		}
	}
	// PATH is prepended with the wrapper bin dir before exec.
	wantPathExport := fmt.Sprintf(`export PATH="%s:$PATH"`, wantBinDir)
	if !strings.Contains(command, wantPathExport) {
		t.Fatalf("command = %q, want PATH export %q", command, wantPathExport)
	}

	// P2: git has only multi-word deny entries, so it gets a smart wrapper (not full-block).
	// The smart wrapper checks $1 and passes through for non-matching args.
	if !strings.Contains(command, "push") {
		t.Fatalf("command = %q, want git smart wrapper to contain 'push' subcommand check", command)
	}
	// Smart wrapper uses pass-through via PATH stripping.
	if !strings.Contains(command, "PATH") || !strings.Contains(command, "exec env") {
		t.Fatalf("command = %q, want git smart wrapper to contain pass-through logic (PATH/exec env)", command)
	}

	// Duplicate base commands: rm -rf and rm /tmp produce ONE wrapper that handles both subargs.
	wantDedupSpec := SessionSpec{
		SessionID:    "SESS-dedup-456",
		Runtime:      "codex",
		DenyCommands: []string{"rm -rf", "rm /tmp"},
	}
	dedupCmd, err := builder.Build(wantDedupSpec, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	rmCount := strings.Count(dedupCmd, "/tmp/aom-policy-SESS-dedup-456/bin/rm")
	// Count exact wrapper creation references to check deduplication (one wrapper for rm).
	wantWrapper := `/tmp/aom-policy-SESS-dedup-456/bin/rm" && chmod`
	if count := strings.Count(dedupCmd, wantWrapper); count != 1 {
		t.Fatalf("command = %q, want exactly 1 rm wrapper, got %d (rmCount=%d)", dedupCmd, count, rmCount)
	}
	// The single rm wrapper should contain checks for both -rf and /tmp subargs.
	if !strings.Contains(dedupCmd, "-rf") {
		t.Fatalf("dedupCmd = %q, want rm wrapper to contain '-rf' check", dedupCmd)
	}
	if !strings.Contains(dedupCmd, "/tmp") {
		t.Fatalf("dedupCmd = %q, want rm wrapper to contain '/tmp' check", dedupCmd)
	}
}

func TestNewBuilderWithRegistryUsesCustomRegistry(t *testing.T) {
	// A custom provider that returns a sentinel exec command with no preamble.
	customProvider := &testProvider{name: "custom", execCmd: "exec custom-agent"}
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
	name     string
	preamble []string
	execCmd  string
}

func (p *testProvider) Name() string            { return p.name }
func (p *testProvider) IdentityFilename() string { return "" }
func (p *testProvider) LaunchShellSpec(_ provider.LaunchSpec, _ func(string) (string, error)) (provider.ShellSpec, error) {
	return provider.ShellSpec{Preamble: p.preamble, ExecCmd: p.execCmd}, nil
}
func (p *testProvider) ResumeInfo() provider.ResumeInfo                         { return provider.ResumeInfo{} }
func (p *testProvider) MCPConfigStyle() provider.MCPStyle                       { return provider.MCPStyleNone }
func (p *testProvider) PolicyEnforcementLevel() provider.PolicyEnforcement      { return provider.PolicyEnforcementInstructionOnly }
func (p *testProvider) NativeSessionDetection() *provider.NativeSessionStrategy { return nil }
func (p *testProvider) StartupDialogResponse() string { return "" }
func (p *testProvider) ModelHint() string             { return "" }
func (p *testProvider) KnownModels() []string         { return nil }
