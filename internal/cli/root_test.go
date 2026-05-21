package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/app"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/project"
	aomruntime "github.com/lattapon-aek/agents-orchestrator-management-private/internal/runtime"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/step"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/tmux"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/worktree"
)

func TestExecuteProjectInitCreatesAOMStructure(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err = Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	requiredPaths := []string{
		filepath.Join(repoRoot, ".aom", "project.yaml"),
		filepath.Join(repoRoot, ".aom", "agents.yaml"),
		filepath.Join(repoRoot, ".aom", "resources.yaml"),
		filepath.Join(repoRoot, ".aom", "policy.yaml"),
		filepath.Join(repoRoot, ".aom", "sessions.db"),
	}

	for _, path := range requiredPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Stat(%q) failed: %v", path, err)
		}
	}

	if got := stdout.String(); !strings.Contains(got, "Project initialized") {
		t.Fatalf("stdout = %q, want project initialized message", got)
	}
}

func TestExecuteProjectInitFiltersSelectedAgents(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err = Execute([]string{"project", "init", "my-app", "--repo", repoRoot, "--agents", "backend-main,reviewer-main"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	agentsData, err := os.ReadFile(filepath.Join(repoRoot, ".aom", "agents.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(agents.yaml) failed: %v", err)
	}
	content := string(agentsData)
	if !strings.Contains(content, "backend-main:") || !strings.Contains(content, "reviewer-main:") {
		t.Fatalf("agents.yaml = %q, want selected agents", content)
	}
	if strings.Contains(content, "orchestrator-main:") {
		t.Fatalf("agents.yaml = %q, do not want filtered-out agent", content)
	}
}

func TestExecuteProjectInitSupportsInlineAgentDefinitions(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err = Execute([]string{"project", "init", "my-app", "--repo", repoRoot, "--agents", "backend-main,frontend-main:builder:claude,reviewer-main"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	agentsData, err := os.ReadFile(filepath.Join(repoRoot, ".aom", "agents.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(agents.yaml) failed: %v", err)
	}
	content := string(agentsData)
	if !strings.Contains(content, "frontend-main:") || !strings.Contains(content, "runtime: claude") || !strings.Contains(content, "role: builder") {
		t.Fatalf("agents.yaml = %q, want inline frontend agent", content)
	}
}

func TestExecuteProjectInitInteractiveAgentSelection(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	runner := Runner{
		app:    app.New(),
		stdin:  strings.NewReader("backend-main,reviewer-main\n"),
		stdout: &stdout,
		stderr: &stderr,
		isTTY: func(io.Reader) bool {
			return true
		},
	}

	if err := runner.Execute([]string{"project", "init", "my-app", "--repo", repoRoot}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	agentsData, err := os.ReadFile(filepath.Join(repoRoot, ".aom", "agents.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(agents.yaml) failed: %v", err)
	}
	content := string(agentsData)
	if !strings.Contains(content, "backend-main:") || !strings.Contains(content, "reviewer-main:") {
		t.Fatalf("agents.yaml = %q, want selected agents", content)
	}
	if strings.Contains(content, "orchestrator-main:") {
		t.Fatalf("agents.yaml = %q, do not want filtered-out agent", content)
	}

	if got := stdout.String(); !strings.Contains(got, "Select agents to enable") {
		t.Fatalf("stdout = %q, want interactive prompt", got)
	}
}

func TestExecuteProjectInitInteractiveSupportsInlineAgentSelection(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	runner := Runner{
		app:    app.New(),
		stdin:  strings.NewReader("backend-main,custom-agent:builder:codex\n"),
		stdout: &stdout,
		stderr: &stderr,
		isTTY: func(io.Reader) bool {
			return true
		},
	}

	if err := runner.Execute([]string{"project", "init", "my-app", "--repo", repoRoot}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	agentsData, err := os.ReadFile(filepath.Join(repoRoot, ".aom", "agents.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(agents.yaml) failed: %v", err)
	}
	content := string(agentsData)
	if !strings.Contains(content, "custom-agent:") || !strings.Contains(content, "runtime: codex") || !strings.Contains(content, "role: builder") {
		t.Fatalf("agents.yaml = %q, want inline custom agent", content)
	}
	if !strings.Contains(stdout.String(), "name:role:runtime") {
		t.Fatalf("stdout = %q, want inline syntax hint", stdout.String())
	}
}

func TestExecuteProjectInitInteractiveBlankSelectionKeepsDefaultAgents(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	runner := Runner{
		app:    app.New(),
		stdin:  strings.NewReader("\n"),
		stdout: &stdout,
		stderr: &stderr,
		isTTY: func(io.Reader) bool {
			return true
		},
	}

	if err := runner.Execute([]string{"project", "init", "my-app", "--repo", repoRoot}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	agentsData, err := os.ReadFile(filepath.Join(repoRoot, ".aom", "agents.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(agents.yaml) failed: %v", err)
	}
	content := string(agentsData)
	if !strings.Contains(content, "backend-main:") || !strings.Contains(content, "reviewer-main:") || !strings.Contains(content, "frontend-main:") {
		t.Fatalf("agents.yaml = %q, want default full agent set", content)
	}
}

func TestExecuteOpenShowsProjectSummary(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	if err := Execute([]string{"open"}, &stdout, &stderr); err != nil {
		t.Fatalf("open failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Project opened") {
		t.Fatalf("stdout = %q, want Project opened", out)
	}
	if !strings.Contains(out, "backend-main") {
		t.Fatalf("stdout = %q, want backend-main in summary", out)
	}
	if !strings.Contains(out, "Terminal:") {
		t.Fatalf("stdout = %q, want Terminal section", out)
	}
	if !strings.Contains(out, "Workspace: aom-my-app") {
		t.Fatalf("stdout = %q, want workspace summary", out)
	}
	if !strings.Contains(out, "Workspace state: reused") {
		t.Fatalf("stdout = %q, want workspace state", out)
	}
	if !strings.Contains(out, "Sessions:") {
		t.Fatalf("stdout = %q, want Sessions section", out)
	}
	if !strings.Contains(out, "  None") {
		t.Fatalf("stdout = %q, want no sessions placeholder", out)
	}
}

func TestExecuteStatusShowsProjectSummary(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "", errors.New("not found") },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Project status") {
		t.Fatalf("stdout = %q, want Project status", out)
	}
	if !strings.Contains(out, "Agents:") {
		t.Fatalf("stdout = %q, want Agents section", out)
	}
	if !strings.Contains(out, "Terminal:") {
		t.Fatalf("stdout = %q, want Terminal section", out)
	}
	if !strings.Contains(out, "Sessions:") {
		t.Fatalf("stdout = %q, want Sessions section", out)
	}
}

func TestExecuteSessionSpawnAndList(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	if err := Execute([]string{"session", "spawn", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Session spawned") {
		t.Fatalf("stdout = %q, want Session spawned", out)
	}
	if !strings.Contains(out, "Pane: %5") {
		t.Fatalf("stdout = %q, want pane binding", out)
	}
	if !strings.Contains(out, "Launch mode: placeholder") {
		t.Fatalf("stdout = %q, want placeholder launch mode", out)
	}

	stdout.Reset()
	stderr.Reset()

	if err := Execute([]string{"session", "list"}, &stdout, &stderr); err != nil {
		t.Fatalf("session list failed: %v", err)
	}

	out = stdout.String()
	if !strings.Contains(out, "Sessions") {
		t.Fatalf("stdout = %q, want Sessions header", out)
	}
	if !strings.Contains(out, "agent=backend-main") {
		t.Fatalf("stdout = %q, want agent listing", out)
	}
	if !strings.Contains(out, "tmux=aom-my-app @1 %5") {
		t.Fatalf("stdout = %q, want tmux binding", out)
	}
}

func TestExecuteSessionSpawnWithMockRuntime(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	var splitCommands []string
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				splitCommands = append(splitCommands, args[len(args)-1])
				return []byte("@1 %5\n"), nil
			case "set-option":
				return nil, nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	if err := Execute([]string{"session", "spawn", "backend-main", "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Launch mode: mock") {
		t.Fatalf("stdout = %q, want mock launch mode", out)
	}
	if len(splitCommands) != 1 {
		t.Fatalf("len(splitCommands) = %d, want 1", len(splitCommands))
	}
	if !strings.Contains(splitCommands[0], "AOM mock runtime boot") {
		t.Fatalf("split command = %q, want mock runtime transcript", splitCommands[0])
	}
}

func TestExecuteSessionSpawnWithRealRuntime(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	var splitCommands []string
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				splitCommands = append(splitCommands, args[len(args)-1])
				return []byte("@1 %5\n"), nil
			case "set-option":
				return nil, nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	restoreLaunchBuilder := stubLaunchBuilderFactory(t, aomruntime.NewBuilderWithLookPath(
		func(string) (string, error) { return "/opt/homebrew/bin/codex", nil },
	))
	defer restoreLaunchBuilder()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	if err := Execute([]string{"session", "spawn", "backend-main", "--real"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Launch mode: real") {
		t.Fatalf("stdout = %q, want real launch mode", out)
	}
	if len(splitCommands) != 1 {
		t.Fatalf("len(splitCommands) = %d, want 1", len(splitCommands))
	}
	if !strings.Contains(splitCommands[0], "exec codex --sandbox workspace-write -a never") {
		t.Fatalf("split command = %q, want codex exec launch", splitCommands[0])
	}
}

func TestExecuteSessionSpawnWithRealClaudeRuntime(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	var splitCommands []string
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				splitCommands = append(splitCommands, args[len(args)-1])
				return []byte("@1 %5\n"), nil
			case "set-option":
				return nil, nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	restoreLaunchBuilder := stubLaunchBuilderFactory(t, aomruntime.NewBuilderWithLookPath(
		func(string) (string, error) { return "/opt/homebrew/bin/claude", nil },
	))
	defer restoreLaunchBuilder()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	if err := Execute([]string{"session", "spawn", "reviewer-main", "--real"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Launch mode: real") {
		t.Fatalf("stdout = %q, want real launch mode", out)
	}
	if len(splitCommands) != 1 {
		t.Fatalf("len(splitCommands) = %d, want 1", len(splitCommands))
	}
	// The command must launch claude with the permissions flag.
	// When policy.yaml configures deny_commands the --disallowed-tools flag
	// will also be present; we check the essential parts rather than exact equality.
	if !strings.Contains(splitCommands[0], "exec claude") ||
		!strings.Contains(splitCommands[0], "--dangerously-skip-permissions") {
		t.Fatalf("split command = %q, want claude exec launch", splitCommands[0])
	}
}

func TestExecuteSessionSpawnWithTaskRealMaterializesCodexIdentityFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree identity materialization test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
	defer restoreAppFactory()

	restoreLaunchBuilder := stubLaunchBuilderFactory(t, aomruntime.NewBuilderWithLookPath(
		func(string) (string, error) { return "/opt/homebrew/bin/codex", nil },
	))
	defer restoreLaunchBuilder()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Materialize codex identity", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--real"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session show failed: %v", err)
	}
	worktreePath := extractLineValue(stdout.String(), "Worktree: ")
	if worktreePath == "" {
		t.Fatalf("could not extract worktree path from %q", stdout.String())
	}

	identityData, err := os.ReadFile(filepath.Join(worktreePath, "AGENTS.md"))
	if err != nil {
		t.Fatalf("ReadFile(AGENTS.md) failed: %v", err)
	}
	if !strings.Contains(string(identityData), "Agent: backend-main") {
		t.Fatalf("AGENTS.md = %q, want backend profile content", string(identityData))
	}
}

func TestExecuteSessionSpawnWithTaskRealMaterializesClaudeIdentityFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree identity materialization test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
	defer restoreAppFactory()

	restoreLaunchBuilder := stubLaunchBuilderFactory(t, aomruntime.NewBuilderWithLookPath(
		func(string) (string, error) { return "/opt/homebrew/bin/claude", nil },
	))
	defer restoreLaunchBuilder()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Materialize frontend identity", "--role", "frontend", "--agent", "frontend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "frontend-main", "--task", taskID, "--real"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session show failed: %v", err)
	}
	worktreePath := extractLineValue(stdout.String(), "Worktree: ")
	if worktreePath == "" {
		t.Fatalf("could not extract worktree path from %q", stdout.String())
	}

	identityData, err := os.ReadFile(filepath.Join(worktreePath, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile(CLAUDE.md) failed: %v", err)
	}
	if !strings.Contains(string(identityData), "Agent: frontend-main") {
		t.Fatalf("CLAUDE.md = %q, want frontend profile content", string(identityData))
	}
}

func TestExecuteSessionSpawnWithRealRuntimeRejectsUnsupportedAgent(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var splitCount int
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "split-window" {
				splitCount++
			}
			return nil, nil
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	restoreLaunchBuilder := stubLaunchBuilderFactory(t, aomruntime.NewBuilderWithLookPath(
		func(string) (string, error) { return "/opt/homebrew/bin/codex", nil },
	))
	defer restoreLaunchBuilder()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	agentsPath := filepath.Join(repoRoot, ".aom", "agents.yaml")
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile(agents.yaml) failed: %v", err)
	}
	updatedAgents := strings.Replace(string(agentsData), "  reviewer-main:\n    runtime: claude", "  reviewer-main:\n    runtime: gemini", 1)
	if updatedAgents == string(agentsData) {
		t.Fatalf("agents.yaml = %q, want reviewer-main runtime fixture", string(agentsData))
	}
	if err := os.WriteFile(agentsPath, []byte(updatedAgents), 0o644); err != nil {
		t.Fatalf("WriteFile(agents.yaml) failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	err = Execute([]string{"session", "spawn", "reviewer-main", "--real"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("session spawn returned nil error, want unsupported runtime failure")
	}
	// gemini is a named stub provider that returns "not yet implemented"
	if !strings.Contains(err.Error(), `"gemini"`) {
		t.Fatalf("error = %q, want error mentioning gemini runtime", err)
	}
	if splitCount != 0 {
		t.Fatalf("splitCount = %d, want no tmux pane creation", splitCount)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Sessions: 0") && !strings.Contains(stdout.String(), "  Sessions: 0") {
		t.Fatalf("stdout = %q, want zero sessions after rejected real launch", stdout.String())
	}
}

func TestExecuteSessionSpawnRejectsConflictingLaunchFlags(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	err = Execute([]string{"session", "spawn", "backend-main", "--mock", "--real"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("session spawn returned nil error, want conflicting flag failure")
	}
	if err.Error() != "--mock and --real cannot be used together" {
		t.Fatalf("error = %q, want conflicting launch flag message", err)
	}
}

func TestExecuteSessionSpawnWithTaskRefreshesArtifacts(t *testing.T) {
	repoRoot := t.TempDir()
	// Resolve symlinks so path comparisons work on macOS (/var → /private/var).
	if resolved, err := filepath.EvalSymlinks(repoRoot); err == nil {
		repoRoot = resolved
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Bind session to task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Task: "+taskID) {
		t.Fatalf("stdout = %q, want task in spawn output", out)
	}
	if !strings.Contains(out, "Worktree status: Active") {
		t.Fatalf("stdout = %q, want active worktree status", out)
	}
	spawnWorktreePath := extractLineValue(out, "Worktree path: ")
	if spawnWorktreePath == "" {
		t.Fatalf("stdout = %q, want worktree path", out)
	}
	worktreesDir := filepath.Join(repoRoot, ".aom", "worktrees")
	if !strings.HasPrefix(spawnWorktreePath, worktreesDir) {
		t.Fatalf("worktree path = %q, want path inside %q", spawnWorktreePath, worktreesDir)
	}
	sessionID := extractSessionID(out)
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "list"}, &stdout, &stderr); err != nil {
		t.Fatalf("session list failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "task="+taskID) {
		t.Fatalf("stdout = %q, want task id in session list", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Task: "+taskID) {
		t.Fatalf("stdout = %q, want task in session show", out)
	} else if worktreePath := extractLineValue(out, "Worktree: "); !strings.HasPrefix(worktreePath, filepath.Join(repoRoot, ".aom", "worktrees")) {
		t.Fatalf("worktree path = %q, want path inside %q", worktreePath, filepath.Join(repoRoot, ".aom", "worktrees"))
	}

	indexData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "index.md"))
	if err != nil {
		t.Fatalf("ReadFile(index.md) failed: %v", err)
	}
	if !strings.Contains(string(indexData), "Active Session: "+sessionID) {
		t.Fatalf("index.md = %q, want active session", string(indexData))
	}

	logData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "session.created") {
		t.Fatalf("log.md = %q, want session.created event", string(logData))
	}
	if !strings.Contains(string(logData), "session.ready") {
		t.Fatalf("log.md = %q, want session.ready event", string(logData))
	}
	if !strings.Contains(string(logData), "Session Booting") {
		t.Fatalf("log.md = %q, want booting lifecycle state", string(logData))
	}
	if !strings.Contains(string(logData), "Session Idle") {
		t.Fatalf("log.md = %q, want idle lifecycle state", string(logData))
	}

	handoffData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "handoff.md"))
	if err != nil {
		t.Fatalf("ReadFile(handoff.md) failed: %v", err)
	}
	if !strings.Contains(string(handoffData), "From Session: "+sessionID) {
		t.Fatalf("handoff.md = %q, want session id", string(handoffData))
	}
	if !strings.Contains(string(handoffData), "From Runtime: codex") {
		t.Fatalf("handoff.md = %q, want runtime", string(handoffData))
	}
	if !strings.Contains(string(handoffData), "Exact Next Action") {
		t.Fatalf("handoff.md = %q, want exact next action section", string(handoffData))
	}
}

func TestExecuteSessionSpawnBlocksSecondDedicatedWriterForSameTask(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	splitCount := 0
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				splitCount++
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
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Enforce writer boundary", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("first session spawn failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	err = Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("second dedicated writer spawn returned nil error, want boundary failure")
	}
	if !strings.Contains(err.Error(), "already has active writer session") {
		t.Fatalf("error = %q, want active writer session message", err)
	}
	if splitCount != 1 {
		t.Fatalf("splitCount = %d, want 1 successful pane launch", splitCount)
	}
}

func TestExecuteSessionSpawnAllowsReadOnlyRoleAlongsideDedicatedWriter(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	splitCount := 0
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				splitCount++
				if splitCount == 1 {
					return []byte("@1 %5\n"), nil
				}
				return []byte("@1 %6\n"), nil
			case "set-option":
				return nil, nil
			case "display-message":
				if splitCount >= 2 {
					return []byte("%6\n"), nil
				}
				return []byte("%5\n"), nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Allow reviewer alongside writer", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("writer session spawn failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "reviewer-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("reviewer session spawn failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Agent: reviewer-main") {
		t.Fatalf("stdout = %q, want reviewer session output", out)
	}
	if splitCount != 2 {
		t.Fatalf("splitCount = %d, want both sessions launched", splitCount)
	}
}

func TestExecuteSessionSpawnAllowsReplacementAfterDetachedWriter(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for detached writer integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				return nil, errors.New("pane not found")
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Detached writer should not block respawn", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("first session spawn failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "status=Detached") {
		t.Fatalf("stdout = %q, want detached status after reconciliation", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("second session spawn failed after detached reconciliation: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Session spawned") {
		t.Fatalf("stdout = %q, want spawn success", out)
	}
}

func TestExecuteCheckpointCreatesCanonicalLogEvent(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Checkpoint task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"checkpoint", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("checkpoint failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Checkpoint created") {
		t.Fatalf("stdout = %q, want checkpoint confirmation", out)
	}
	checkpointID := extractEntityID(out, "Checkpoint: ")
	if checkpointID == "" {
		t.Fatalf("could not extract checkpoint id from %q", out)
	}

	logData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "checkpoint.created") || !strings.Contains(string(logData), checkpointID) {
		t.Fatalf("log.md = %q, want checkpoint event with id", string(logData))
	}

	indexData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "index.md"))
	if err != nil {
		t.Fatalf("ReadFile(index.md) failed: %v", err)
	}
	if !strings.Contains(string(indexData), "Latest Checkpoint: "+checkpointID) {
		t.Fatalf("index.md = %q, want latest checkpoint id", string(indexData))
	}
}

func TestExecuteHandoffWritesHandoffArtifactAndMarksSessionWaiting(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Handoff task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"handoff", sessionID, "--to", "reviewer-main", "--reason", "ready for review"}, &stdout, &stderr); err != nil {
		t.Fatalf("handoff failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Handoff prepared") {
		t.Fatalf("stdout = %q, want handoff confirmation", out)
	}
	if !strings.Contains(out, "To role: reviewer") || !strings.Contains(out, "To agent: reviewer-main") {
		t.Fatalf("stdout = %q, want reviewer handoff target", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: WaitingHandoff") {
		t.Fatalf("stdout = %q, want WaitingHandoff status", out)
	}

	handoffData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "handoff.md"))
	if err != nil {
		t.Fatalf("ReadFile(handoff.md) failed: %v", err)
	}
	if !strings.Contains(string(handoffData), "To Role: reviewer") || !strings.Contains(string(handoffData), "From Session: "+sessionID) {
		t.Fatalf("handoff.md = %q, want populated transfer packet", string(handoffData))
	}

	indexData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "index.md"))
	if err != nil {
		t.Fatalf("ReadFile(index.md) failed: %v", err)
	}
	if !strings.Contains(string(indexData), "handoff.md: present") {
		t.Fatalf("index.md = %q, want handoff artifact inventory", string(indexData))
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Preferred role: reviewer") || !strings.Contains(out, "Preferred agent: reviewer-main") {
		t.Fatalf("stdout = %q, want ownership moved to reviewer-main", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "role=reviewer") || !strings.Contains(out, "agent=reviewer-main") {
		t.Fatalf("stdout = %q, want active step ownership moved to reviewer-main", out)
	}
}

func TestExecuteSessionSpawnWithTaskLogsFailureWhenPaneCreationFails(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				return nil, errors.New("split failed")
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Bind failing session to task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	err = Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("session spawn should fail when pane creation fails")
	}
	if !strings.Contains(err.Error(), "split failed") {
		t.Fatalf("error = %q, want split failure", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "list"}, &stdout, &stderr); err != nil {
		t.Fatalf("session list failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "status=Failed") {
		t.Fatalf("stdout = %q, want failed session status", out)
	}

	logData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "session.failed") {
		t.Fatalf("log.md = %q, want session.failed event", string(logData))
	}
	if !strings.Contains(string(logData), "session.created") {
		t.Fatalf("log.md = %q, want session.created event before failure", string(logData))
	}
	if !strings.Contains(string(logData), "Session Failed") {
		t.Fatalf("log.md = %q, want failed lifecycle state", string(logData))
	}
}

func TestExecuteHandoffToRoleOnlyPreservesStatusTransfersOwnership(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Role-only handoff task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	stdout.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	stepID := extractStepID(stdout.String())

	stdout.Reset()
	if err := Execute([]string{"task", "update", taskID, "--status", "ready"}, &stdout, &stderr); err != nil {
		t.Fatalf("task update to ready failed: %v", err)
	}
	stdout.Reset()
	if err := Execute([]string{"task", "update", taskID, "--status", "in-progress"}, &stdout, &stderr); err != nil {
		t.Fatalf("task update to in-progress failed: %v", err)
	}
	stdout.Reset()
	if err := Execute([]string{"step", "update", stepID, "--status", "confirmed"}, &stdout, &stderr); err != nil {
		t.Fatalf("step update to confirmed failed: %v", err)
	}
	stdout.Reset()
	if err := Execute([]string{"step", "update", stepID, "--status", "ready"}, &stdout, &stderr); err != nil {
		t.Fatalf("step update to ready failed: %v", err)
	}
	stdout.Reset()
	if err := Execute([]string{"step", "update", stepID, "--status", "in-progress"}, &stdout, &stderr); err != nil {
		t.Fatalf("step update to in-progress failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"handoff", sessionID, "--to", "reviewer"}, &stdout, &stderr); err != nil {
		t.Fatalf("handoff failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "To agent: -") {
		t.Fatalf("stdout = %q, want role-only handoff", out)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	// Handoff preserves InProgress status; only transfers ownership to the new role.
	if out := stdout.String(); !strings.Contains(out, "Status: InProgress") || !strings.Contains(out, "Preferred role: reviewer") || !strings.Contains(out, "Preferred agent: -") {
		t.Fatalf("stdout = %q, want role-only handoff to keep InProgress and transfer ownership to reviewer", out)
	}

	stdout.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, stepID) || !strings.Contains(out, "status=InProgress") || !strings.Contains(out, "role=reviewer") || !strings.Contains(out, "agent=-") {
		t.Fatalf("stdout = %q, want active step to keep InProgress under reviewer role", out)
	}
}

func TestExecuteSessionSpawnWithTaskLogsFailureWhenPaneAnnotationFails(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				return nil, errors.New("annotate failed")
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Bind annotated session to task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	err = Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("session spawn should fail when pane annotation fails")
	}
	if !strings.Contains(err.Error(), "annotate failed") {
		t.Fatalf("error = %q, want annotate failure", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "list"}, &stdout, &stderr); err != nil {
		t.Fatalf("session list failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "status=Failed") {
		t.Fatalf("stdout = %q, want failed session status", out)
	}

	logData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "session.created") {
		t.Fatalf("log.md = %q, want session.created event", string(logData))
	}
	if !strings.Contains(string(logData), "session.ready") {
		t.Fatalf("log.md = %q, want session.ready event before annotation failure", string(logData))
	}
	if !strings.Contains(string(logData), "session.failed") {
		t.Fatalf("log.md = %q, want session.failed event", string(logData))
	}
}

func TestExecuteSessionShowAttachAndCapture(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
			case "select-pane":
				return nil, nil
			case "capture-pane":
				return []byte("hello from pane\n"), nil
			default:
				return nil, nil
			}
		},
		func(name string, args ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}

	spawnOut := stdout.String()
	if !strings.Contains(spawnOut, "Session: SESS-") {
		t.Fatalf("stdout = %q, want session id", spawnOut)
	}
	sessionID := extractSessionID(spawnOut)
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", spawnOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Tmux pane: %5") {
		t.Fatalf("stdout = %q, want pane detail", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"capture", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "hello from pane") {
		t.Fatalf("stdout = %q, want captured pane output", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"attach", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("attach failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Attaching to "+sessionID+" (%5)") {
		t.Fatalf("stdout = %q, want attach summary", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Sessions:") {
		t.Fatalf("stdout = %q, want Sessions section", out)
	}
	if !strings.Contains(out, "agent=backend-main") {
		t.Fatalf("stdout = %q, want session summary row", out)
	}
	if !strings.Contains(out, "Sessions: 1") && !strings.Contains(out, "  Sessions: 1") {
		t.Fatalf("stdout = %q, want session count", out)
	}
}

func TestExecuteAttachLogsOperatorInterventionForTaskBoundSession(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
			case "select-pane":
				return nil, nil
			default:
				return nil, nil
			}
		},
		func(name string, args ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Intervene in active task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"attach", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("attach failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Attaching to "+sessionID+" (%5)") {
		t.Fatalf("stdout = %q, want attach summary", out)
	}

	indexData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "index.md"))
	if err != nil {
		t.Fatalf("ReadFile(index.md) failed: %v", err)
	}
	if !strings.Contains(string(indexData), "Active Session: "+sessionID) {
		t.Fatalf("index.md = %q, want active session after attach", string(indexData))
	}

	logData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "operator.intervention") {
		t.Fatalf("log.md = %q, want operator.intervention event", string(logData))
	}
	if !strings.Contains(string(logData), "Re-analysis required") {
		t.Fatalf("log.md = %q, want re-analysis marker", string(logData))
	}
}

func TestExecuteSessionSendDeliversPromptAndAppendsTaskEvent(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
			case "send-keys":
				return nil, nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Deliver prompt into task session", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "send", sessionID, "read", ".agent/task.md", "and", "begin", "work"}, &stdout, &stderr); err != nil {
		t.Fatalf("session send failed: %v", err)
	}
	sendOut := stdout.String()
	if !strings.Contains(sendOut, "Prompt delivered") {
		t.Fatalf("stdout = %q, want prompt summary", sendOut)
	}
	if !strings.Contains(sendOut, "Session: "+sessionID) {
		t.Fatalf("stdout = %q, want session id", sendOut)
	}
	if !strings.Contains(sendOut, "Pane: %5") {
		t.Fatalf("stdout = %q, want pane target", sendOut)
	}
	if !strings.Contains(sendOut, "Message: read .agent/task.md and begin work") {
		t.Fatalf("stdout = %q, want joined message", sendOut)
	}

	logData, err := os.ReadFile(filepath.Join(taskArtifactDir(repoRoot, taskID), "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "orchestrator.prompt") {
		t.Fatalf("log.md = %q, want orchestrator.prompt event", string(logData))
	}
	if !strings.Contains(string(logData), "Prompt delivered to session "+sessionID+": read .agent/task.md and begin work") {
		t.Fatalf("log.md = %q, want prompt summary", string(logData))
	}
}

func TestExecuteTaskCreateShowAndStepList(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Implement milestone 3", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}

	createOut := stdout.String()
	if !strings.Contains(createOut, "Task created") {
		t.Fatalf("stdout = %q, want Task created", createOut)
	}
	taskID := extractEntityID(createOut, "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", createOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	showOut := stdout.String()
	if !strings.Contains(showOut, "Mode: Direct") {
		t.Fatalf("stdout = %q, want Direct mode", showOut)
	}
	if !strings.Contains(showOut, "Status: Planned") {
		t.Fatalf("stdout = %q, want Planned status", showOut)
	}
	if !strings.Contains(showOut, "Worktree status: Ready") {
		t.Fatalf("stdout = %q, want ready worktree status", showOut)
	}
	if !strings.Contains(showOut, "Worktree branch: aom/") {
		t.Fatalf("stdout = %q, want worktree branch", showOut)
	}
	if !strings.Contains(showOut, ".aom") || !strings.Contains(showOut, "worktrees") {
		t.Fatalf("stdout = %q, want worktree path", showOut)
	}
	artifactRoot := extractLineValue(showOut, "Artifact root: ")
	if artifactRoot == "" {
		t.Fatalf("stdout = %q, want artifact root", showOut)
	}
	taskLog := extractLineValue(showOut, "Task log: ")
	if taskLog == "" {
		t.Fatalf("stdout = %q, want task log path", showOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	stepOut := stdout.String()
	if !strings.Contains(stepOut, "type=implementation") {
		t.Fatalf("stdout = %q, want implementation step", stepOut)
	}
	if !strings.Contains(stepOut, "status=Proposed") {
		t.Fatalf("stdout = %q, want Proposed step", stepOut)
	}
	if !strings.Contains(stepOut, "agent=backend-main") {
		t.Fatalf("stdout = %q, want preferred agent", stepOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Tasks: 1") && !strings.Contains(out, "  Tasks: 1") {
		t.Fatalf("stdout = %q, want task count", out)
	}
	if !strings.Contains(out, "title=Implement milestone 3") {
		t.Fatalf("stdout = %q, want task detail row", out)
	}
	if !strings.Contains(out, "worktree=Ready | branch=aom/") {
		t.Fatalf("stdout = %q, want ready worktree summary", out)
	}
	if !strings.Contains(out, filepath.Join(".aom", "worktrees")) {
		t.Fatalf("stdout = %q, want worktree artifact path summary", out)
	}
	if !strings.Contains(out, "next=confirm the proposed step and move the task to Ready") {
		t.Fatalf("stdout = %q, want recommended next action", out)
	}
	if !strings.Contains(out, "* STEP-") || !strings.Contains(out, "status=Proposed") {
		t.Fatalf("stdout = %q, want task step summary", out)
	}

	artifactDir := taskArtifactDir(repoRoot, taskID)
	for _, name := range []string{"task.md", "state.md", "index.md", "log.md"} {
		if _, err := os.Stat(filepath.Join(artifactDir, name)); err != nil {
			t.Fatalf("artifact %s missing: %v", name, err)
		}
	}
}

func TestExecuteTaskCreateProvisionsWorktreeWhenRepoIsGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree provisioning integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Implement worktree provisioning", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	showOut := stdout.String()
	if !strings.Contains(showOut, "Worktree status: Ready") {
		t.Fatalf("stdout = %q, want ready worktree status", showOut)
	}
	if !strings.Contains(showOut, ".aom") || !strings.Contains(showOut, "worktrees") {
		t.Fatalf("stdout = %q, want worktree path", showOut)
	}
	worktreePath := extractLineValue(showOut, "Worktree path: ")
	if worktreePath == "" {
		t.Fatalf("could not extract worktree path from %q", showOut)
	}
	artifactRoot := extractLineValue(showOut, "Artifact root: ")
	if artifactRoot == "" {
		t.Fatalf("stdout = %q, want artifact root", showOut)
	}
	if same, err := samePath(artifactRoot, filepath.Join(worktreePath, ".agent")); err != nil || !same {
		t.Fatalf("artifact root = %q, want %q (same=%t err=%v)", artifactRoot, filepath.Join(worktreePath, ".agent"), same, err)
	}
	taskLog := extractLineValue(showOut, "Task log: ")
	if taskLog == "" {
		t.Fatalf("stdout = %q, want task log path", showOut)
	}
	if same, err := samePath(filepath.Dir(taskLog), filepath.Join(worktreePath, ".agent")); err != nil || !same {
		t.Fatalf("task log dir = %q, want %q (same=%t err=%v)", filepath.Dir(taskLog), filepath.Join(worktreePath, ".agent"), same, err)
	}
	for _, name := range []string{"task.md", "state.md", "index.md", "log.md"} {
		if _, err := os.Stat(filepath.Join(worktreePath, ".agent", name)); err != nil {
			t.Fatalf("artifact %s missing in worktree .agent: %v", name, err)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, filepath.Join(worktreePath, ".agent")) {
		t.Fatalf("stdout = %q, want worktree artifact path summary", out)
	}
}

func TestExecuteSessionSpawnUsesProvisionedWorktreeWhenRepoIsGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree provisioning integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Spawn in provisioned worktree", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	spawnOut := stdout.String()
	if !strings.Contains(spawnOut, "Worktree status: Active") {
		t.Fatalf("stdout = %q, want active worktree status", spawnOut)
	}
	if !strings.Contains(spawnOut, ".aom") || !strings.Contains(spawnOut, "worktrees") {
		t.Fatalf("stdout = %q, want provisioned worktree path", spawnOut)
	}
	sessionID := extractSessionID(spawnOut)
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", spawnOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session show failed: %v", err)
	}
	showOut := stdout.String()
	if !strings.Contains(showOut, "Task: "+taskID) {
		t.Fatalf("stdout = %q, want task in session show", showOut)
	}
	if !strings.Contains(showOut, ".aom") || !strings.Contains(showOut, "worktrees") {
		t.Fatalf("stdout = %q, want provisioned worktree path", showOut)
	}
	worktreePath := extractLineValue(showOut, "Worktree: ")
	if worktreePath == "" {
		t.Fatalf("could not extract worktree path from %q", showOut)
	}
	indexData, err := os.ReadFile(filepath.Join(worktreePath, ".agent", "index.md"))
	if err != nil {
		t.Fatalf("ReadFile(index.md) failed: %v", err)
	}
	if !strings.Contains(string(indexData), "Active Session: "+sessionID) {
		t.Fatalf("index.md = %q, want active session in worktree artifact", string(indexData))
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "worktree=Active | branch=aom/") {
		t.Fatalf("stdout = %q, want active worktree summary", out)
	}
}

func TestExecuteStatusMarksStaleWorktreeNeedsRepair(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree repair integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Repair stale worktree", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	worktreePath := extractLineValue(stdout.String(), "Worktree path: ")
	if worktreePath == "" {
		t.Fatalf("could not extract worktree path from %q", stdout.String())
	}

	if err := os.RemoveAll(worktreePath); err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	statusOut := stdout.String()
	if !strings.Contains(statusOut, "worktree=NeedsRepair | branch=aom/") {
		t.Fatalf("stdout = %q, want worktree needs-repair summary", statusOut)
	}
	if !strings.Contains(statusOut, "repair=run \"aom worktree repair "+taskID+"\" to recreate the missing git worktree path before continuing") {
		t.Fatalf("stdout = %q, want missing-path repair hint", statusOut)
	}
	if !strings.Contains(statusOut, "next=recreate the missing task worktree before continuing") {
		t.Fatalf("stdout = %q, want missing-path next action", statusOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	showOut := stdout.String()
	if !strings.Contains(showOut, "Worktree hint: run \"aom worktree repair "+taskID+"\" to recreate the missing git worktree path before continuing") {
		t.Fatalf("stdout = %q, want missing-path task hint", showOut)
	}
	if !strings.Contains(showOut, "Recommended next action: recreate the missing task worktree before continuing") {
		t.Fatalf("stdout = %q, want missing-path task next action", showOut)
	}
}

func TestExecuteWorktreeRepairRestoresMissingGitWorktreeAndArtifacts(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree repair integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Repair command smoke", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	worktreePath := extractLineValue(stdout.String(), "Worktree path: ")
	if worktreePath == "" {
		t.Fatalf("could not extract worktree path from %q", stdout.String())
	}

	if err := os.RemoveAll(worktreePath); err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"worktree", "repair", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("worktree repair failed: %v", err)
	}
	repairOut := stdout.String()
	if !strings.Contains(repairOut, "Worktree repaired") {
		t.Fatalf("stdout = %q, want repair confirmation", repairOut)
	}
	if !strings.Contains(repairOut, "Status: Ready") {
		t.Fatalf("stdout = %q, want ready status after repair", repairOut)
	}

	for _, name := range []string{"task.md", "state.md", "index.md", "log.md"} {
		if _, err := os.Stat(filepath.Join(worktreePath, ".agent", name)); err != nil {
			t.Fatalf("artifact %s missing after repair: %v", name, err)
		}
	}

	logData, err := os.ReadFile(filepath.Join(worktreePath, ".agent", "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "worktree.repaired") {
		t.Fatalf("log.md = %q, want worktree.repaired event", string(logData))
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	showOut := stdout.String()
	if !strings.Contains(showOut, "Worktree status: Ready") {
		t.Fatalf("stdout = %q, want ready worktree status", showOut)
	}
}

func TestExecuteStatusReconcilesDetachedSessionAndDowngradesWorktreeToReady(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for session/worktree reconciliation integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				return nil, errors.New("pane not found")
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Reconcile missing pane", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	statusOut := stdout.String()
	if !strings.Contains(statusOut, "status=Detached") {
		t.Fatalf("stdout = %q, want detached session status", statusOut)
	}
	if !strings.Contains(statusOut, "worktree=Ready | branch=aom/") {
		t.Fatalf("stdout = %q, want worktree downgraded to Ready", statusOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: Detached") {
		t.Fatalf("stdout = %q, want detached status in session show", out)
	}
}

func TestExecuteSessionShowByAgentNameReconcilesDetachedSession(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				return nil, errors.New("pane not found")
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("session show by agent failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: Detached") {
		t.Fatalf("stdout = %q, want detached status in agent-name session show", out)
	}
}

func TestExecuteSessionStopMarksStoppedAndDowngradesWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for session stop integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
			case "kill-pane":
				return nil, nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Stop live session", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "stop", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session stop failed: %v", err)
	}
	stopOut := stdout.String()
	if !strings.Contains(stopOut, "Session stopped") {
		t.Fatalf("stdout = %q, want stop confirmation", stopOut)
	}
	if !strings.Contains(stopOut, "Status: Stopped") {
		t.Fatalf("stdout = %q, want stopped status", stopOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: Stopped") {
		t.Fatalf("stdout = %q, want stopped status in session show", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	statusOut := stdout.String()
	if !strings.Contains(statusOut, "status=Stopped") {
		t.Fatalf("stdout = %q, want stopped session in status", statusOut)
	}
	if !strings.Contains(statusOut, "worktree=Ready | branch=aom/") {
		t.Fatalf("stdout = %q, want worktree downgraded to Ready", statusOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	worktreePath := extractLineValue(stdout.String(), "Worktree path: ")
	if worktreePath == "" {
		t.Fatalf("could not extract worktree path from %q", stdout.String())
	}
	logData, err := os.ReadFile(filepath.Join(worktreePath, ".agent", "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "session.stopped") {
		t.Fatalf("log.md = %q, want session.stopped event", string(logData))
	}
}

func TestExecuteSessionStopMarksStoppedWhenKillPaneFails(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for session stop integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
			case "kill-pane":
				return nil, errors.New("pane busy")
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Stop despite pane cleanup failure", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "stop", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("session stop failed: %v", err)
	}
	stopOut := stdout.String()
	if !strings.Contains(stopOut, "Session stopped") {
		t.Fatalf("stdout = %q, want stop confirmation", stopOut)
	}
	if !strings.Contains(stopOut, "Status: Stopped") {
		t.Fatalf("stdout = %q, want stopped status", stopOut)
	}
	if !strings.Contains(stopOut, "Warning: tmux pane cleanup failed") {
		t.Fatalf("stdout = %q, want pane cleanup warning", stopOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	statusOut := stdout.String()
	if !strings.Contains(statusOut, "status=Stopped") {
		t.Fatalf("stdout = %q, want stopped session in status", statusOut)
	}
	if !strings.Contains(statusOut, "worktree=Ready | branch=aom/") {
		t.Fatalf("stdout = %q, want worktree downgraded to Ready", statusOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	worktreePath := extractLineValue(stdout.String(), "Worktree path: ")
	if worktreePath == "" {
		t.Fatalf("could not extract worktree path from %q", stdout.String())
	}
	logData, err := os.ReadFile(filepath.Join(worktreePath, ".agent", "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "tmux cleanup warning") {
		t.Fatalf("log.md = %q, want pane cleanup warning captured in canonical log", string(logData))
	}
}

func TestExecuteSessionArchiveMarksStoppedSessionArchived(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			if len(args) == 0 {
				return nil, nil
			}
			switch args[0] {
			case "has-session":
				return nil, errors.New("session not found")
			case "new-session":
				return nil, nil
			case "split-window":
				return []byte("@1 %5\n"), nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())
	if sessionID == "" {
		t.Fatalf("could not extract session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "stop", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session stop failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "archive", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session archive failed: %v", err)
	}
	archiveOut := stdout.String()
	if !strings.Contains(archiveOut, "Session archived") {
		t.Fatalf("stdout = %q, want archive confirmation", archiveOut)
	}
	if !strings.Contains(archiveOut, "Status: Archived") {
		t.Fatalf("stdout = %q, want archived status", archiveOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", sessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("session show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: Archived") {
		t.Fatalf("stdout = %q, want archived status in session show", out)
	}
}

func TestExecuteSessionReplaceWithRealRuntimeUsesCodexLaunchCommand(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for session replace integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	firstHasSession := true
	splitCount := 0
	var splitCommands []string
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				splitCount++
				splitCommands = append(splitCommands, args[len(args)-1])
				if splitCount == 1 {
					return []byte("@1 %5\n"), nil
				}
				return []byte("@1 %6\n"), nil
			case "set-option":
				return nil, nil
			case "display-message":
				target := args[len(args)-2]
				if target == "%5" {
					return []byte("%5\n"), nil
				}
				if target == "%6" {
					return []byte("%6\n"), nil
				}
				return nil, errors.New("pane not found")
			case "kill-pane":
				return nil, nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	restoreLaunchBuilder := stubLaunchBuilderFactory(t, aomruntime.NewBuilderWithLookPath(
		func(string) (string, error) { return "/opt/homebrew/bin/codex", nil },
	))
	defer restoreLaunchBuilder()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Replace with real runtime", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	oldSessionID := extractSessionID(stdout.String())
	if oldSessionID == "" {
		t.Fatalf("could not extract old session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "replace", oldSessionID, "--agent", "backend-main", "--reason", "switch to real runtime", "--real"}, &stdout, &stderr); err != nil {
		t.Fatalf("session replace failed: %v", err)
	}
	replaceOut := stdout.String()
	if !strings.Contains(replaceOut, "Session replaced") {
		t.Fatalf("stdout = %q, want replace confirmation", replaceOut)
	}
	if splitCount != 2 {
		t.Fatalf("splitCount = %d, want 2 pane launches", splitCount)
	}
	if !strings.Contains(splitCommands[len(splitCommands)-1], "exec codex --sandbox workspace-write -a never") {
		t.Fatalf("replacement split command = %q, want codex exec launch", splitCommands[len(splitCommands)-1])
	}
}

func TestExecuteSessionReplaceSupersedesOldSessionInSameTaskWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for session replace integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	firstHasSession := true
	splitCount := 0
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				splitCount++
				if splitCount == 1 {
					return []byte("@1 %5\n"), nil
				}
				return []byte("@1 %6\n"), nil
			case "set-option":
				return nil, nil
			case "display-message":
				target := args[len(args)-2]
				if target == "%5" {
					return []byte("%5\n"), nil
				}
				if target == "%6" {
					return []byte("%6\n"), nil
				}
				return nil, errors.New("pane not found")
			case "kill-pane":
				return nil, nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Replace live session", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	oldSessionID := extractSessionID(stdout.String())
	if oldSessionID == "" {
		t.Fatalf("could not extract old session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "replace", oldSessionID, "--agent", "reviewer-main", "--reason", "provider limit", "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session replace failed: %v", err)
	}
	replaceOut := stdout.String()
	if !strings.Contains(replaceOut, "Session replaced") {
		t.Fatalf("stdout = %q, want replace confirmation", replaceOut)
	}
	if !strings.Contains(replaceOut, "Old session result: stopped (Stopped)") {
		t.Fatalf("stdout = %q, want explicit old session outcome", replaceOut)
	}
	newSessionID := extractEntityID(replaceOut, "New session: ")
	if newSessionID == "" {
		t.Fatalf("could not extract new session id from %q", replaceOut)
	}
	if newSessionID == oldSessionID {
		t.Fatalf("new session id = old session id = %q", newSessionID)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", oldSessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("old session show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: Stopped") {
		t.Fatalf("stdout = %q, want stopped old session", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", newSessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("new session show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Agent: reviewer-main") || !strings.Contains(out, "Task: "+taskID) {
		t.Fatalf("stdout = %q, want reviewer replacement bound to same task", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	statusOut := stdout.String()
	if !strings.Contains(statusOut, "status=Stopped") {
		t.Fatalf("stdout = %q, want stopped old session in summary", statusOut)
	}
	if !strings.Contains(statusOut, newSessionID) {
		t.Fatalf("stdout = %q, want replacement session in summary", statusOut)
	}
	if !strings.Contains(statusOut, "worktree=Active | branch=aom/") {
		t.Fatalf("stdout = %q, want active worktree retained by replacement session", statusOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	worktreePath := extractLineValue(stdout.String(), "Worktree path: ")
	if worktreePath == "" {
		t.Fatalf("could not extract worktree path from %q", stdout.String())
	}
	logData, err := os.ReadFile(filepath.Join(worktreePath, ".agent", "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "session.replaced") {
		t.Fatalf("log.md = %q, want session.replaced event", string(logData))
	}
}

func TestExecuteSessionReplaceArchivesDetachedSession(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for session replace integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	firstHasSession := true
	splitCount := 0
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				splitCount++
				if splitCount == 1 {
					return []byte("@1 %5\n"), nil
				}
				return []byte("@1 %6\n"), nil
			case "set-option":
				return nil, nil
			case "display-message":
				target := args[len(args)-2]
				if target == "%5" {
					return nil, errors.New("pane not found")
				}
				if target == "%6" {
					return []byte("%6\n"), nil
				}
				return nil, errors.New("pane not found")
			case "kill-pane":
				t.Fatalf("kill-pane should not run for detached replacement")
				return nil, nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Replace detached session", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	oldSessionID := extractSessionID(stdout.String())
	if oldSessionID == "" {
		t.Fatalf("could not extract old session id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "replace", oldSessionID, "--agent", "reviewer-main", "--reason", "recover detached pane", "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session replace failed: %v", err)
	}
	replaceOut := stdout.String()
	if !strings.Contains(replaceOut, "Old session result: archived (Archived)") {
		t.Fatalf("stdout = %q, want archived old session outcome", replaceOut)
	}
	newSessionID := extractEntityID(replaceOut, "New session: ")
	if newSessionID == "" {
		t.Fatalf("could not extract new session id from %q", replaceOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", oldSessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("old session show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: Archived") {
		t.Fatalf("stdout = %q, want archived old session", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	statusOut := stdout.String()
	if !strings.Contains(statusOut, "status=Archived") {
		t.Fatalf("stdout = %q, want archived old session in summary", statusOut)
	}
	if !strings.Contains(statusOut, newSessionID) {
		t.Fatalf("stdout = %q, want replacement session in summary", statusOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	worktreePath := extractLineValue(stdout.String(), "Worktree path: ")
	if worktreePath == "" {
		t.Fatalf("could not extract worktree path from %q", stdout.String())
	}
	logData, err := os.ReadFile(filepath.Join(worktreePath, ".agent", "log.md"))
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "session.archived") {
		t.Fatalf("log.md = %q, want session.archived event", string(logData))
	}
	if !strings.Contains(string(logData), "session.replaced") {
		t.Fatalf("log.md = %q, want session.replaced event", string(logData))
	}
}

func TestExecuteSessionReplaceLeavesWorkingSessionRunning(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for session replace integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit("add", "README.md")
	runGit("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	firstHasSession := true
	var splitCount int
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				splitCount++
				if splitCount == 1 {
					return []byte("@1 %5\n"), nil
				}
				return []byte("@1 %6\n"), nil
			case "set-option":
				return nil, nil
			case "display-message":
				target := args[len(args)-2]
				if target == "%5" {
					return []byte("%5\n"), nil
				}
				if target == "%6" {
					return []byte("%6\n"), nil
				}
				return nil, errors.New("pane not found")
			case "kill-pane":
				return nil, nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Replace working session", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	oldSessionID := extractSessionID(stdout.String())
	if oldSessionID == "" {
		t.Fatalf("could not extract old session id from %q", stdout.String())
	}

	projectResult, err := project.NewService().Open(".")
	if err != nil {
		t.Fatalf("project open failed: %v", err)
	}
	sessionService, sqlDB, err := app.New().OpenSessionService(projectResult.DBPath)
	if err != nil {
		t.Fatalf("open session service failed: %v", err)
	}
	record, err := sessionService.Get(oldSessionID)
	if err != nil {
		sqlDB.Close()
		t.Fatalf("load session failed: %v", err)
	}
	if record == nil {
		sqlDB.Close()
		t.Fatalf("session %q not found", oldSessionID)
	}
	record.Status = "Working"
	if _, err := sessionService.Save(*record); err != nil {
		sqlDB.Close()
		t.Fatalf("save working session failed: %v", err)
	}
	sqlDB.Close()

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "replace", oldSessionID, "--agent", "reviewer-main", "--reason", "provider limit", "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session replace failed: %v", err)
	}
	replaceOut := stdout.String()
	if !strings.Contains(replaceOut, "Old session result: left running (Working requires operator intervention)") {
		t.Fatalf("stdout = %q, want explicit left-running outcome", replaceOut)
	}
	if !strings.Contains(replaceOut, "Action hint: run \"aom session stop ") {
		t.Fatalf("stdout = %q, want explicit action hint for left-running session", replaceOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "show", oldSessionID}, &stdout, &stderr); err != nil {
		t.Fatalf("old session show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: Working") {
		t.Fatalf("stdout = %q, want old working session to remain running", out)
	}
}

func TestExecuteTaskUpdateCloseAndStepUpdate(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Implement milestone 3", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	stdout.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	stepID := extractStepID(stdout.String())
	if stepID == "" {
		t.Fatalf("could not extract step id from %q", stdout.String())
	}

	stdout.Reset()
	if err := Execute([]string{"task", "update", taskID, "--mode", "bugfix", "--status", "ready"}, &stdout, &stderr); err != nil {
		t.Fatalf("task update failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Mode: Bugfix") || !strings.Contains(out, "Status: Ready") {
		t.Fatalf("stdout = %q, want updated task fields", out)
	}

	stdout.Reset()
	if err := Execute([]string{"step", "update", stepID, "--status", "confirmed"}, &stdout, &stderr); err != nil {
		t.Fatalf("step update to confirmed failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"step", "update", stepID, "--status", "ready"}, &stdout, &stderr); err != nil {
		t.Fatalf("step update to ready failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: Ready") {
		t.Fatalf("stdout = %q, want ready step", out)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "update", taskID, "--status", "in-progress"}, &stdout, &stderr); err != nil {
		t.Fatalf("task update to in-progress failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "close", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task close failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: Done") {
		t.Fatalf("stdout = %q, want Done status", out)
	}

	stdout.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "status=Done") {
		t.Fatalf("stdout = %q, want done task summary", out)
	}
	if !strings.Contains(out, "next=task is closed; archive later if needed") {
		t.Fatalf("stdout = %q, want closed task next action", out)
	}
	if !strings.Contains(out, "status=Ready") {
		t.Fatalf("stdout = %q, want ready step summary", out)
	}
}

func TestExecuteStatusHighlightsNeedsAttention(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Investigate failing provider", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	stdout.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	stepID := extractStepID(stdout.String())
	if stepID == "" {
		t.Fatalf("could not extract step id from %q", stdout.String())
	}

	stdout.Reset()
	if err := Execute([]string{"task", "update", taskID, "--status", "ready"}, &stdout, &stderr); err != nil {
		t.Fatalf("task update to ready failed: %v", err)
	}
	stdout.Reset()
	if err := Execute([]string{"task", "update", taskID, "--status", "in-progress"}, &stdout, &stderr); err != nil {
		t.Fatalf("task update to in-progress failed: %v", err)
	}
	stdout.Reset()
	if err := Execute([]string{"step", "update", stepID, "--status", "confirmed"}, &stdout, &stderr); err != nil {
		t.Fatalf("step update to confirmed failed: %v", err)
	}
	stdout.Reset()
	if err := Execute([]string{"step", "update", stepID, "--status", "ready"}, &stdout, &stderr); err != nil {
		t.Fatalf("step update to ready failed: %v", err)
	}
	stdout.Reset()
	if err := Execute([]string{"step", "update", stepID, "--status", "in-progress"}, &stdout, &stderr); err != nil {
		t.Fatalf("step update to in-progress failed: %v", err)
	}
	stdout.Reset()
	if err := Execute([]string{"step", "update", stepID, "--status", "needs-attention"}, &stdout, &stderr); err != nil {
		t.Fatalf("step update to needs-attention failed: %v", err)
	}
	stdout.Reset()
	if err := Execute([]string{"task", "update", taskID, "--status", "needs-attention"}, &stdout, &stderr); err != nil {
		t.Fatalf("task update to needs-attention failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "status=NeedsAttention") {
		t.Fatalf("stdout = %q, want needs-attention status", out)
	}
	if !strings.Contains(out, "next=operator review is needed before work continues") {
		t.Fatalf("stdout = %q, want operator review hint", out)
	}
}

func TestExecuteReviewPreparesNotesWithoutTmux(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "", errors.New("not found") },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Review wrapper task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"review", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("review failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Review prepared") {
		t.Fatalf("stdout = %q, want review prepared header", out)
	}
	if !strings.Contains(out, "Review step: STEP-") {
		t.Fatalf("stdout = %q, want review step id", out)
	}
	if !strings.Contains(out, "Reviewer agent: reviewer-main") {
		t.Fatalf("stdout = %q, want reviewer-main", out)
	}
	if !strings.Contains(out, "tmux is unavailable here") {
		t.Fatalf("stdout = %q, want tmux-unavailable next action", out)
	}

	reviewNotesPath := filepath.Join(taskArtifactDir(repoRoot, taskID), "review-notes.md")
	data, err := os.ReadFile(reviewNotesPath)
	if err != nil {
		t.Fatalf("ReadFile(review-notes.md) failed: %v", err)
	}
	if !strings.Contains(string(data), "Reviewer: reviewer-main") {
		t.Fatalf("review-notes.md = %q, want reviewer template", string(data))
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Unresolved review items: 0") {
		t.Fatalf("stdout = %q, want unresolved review count", out)
	} else if !strings.Contains(out, "Status: Ready") {
		t.Fatalf("stdout = %q, want task promoted to Ready after review preparation", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "type=review") || !strings.Contains(out, "role=reviewer") || !strings.Contains(out, "agent=reviewer-main") {
		t.Fatalf("stdout = %q, want explicit reviewer step", out)
	}
	if out := stdout.String(); !strings.Contains(out, "status=Ready") {
		t.Fatalf("stdout = %q, want ready review step", out)
	}
}

func TestExecuteReviewReusesExistingReviewStep(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "", errors.New("not found") },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Reuse review step task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	projectResult, err := app.New().Projects.Open(".")
	if err != nil {
		t.Fatalf("project open failed: %v", err)
	}
	stepService, sqlDB, err := app.New().OpenStepService(projectResult.DBPath)
	if err != nil {
		t.Fatalf("open step service failed: %v", err)
	}
	createdStep, err := stepService.Create(step.CreateParams{
		ProjectID: projectResult.Project.ID,
		TaskID:    taskID,
		StepType:  "review",
		Title:     "Review existing work",
		Status:    "confirmed",
		RoleName:  "backend",
		AgentName: "backend-main",
	})
	sqlDB.Close()
	if err != nil {
		t.Fatalf("create review step failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"review", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("review failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Review step: "+createdStep.ID) {
		t.Fatalf("stdout = %q, want reused review step id", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	out := stdout.String()
	if strings.Count(out, "type=review") != 1 {
		t.Fatalf("stdout = %q, want one review step", out)
	}
	if !strings.Contains(out, createdStep.ID) || !strings.Contains(out, "status=Ready") || !strings.Contains(out, "role=reviewer") || !strings.Contains(out, "agent=reviewer-main") {
		t.Fatalf("stdout = %q, want retargeted ready reviewer step", out)
	}
}

func TestExecuteReviewReusesExistingReviewerSession(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	splitCount := 0
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
				splitCount++
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
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Reuse reviewer session task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"review", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("first review failed: %v", err)
	}
	firstOut := stdout.String()
	if !strings.Contains(firstOut, "Session spawned: ") {
		t.Fatalf("stdout = %q, want spawned reviewer session", firstOut)
	}
	sessionID := extractEntityID(firstOut, "Session spawned: ")
	if sessionID == "" {
		t.Fatalf("could not extract spawned session id from %q", firstOut)
	}
	if splitCount != 1 {
		t.Fatalf("splitCount = %d, want 1 after first review", splitCount)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"review", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("second review failed: %v", err)
	}
	secondOut := stdout.String()
	if !strings.Contains(secondOut, "Session reused: "+sessionID) {
		t.Fatalf("stdout = %q, want reused reviewer session", secondOut)
	}
	if splitCount != 1 {
		t.Fatalf("splitCount = %d, want no additional pane creation", splitCount)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: InProgress") {
		t.Fatalf("stdout = %q, want task promoted to InProgress once reviewer session is live", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "type=review") || !strings.Contains(out, "status=InProgress") {
		t.Fatalf("stdout = %q, want live review step in progress", out)
	}
}

func TestExecuteReviewFindingsMoveTaskAndReviewStepToNeedsAttention(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Review findings transition task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"review", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("first review failed: %v", err)
	}
	reviewStepID := extractEntityID(stdout.String(), "Review step: ")
	if reviewStepID == "" {
		t.Fatalf("could not extract review step id from %q", stdout.String())
	}

	reviewNotesPath := filepath.Join(taskArtifactDir(repoRoot, taskID), "review-notes.md")
	reviewContent := `# Review Notes

## Summary
- Status: Needs fixes

## Items

### RVW-001
- Severity: high
- Path: internal/auth/handler.go
- Issue: inconsistent error payload
- Expected Fix: use shared envelope helper
- Status: open
- Owner: backend
`
	if err := os.WriteFile(reviewNotesPath, []byte(reviewContent), 0o644); err != nil {
		t.Fatalf("WriteFile(review-notes.md) failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"review", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("second review failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Review findings detected: 1") {
		t.Fatalf("stdout = %q, want findings detection summary", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Status: NeedsAttention") {
		t.Fatalf("stdout = %q, want task moved to NeedsAttention", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, reviewStepID) || !strings.Contains(out, "status=NeedsAttention") {
		t.Fatalf("stdout = %q, want review step moved to NeedsAttention", out)
	}
}

func TestExecuteReviewFindingsResetPreferredOwnerToSharedFindingOwner(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Review owner hint task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	stdout.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())

	stdout.Reset()
	if err := Execute([]string{"handoff", sessionID, "--to", "reviewer-main", "--reason", "ready for review"}, &stdout, &stderr); err != nil {
		t.Fatalf("handoff failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"review", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("first review failed: %v", err)
	}
	stdout.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	followupStepID := extractStepID(stdout.String())

	reviewNotesPath := filepath.Join(taskArtifactDir(repoRoot, taskID), "review-notes.md")
	reviewContent := `# Review Notes

## Summary
- Status: Needs fixes

## Items

### RVW-001
- Severity: high
- Path: internal/auth/handler.go
- Issue: inconsistent error payload
- Expected Fix: use shared envelope helper
- Status: open
- Owner: backend
`
	if err := os.WriteFile(reviewNotesPath, []byte(reviewContent), 0o644); err != nil {
		t.Fatalf("WriteFile(review-notes.md) failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"review", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("second review failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Preferred role: backend") || !strings.Contains(out, "Preferred agent: backend-main") {
		t.Fatalf("stdout = %q, want preferred owner reset to backend-main auto-pick hint", out)
	}

	stdout.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, followupStepID) || !strings.Contains(out, "role=backend") || !strings.Contains(out, "agent=backend-main") {
		t.Fatalf("stdout = %q, want follow-up step owner hint reset to backend-main", out)
	}
}

func TestExecuteStatusSurfacesUnresolvedReviewItems(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "", errors.New("not found") },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Review findings task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	reviewNotesPath := filepath.Join(taskArtifactDir(repoRoot, taskID), "review-notes.md")
	reviewContent := `# Review Notes

## Summary
- Status: Needs fixes

## Items

### RVW-001
- Severity: high
- Path: internal/auth/handler.go
- Issue: inconsistent error payload
- Expected Fix: use shared envelope helper
- Status: open
- Owner: backend
`
	if err := os.WriteFile(reviewNotesPath, []byte(reviewContent), 0o644); err != nil {
		t.Fatalf("WriteFile(review-notes.md) failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "reviews=open:1") {
		t.Fatalf("stdout = %q, want unresolved review count in status", out)
	}
	if !strings.Contains(out, "review-owner=backend-main (backend)") {
		t.Fatalf("stdout = %q, want backend-main auto-picked review owner hint", out)
	}
	if !strings.Contains(out, "next=address unresolved review items and route follow-up work to backend-main (backend)") {
		t.Fatalf("stdout = %q, want review-driven next action with auto-picked owner hint", out)
	}
}

func TestExecuteStatusHighlightsMixedReviewOwners(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "", errors.New("not found") },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Mixed owner review task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	reviewNotesPath := filepath.Join(taskArtifactDir(repoRoot, taskID), "review-notes.md")
	reviewContent := `# Review Notes

## Summary
- Status: Needs fixes

## Items

### RVW-001
- Status: open
- Owner: backend

### RVW-002
- Status: open
- Owner: qa
`
	if err := os.WriteFile(reviewNotesPath, []byte(reviewContent), 0o644); err != nil {
		t.Fatalf("WriteFile(review-notes.md) failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "review-owner=mixed owners - operator must choose") {
		t.Fatalf("stdout = %q, want mixed review owner hint", out)
	}
	if !strings.Contains(out, "next=review findings have mixed owners; operator must choose the follow-up owner before continuing") {
		t.Fatalf("stdout = %q, want mixed-owner next action", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Review owner hint: mixed owners - operator must choose") {
		t.Fatalf("stdout = %q, want task show mixed-owner hint", out)
	}
}

func TestExecuteOpenFailsClearlyWhenTmuxIsUnavailable(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "", errors.New("not found") },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	err = Execute([]string{"open"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("open should fail when tmux is unavailable")
	}
	if !strings.Contains(err.Error(), "ensure tmux workspace: tmux is not available in the current environment") {
		t.Fatalf("error = %q, want missing tmux message", err)
	}
}

// TestCaptureAllNoSessions verifies that --all prints a friendly "no sessions"
// message when the project has no sessions with live panes.
func TestCaptureAllNoSessions(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWD) }()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout, stderr bytes.Buffer
	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init: %v", err)
	}
	stdout.Reset()

	if err := Execute([]string{"capture", "--all"}, &stdout, &stderr); err != nil {
		t.Fatalf("capture --all: %v", err)
	}
	if !strings.Contains(stdout.String(), "No active sessions") {
		t.Fatalf("stdout = %q, want 'No active sessions'", stdout.String())
	}
}

// TestCaptureAllMutuallyExclusiveWithSessionID verifies that passing both
// --all and a positional session ID returns an error.
func TestCaptureAllMutuallyExclusiveWithSessionID(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWD) }()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout, stderr bytes.Buffer
	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init: %v", err)
	}

	err := Execute([]string{"capture", "--all", "SESS-001"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("capture --all <session-id> should return an error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %q, want 'mutually exclusive'", err.Error())
	}
}

// TestCaptureAllWithActiveSessions verifies that --all prints a section header
// per active session and the captured pane content beneath it.
func TestCaptureAllWithActiveSessions(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWD) }()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	const paneID = "%5"
	const paneContent = "status=InProgress\ntask=t-001\n"
	firstHasSession := true

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
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
				return []byte("@1 " + paneID + "\n"), nil
			case "set-option":
				return nil, nil
			case "display-message":
				// PaneExists: return the pane ID so the check passes.
				return []byte(paneID + "\n"), nil
			case "capture-pane":
				return []byte(paneContent), nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout, stderr bytes.Buffer
	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init: %v", err)
	}
	stdout.Reset()

	// Spawn a session so the DB has a record with a live pane.
	if err := Execute([]string{"session", "spawn", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn: %v", err)
	}
	stdout.Reset()

	if err := Execute([]string{"capture", "--all"}, &stdout, &stderr); err != nil {
		t.Fatalf("capture --all: %v", err)
	}
	out := stdout.String()

	// Expect a section header containing the agent name.
	if !strings.Contains(out, "backend-main") {
		t.Fatalf("stdout = %q, want 'backend-main' header", out)
	}
	// Expect the pane content to be printed.
	if !strings.Contains(out, "status=InProgress") {
		t.Fatalf("stdout = %q, want pane content 'status=InProgress'", out)
	}
	// Expect the summary count line at the end.
	if !strings.Contains(out, "session(s) captured") {
		t.Fatalf("stdout = %q, want 'session(s) captured' summary line", out)
	}
}

// TestCaptureAllSummaryMode verifies that --all --summary applies the
// signal-filter to each session's output.
func TestCaptureAllSummaryMode(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWD) }()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	const paneID = "%5"
	// Include one signal line and one noise line to verify filtering.
	const paneContent = "random prose that should be filtered\nstatus=Done\n"
	firstHasSession := true

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
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
				return []byte("@1 " + paneID + "\n"), nil
			case "set-option":
				return nil, nil
			case "display-message":
				return []byte(paneID + "\n"), nil
			case "capture-pane":
				return []byte(paneContent), nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout, stderr bytes.Buffer
	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init: %v", err)
	}
	stdout.Reset()

	if err := Execute([]string{"session", "spawn", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn: %v", err)
	}
	stdout.Reset()

	if err := Execute([]string{"capture", "--all", "--summary"}, &stdout, &stderr); err != nil {
		t.Fatalf("capture --all --summary: %v", err)
	}
	out := stdout.String()

	// Signal line should be present.
	if !strings.Contains(out, "status=Done") {
		t.Fatalf("stdout = %q, want 'status=Done' (signal line)", out)
	}
	// Noise line should be filtered out.
	if strings.Contains(out, "random prose") {
		t.Fatalf("stdout = %q, 'random prose' should be filtered by --summary", out)
	}
}

func stubAppFactory(t *testing.T, manager *tmux.Manager) func() {
	t.Helper()

	original := newApp
	newApp = func() *app.App {
		return &app.App{
			Planner:  app.New().Planner,
			Projects: project.NewService(),
			Tmux:     manager,
		}
	}

	return func() {
		newApp = original
	}
}

func stubLaunchBuilderFactory(t *testing.T, builder *aomruntime.Builder) func() {
	t.Helper()

	original := newLaunchBuilder
	newLaunchBuilder = func() *aomruntime.Builder {
		return builder
	}

	return func() {
		newLaunchBuilder = original
	}
}

func extractSessionID(output string) string {
	return extractEntityID(output, "Session: ")
}

func extractEntityID(output, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, prefix))
	}

	return ""
}

func extractLineValue(output, prefix string) string {
	return extractEntityID(output, prefix)
}

func extractStepID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// Support both old "  - STEP-xxx | ..." and new "STEP-xxx | ..." formats.
		line = strings.TrimPrefix(line, "- ")
		if !strings.HasPrefix(line, "STEP-") {
			continue
		}
		parts := strings.SplitN(line, " | ", 2)
		if len(parts) == 0 {
			continue
		}
		return strings.TrimSpace(parts[0])
	}

	return ""
}

func TestExecutePlanShowsRecommendation(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"plan", "fix login bug"}, &stdout, &stderr); err != nil {
		t.Fatalf("plan failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Plan") {
		t.Fatalf("stdout = %q, want Plan header", out)
	}
	if !strings.Contains(out, "Mode: Bugfix") {
		t.Fatalf("stdout = %q, want Bugfix mode", out)
	}
	if !strings.Contains(out, "Recommended agent: backend-main") {
		t.Fatalf("stdout = %q, want backend-main recommendation", out)
	}
	if !strings.Contains(out, "Proposed steps:") {
		t.Fatalf("stdout = %q, want proposed steps", out)
	}
}

func TestExecutePlanCreatePersistsTaskAndSteps(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"plan", "fix login bug", "--create"}, &stdout, &stderr); err != nil {
		t.Fatalf("plan --create failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Task created from plan") {
		t.Fatalf("stdout = %q, want created-from-plan summary", out)
	}
	taskID := extractEntityID(out, "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", out)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	showOut := stdout.String()
	if !strings.Contains(showOut, "Mode: Bugfix") {
		t.Fatalf("stdout = %q, want Bugfix mode", showOut)
	}
	if !strings.Contains(showOut, "Preferred agent: backend-main") {
		t.Fatalf("stdout = %q, want backend-main ownership", showOut)
	}
	if !strings.Contains(showOut, "Worktree status: Ready") {
		t.Fatalf("stdout = %q, want ready worktree status", showOut)
	}

	stdout.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	stepOut := stdout.String()
	if !strings.Contains(stepOut, "type=research") {
		t.Fatalf("stdout = %q, want research step", stepOut)
	}
	if !strings.Contains(stepOut, "type=implementation") {
		t.Fatalf("stdout = %q, want implementation step", stepOut)
	}
	if !strings.Contains(stepOut, "dependencies=STEP-") {
		t.Fatalf("stdout = %q, want sequential dependency", stepOut)
	}

	artifactDir := taskArtifactDir(repoRoot, taskID)
	if _, err := os.Stat(filepath.Join(artifactDir, "log.md")); err != nil {
		t.Fatalf("plan artifact log missing: %v", err)
	}
}

func TestExecutePlanCreateFailsBeforePersistingTaskOnEmptyGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for empty repo integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	// project init now calls ensureGitReady which creates the initial commit,
	// so plan --create should succeed on an empty git repo.
	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"plan", "fix login bug", "--create"}, &stdout, &stderr); err != nil {
		t.Fatalf("plan --create failed: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if strings.Contains(stdout.String(), "Tasks: 0") {
		t.Fatalf("stdout = %q, want at least one task after plan --create", stdout.String())
	}
}

func TestExecuteTaskCreateFailsBeforePersistingTaskOnEmptyGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for empty repo integration test")
	}

	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	runGit("init", "-b", "main")

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	// project init now calls ensureGitReady which creates the initial commit,
	// so task create should succeed on an empty git repo.
	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "create", "Implement initial task"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if strings.Contains(stdout.String(), "Tasks: 0") {
		t.Fatalf("stdout = %q, want at least one task after task create", stdout.String())
	}
}

// taskArtifactDir returns the artifact directory for a task. When the repo has
// git initialized (normal flow after project init), artifacts are stored in the
// worktree .agent directory. Falls back to .aom/tasks/<taskID> for legacy or
// non-git scenarios.
func taskArtifactDir(repoRoot, taskID string) string {
	worktreePrefix := strings.ToLower(taskID)
	worktreesDir := filepath.Join(repoRoot, ".aom", "worktrees")
	if entries, err := os.ReadDir(worktreesDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), worktreePrefix) {
				continue
			}
			agentDir := filepath.Join(worktreesDir, e.Name(), ".agent")
			if _, err := os.Stat(agentDir); err == nil {
				return agentDir
			}
		}
	}
	return filepath.Join(repoRoot, ".aom", "tasks", taskID)
}

func samePath(left, right string) (bool, error) {
	leftEval, err := filepath.EvalSymlinks(left)
	if err != nil {
		return false, err
	}
	rightEval, err := filepath.EvalSymlinks(right)
	if err != nil {
		return false, err
	}

	return filepath.Clean(leftEval) == filepath.Clean(rightEval), nil
}

func TestWorktreeHintClassifiesUnregisteredPaths(t *testing.T) {
	mapping := &worktree.Record{Status: worktree.StatusNeedsRepair}

	artifactOnly := worktreeHint("TASK-001", mapping, worktree.DriftUnregisteredArtifactOnlyPath)
	if !strings.Contains(artifactOnly, `run "aom worktree repair TASK-001"`) || !strings.Contains(artifactOnly, "only contains AOM-owned content") {
		t.Fatalf("artifactOnly hint = %q, want repair-now guidance", artifactOnly)
	}

	dirty := worktreeHint("TASK-001", mapping, worktree.DriftUnregisteredDirtyPath)
	if !strings.Contains(dirty, "clean up non-artifact content manually") {
		t.Fatalf("dirty hint = %q, want manual cleanup guidance", dirty)
	}
}

func TestRecommendTaskActionClassifiesUnregisteredPaths(t *testing.T) {
	mapping := &worktree.Record{Status: worktree.StatusNeedsRepair}

	artifactOnly := recommendTaskAction("Ready", nil, mapping, worktree.DriftUnregisteredArtifactOnlyPath, 0, "", false)
	if artifactOnly != "run worktree repair to recreate the unregistered task worktree" {
		t.Fatalf("artifactOnly next action = %q", artifactOnly)
	}

	dirty := recommendTaskAction("Ready", nil, mapping, worktree.DriftUnregisteredDirtyPath, 0, "", false)
	if dirty != "inspect the existing task worktree path and clean it up manually before continuing" {
		t.Fatalf("dirty next action = %q", dirty)
	}
}

func TestBriefSummaryTruncatesToFirstLine(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"single line brief", "single line brief"},
		{"first line\nsecond line\nthird line", "first line"},
		{"\n\nfirst non-empty line\nsecond", "first non-empty line"},
		{strings.Repeat("x", 250), strings.Repeat("x", 200) + "..."},
		{"", ""},
	}
	for _, tc := range cases {
		got := briefSummary(tc.input)
		if got != tc.want {
			t.Errorf("briefSummary(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestInterpretEscapesConvertsNewlinesAndTabs(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`line1\nline2`, "line1\nline2"},
		{`col1\tcol2`, "col1\tcol2"},
		{`### header\n- bullet\n- bullet2`, "### header\n- bullet\n- bullet2"},
		{"no escapes here", "no escapes here"},
		{`mixed \n and \t`, "mixed \n and \t"},
	}
	for _, tc := range cases {
		got := interpretEscapes(tc.input)
		if got != tc.want {
			t.Errorf("interpretEscapes(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestDoctorReportsNoProjectWhenAOMDirMissing(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	_ = Execute([]string{"doctor"}, &stdout, &stderr)
	out := stdout.String()

	if !strings.Contains(out, "AOM Doctor") {
		t.Fatalf("stdout = %q, want AOM Doctor header", out)
	}
	if !strings.Contains(out, ".aom/ directory not found") {
		t.Fatalf("stdout = %q, want missing .aom/ message", out)
	}
}

func TestDoctorPassesOnInitializedProject(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "test-proj", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	_ = Execute([]string{"doctor"}, &stdout, &stderr)
	out := stdout.String()

	if !strings.Contains(out, "AOM Doctor") {
		t.Fatalf("stdout = %q, want AOM Doctor header", out)
	}
	if !strings.Contains(out, "[PASS]") {
		t.Fatalf("stdout = %q, want at least one PASS", out)
	}
	if !strings.Contains(out, "project config") {
		t.Fatalf("stdout = %q, want project config check", out)
	}
	if !strings.Contains(out, "Summary:") {
		t.Fatalf("stdout = %q, want Summary line", out)
	}
}

func TestRuntimeListRequiresProject(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err = Execute([]string{"runtime", "list"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error without project, got nil")
	}
}

func TestRuntimeListShowsConfiguredRuntimes(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "test-proj", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"runtime", "list"}, &stdout, &stderr); err != nil {
		t.Fatalf("runtime list failed: %v", err)
	}
	out := stdout.String()

	if !strings.Contains(out, "Configured runtimes") {
		t.Fatalf("stdout = %q, want Configured runtimes header", out)
	}
	if !strings.Contains(out, "RUNTIME") {
		t.Fatalf("stdout = %q, want RUNTIME column", out)
	}
	if !strings.Contains(out, "claude") {
		t.Fatalf("stdout = %q, want claude runtime entry", out)
	}
}

func TestRuntimeInspectShowsResumeSupport(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "test-proj", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"runtime", "inspect", "claude"}, &stdout, &stderr); err != nil {
		t.Fatalf("runtime inspect failed: %v", err)
	}
	out := stdout.String()

	if !strings.Contains(out, "Runtime: claude") {
		t.Fatalf("stdout = %q, want Runtime header", out)
	}
	if !strings.Contains(out, "Resume:") {
		t.Fatalf("stdout = %q, want Resume field", out)
	}
	if !strings.Contains(out, "true") {
		t.Fatalf("stdout = %q, want resume=true for claude", out)
	}
}

func TestRuntimeInspectRequiresRuntimeName(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err = Execute([]string{"runtime", "inspect"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error without runtime name, got nil")
	}
}

func TestExecuteReviewCloseTransitionsTaskToInProgress(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "", errors.New("not found") },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Close review task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")
	if taskID == "" {
		t.Fatalf("could not extract task id from %q", stdout.String())
	}

	// Prepare a review step (tmux unavailable so it stops after notes creation).
	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"review", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("review failed: %v", err)
	}

	// Extract the review step id.
	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	stepOut := stdout.String()
	reviewStepID := ""
	for _, line := range strings.Split(stepOut, "\n") {
		if strings.Contains(line, "type=review") {
			for _, part := range strings.Fields(line) {
				if strings.HasPrefix(part, "STEP-") {
					reviewStepID = part
				}
			}
		}
	}
	if reviewStepID == "" {
		t.Fatalf("could not find review step in %q", stepOut)
	}

	// Close the review.
	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"review", "close", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("review close failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Review closed") {
		t.Fatalf("stdout = %q, want review closed header", out)
	}
	if !strings.Contains(out, "Task: "+taskID) {
		t.Fatalf("stdout = %q, want task id", out)
	}
	if !strings.Contains(out, "Task status: in-progress") {
		t.Fatalf("stdout = %q, want in-progress status", out)
	}

	// Verify the task is now InProgress.
	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"task", "show", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
	if showOut := stdout.String(); !strings.Contains(showOut, "Status: InProgress") {
		t.Fatalf("task show stdout = %q, want InProgress", showOut)
	}

	// Verify the review step is Completed.
	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	if stepListOut := stdout.String(); !strings.Contains(stepListOut, "status=Completed") {
		t.Fatalf("step list stdout = %q, want Completed review step", stepListOut)
	}

	// Verify the log records a review.closed event.
	logPath := filepath.Join(taskArtifactDir(repoRoot, taskID), "log.md")
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "review.closed") {
		t.Fatalf("log.md = %q, want review.closed event", string(logData))
	}
}

func TestExecuteSessionSendUsesAOMActorEnvVar(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
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
			case "set-option", "send-keys":
				return nil, nil
			case "display-message":
				return []byte("%5\n"), nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Actor env var task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	stdout.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--task", taskID, "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())

	// Set AOM_ACTOR and send a prompt.
	t.Setenv("AOM_ACTOR", "orchestrator-ai")

	stdout.Reset()
	stderr.Reset()
	if err := Execute([]string{"session", "send", sessionID, "start work"}, &stdout, &stderr); err != nil {
		t.Fatalf("session send failed: %v", err)
	}

	logPath := filepath.Join(taskArtifactDir(repoRoot, taskID), "log.md")
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(log.md) failed: %v", err)
	}
	if !strings.Contains(string(logData), "orchestrator-ai") {
		t.Fatalf("log.md = %q, want orchestrator-ai actor", string(logData))
	}
}

func TestExecuteSessionRebindRejectsNonDetached(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	firstHasSession := true
	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			if len(args) == 0 {
				return nil, nil
			}
			switch args[0] {
			case "has-session":
				if firstHasSession {
					firstHasSession = false
					return nil, errors.New("not found")
				}
				return nil, nil
			case "new-session":
				return nil, nil
			case "split-window":
				return []byte("@1 %5\n"), nil
			case "set-option":
				return nil, nil
			case "display-message":
				// Return the pane ID so PaneExists returns true and the
				// session stays Idle during loadSessionByIdentifier reconciliation.
				return []byte("%5\n"), nil
			default:
				return nil, nil
			}
		},
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"session", "spawn", "backend-main", "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("session spawn failed: %v", err)
	}
	sessionID := extractSessionID(stdout.String())

	// Session is Idle, not Detached — rebind must reject it.
	rebindErr := Execute([]string{"session", "rebind", sessionID}, &stdout, &stderr)
	if rebindErr == nil {
		t.Fatal("expected error rebinding non-Detached session, got nil")
	}
	if !strings.Contains(rebindErr.Error(), "rebind only applies to Detached") {
		t.Fatalf("error = %q, want detached message", rebindErr.Error())
	}
}
