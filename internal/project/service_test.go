package project

import (
	"path/filepath"
	"testing"
)

func TestServiceOpenSyncsConfigToDB(t *testing.T) {
	repoRoot := t.TempDir()

	service := NewService()
	if _, err := service.Init(InitParams{
		Name:     "my-app",
		RepoPath: repoRoot,
	}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	result, err := service.Open(repoRoot)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if result.Project.Name != "my-app" {
		t.Fatalf("project name = %q, want %q", result.Project.Name, "my-app")
	}
	if len(result.Agents) != 3 {
		t.Fatalf("agent count = %d, want 3", len(result.Agents))
	}
	if result.DBPath != filepath.Join(repoRoot, ".aom", "sessions.db") {
		t.Fatalf("db path = %q, want %q", result.DBPath, filepath.Join(repoRoot, ".aom", "sessions.db"))
	}
	if result.TerminalDriver != "tmux" {
		t.Fatalf("terminal driver = %q, want %q", result.TerminalDriver, "tmux")
	}
	if result.SessionPrefix != "my-app" {
		t.Fatalf("session prefix = %q, want %q", result.SessionPrefix, "my-app")
	}
}
