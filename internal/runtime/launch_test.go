package runtime

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/provider"
)

// testIsWSL2 returns true when the test is running inside a WSL2 kernel.
// On WSL2, the codex provider auto-applies --dangerously-bypass-approvals-and-sandbox
// so tests that expect --sandbox danger-full-access must adjust their assertions.
func testIsWSL2() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

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
	if testIsWSL2() {
		// WSL2: auto-bypass kicks in; danger-full-access is never used.
		if !strings.Contains(command, "--dangerously-bypass-approvals-and-sandbox") {
			t.Fatalf("WSL2: command = %q, want --dangerously-bypass-approvals-and-sandbox", command)
		}
	} else {
		if !strings.Contains(command, "exec nice -n 19 codex --sandbox danger-full-access -a never") {
			t.Fatalf("command = %q, want codex exec command with danger-full-access", command)
		}
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
	if !strings.Contains(command, "exec nice -n 10 claude --dangerously-skip-permissions") || !strings.Contains(command, "unset CLAUDECODE") {
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
	if !strings.Contains(command, "exec nice -n 10 claude --resume abc-123-def --dangerously-skip-permissions") || !strings.Contains(command, "unset CLAUDECODE") {
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
	// The sandbox flag differs on WSL2 (auto-bypass) vs non-WSL2 (danger-full-access).
	// The printf format now uses POSIX-compatible \" escapes (not \xNN hex) so dash can execute it.
	sandboxSuffix := "--sandbox danger-full-access -a never"
	if testIsWSL2() {
		sandboxSuffix = "--dangerously-bypass-approvals-and-sandbox"
	}
	want := "sh -lc 'export AOM_RUNTIME=codex; export PYTHONDONTWRITEBYTECODE=1; export npm_config_cache=\"/tmp/aom-npm-cache-$(id -u)\"; export GIT_OPTIONAL_LOCKS=0; export GIT_TERMINAL_PROMPT=0; [ -f \"$HOME/.codex/version.json\" ] || { mkdir -p \"$HOME/.codex\" && printf \"{\\\"dismissed_version\\\":\\\"9999.0.0\\\"}\\n\" > \"$HOME/.codex/version.json\"; }; exec nice -n 19 codex resume sess-xyz-789 " + sandboxSuffix + " -c agents.max_threads=1 -c background_terminal_max_timeout=60000 -c agents.job_max_runtime_seconds=120'"
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}

func TestBuilderBuildFreshStartWhenNoAgentSessionID(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		return "/usr/bin/" + name, nil
	})

	// codexWant differs on WSL2: auto-bypass replaces danger-full-access.
	codexWant := "exec nice -n 19 codex --sandbox danger-full-access -a never"
	if testIsWSL2() {
		codexWant = "exec nice -n 19 codex --dangerously-bypass-approvals-and-sandbox"
	}
	for _, tc := range []struct {
		runtime     string
		wantContain string
	}{
		{"claude", "exec nice -n 10 claude --dangerously-skip-permissions"},
		{"codex", codexWant},
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
	// Smart wrapper must use double-quoted sed (\"s|...|g\"), NOT single-quoted sed ('s|...|').
	// The entire preamble is embedded inside sh -lc '...' by assembleLoginShellCommand; a
	// literal single quote anywhere in the inner content terminates the outer quoted string,
	// causing sh to exit immediately with a syntax error and the tmux pane to close.
	inner := strings.TrimPrefix(command, "sh -lc '")
	inner = strings.TrimSuffix(inner, "'")
	if strings.Contains(inner, "'") {
		t.Fatalf("command inner content contains single quote (breaks sh -lc '...' outer wrapper):\ncommand = %q", command)
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

func TestBuilderBuildCodexBypassSandbox(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		if name == "codex" {
			return "/usr/bin/codex", nil
		}
		return "", fmt.Errorf("unexpected lookup %q", name)
	})

	// Fresh start with BypassSandbox=true must use --dangerously-bypass-approvals-and-sandbox.
	freshCmd, err := builder.Build(SessionSpec{
		Runtime:       "codex",
		BypassSandbox: true,
	}, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build (fresh) failed: %v", err)
	}
	if !strings.Contains(freshCmd, "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("fresh command = %q, want --dangerously-bypass-approvals-and-sandbox", freshCmd)
	}
	if strings.Contains(freshCmd, "--sandbox") {
		t.Fatalf("fresh command = %q, must not contain --sandbox when BypassSandbox=true", freshCmd)
	}

	// Resume with BypassSandbox=true.
	resumeCmd, err := builder.Build(SessionSpec{
		Runtime:        "codex",
		AgentSessionID: "sess-bypass-001",
		BypassSandbox:  true,
	}, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build (resume) failed: %v", err)
	}
	if !strings.Contains(resumeCmd, "codex resume sess-bypass-001 --dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("resume command = %q, want codex resume ... --dangerously-bypass-approvals-and-sandbox", resumeCmd)
	}
	if strings.Contains(resumeCmd, "--sandbox") {
		t.Fatalf("resume command = %q, must not contain --sandbox when BypassSandbox=true", resumeCmd)
	}
}

func TestBuilderBuildCodexPrependsWorkspaceCd(t *testing.T) {
	builder := NewBuilderWithLookPath(func(name string) (string, error) {
		if name == "codex" {
			return "/usr/bin/codex", nil
		}
		return "", fmt.Errorf("unexpected lookup %q", name)
	})

	workspacePath := "/tmp/e2e-proj/.aom/agents/backend-main/workspace"
	command, err := builder.Build(SessionSpec{
		Runtime:      "codex",
		WorktreePath: workspacePath,
	}, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// The cd must appear in the preamble, before the AOM_RUNTIME export.
	cdStatement := fmt.Sprintf(`cd "%s"`, workspacePath)
	if !strings.Contains(command, cdStatement) {
		t.Fatalf("command = %q, want preamble to contain %q", command, cdStatement)
	}
	aomRuntimeIdx := strings.Index(command, "export AOM_RUNTIME=codex")
	cdIdx := strings.Index(command, cdStatement)
	if cdIdx == -1 || aomRuntimeIdx == -1 || cdIdx >= aomRuntimeIdx {
		t.Fatalf("command = %q, want cd %q to appear before export AOM_RUNTIME=codex", command, workspacePath)
	}

	// Without WorktreePath, no cd statement should appear.
	commandNoCd, err := builder.Build(SessionSpec{Runtime: "codex"}, LaunchModeReal)
	if err != nil {
		t.Fatalf("Build (no WorktreePath) failed: %v", err)
	}
	if strings.Contains(commandNoCd, `cd "`) {
		t.Fatalf("command without WorktreePath = %q, must not contain cd statement", commandNoCd)
	}

	// Inner content must not contain single quotes (breaks sh -lc '...' wrapper).
	inner := strings.TrimPrefix(command, "sh -lc '")
	inner = strings.TrimSuffix(inner, "'")
	if strings.Contains(inner, "'") {
		t.Fatalf("command inner content contains single quote:\ncommand = %q", command)
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
