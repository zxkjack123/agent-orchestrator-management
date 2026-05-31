package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadGoalFile(t *testing.T) {
	dir := t.TempDir()
	aomDir := filepath.Join(dir, ".aom")
	if err := os.MkdirAll(aomDir, 0o755); err != nil {
		t.Fatal(err)
	}

	const text = "Build an authentication service with OAuth2 support."
	path, err := WriteGoalFile(dir, text)
	if err != nil {
		t.Fatalf("WriteGoalFile: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}

	rec, err := ReadGoalFile(dir)
	if err != nil {
		t.Fatalf("ReadGoalFile: %v", err)
	}
	if rec.Text != text {
		t.Errorf("text: got %q, want %q", rec.Text, text)
	}
	if rec.Status != "open" {
		t.Errorf("status: got %q, want %q", rec.Status, "open")
	}
	if rec.SetAt.IsZero() {
		t.Error("expected non-zero SetAt")
	}
}

func TestReadGoalFileNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadGoalFile(dir)
	if err == nil {
		t.Fatal("expected error when no goal file exists")
	}
}

func TestCompleteGoalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".aom"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := WriteGoalFile(dir, "some goal"); err != nil {
		t.Fatal(err)
	}

	if err := CompleteGoalFile(dir); err != nil {
		t.Fatalf("CompleteGoalFile: %v", err)
	}

	rec, err := ReadGoalFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "complete" {
		t.Errorf("status: got %q, want %q", rec.Status, "complete")
	}
	if rec.Text != "some goal" {
		t.Errorf("text should be preserved, got %q", rec.Text)
	}
}

func TestGoalPathIsInsideAOM(t *testing.T) {
	path := GoalPath("/project/root")
	expected := "/project/root/.aom/goal.json"
	if path != expected {
		t.Errorf("GoalPath: got %q, want %q", path, expected)
	}
}
