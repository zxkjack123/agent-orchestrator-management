package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/app"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/project"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/tmux"
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

func stubAppFactory(t *testing.T, manager *tmux.Manager) func() {
	t.Helper()

	original := newApp
	newApp = func() *app.App {
		return &app.App{
			Projects: project.NewService(),
			Tmux:     manager,
		}
	}

	return func() {
		newApp = original
	}
}

func extractSessionID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "Session: ") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "Session: "))
	}

	return ""
}
