package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
)

// TestMessageWatchExitsOnNewMessage verifies that executeMessageWatch returns
// as soon as a new message is appended to the mailbox — not at timeout.
func TestMessageWatchExitsOnNewMessage(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Minimal AOM project skeleton so FindProjectRoot succeeds.
	aomDir := filepath.Join(dir, ".aom")
	if err := os.MkdirAll(aomDir, 0o755); err != nil {
		t.Fatalf("mkdir .aom: %v", err)
	}
	projectYAML := `name: watch-test
repo: .
default_branch: main
runtime:
  terminal: tmux
  session_prefix: wt
context:
  state_dir: .aom/state
`
	if err := os.WriteFile(filepath.Join(aomDir, "project.yaml"), []byte(projectYAML), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}

	// Pre-populate mailbox with existing content (watch must skip this).
	mailboxDir := filepath.Join(aomDir, "mailbox")
	if err := os.MkdirAll(mailboxDir, 0o755); err != nil {
		t.Fatalf("mkdir mailbox: %v", err)
	}
	mailboxPath := filepath.Join(mailboxDir, "agent-a.md")
	existingContent := "# Mailbox: agent-a\n\n## Messages\n\n"
	if err := os.WriteFile(mailboxPath, []byte(existingContent), 0o644); err != nil {
		t.Fatalf("write mailbox: %v", err)
	}

	// Append a new message 300ms after watch starts.
	go func() {
		time.Sleep(300 * time.Millisecond)
		f, err := os.OpenFile(mailboxPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		defer f.Close()
		_, _ = f.WriteString("### 2026-01-01T00:00:00Z | MSG-1 | from: agent-b\nhello from b\n\n")
	}()

	r := Runner{app: &app.App{}, stdout: &bytes.Buffer{}}
	var out bytes.Buffer
	r.stdout = &out

	start := time.Now()
	// Timeout 10s — if the bug is still present, this call blocks for 10s.
	err = r.executeMessageWatch([]string{"--agent", "agent-a", "--timeout", "10s"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("executeMessageWatch returned error: %v", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("watch took %v — expected exit within 3s of message arrival, not at timeout", elapsed)
	}
	if !strings.Contains(out.String(), "[inbox]") {
		t.Errorf("expected [inbox] output, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "hello from b") {
		t.Errorf("expected message text in output, got: %q", out.String())
	}
}

// TestMessageWatchTimesOutWhenNoMessage verifies that executeMessageWatch
// prints the timeout message and returns nil when no new message arrives.
func TestMessageWatchTimesOutWhenNoMessage(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	aomDir := filepath.Join(dir, ".aom")
	_ = os.MkdirAll(aomDir, 0o755)
	projectYAML := `name: watch-timeout-test
repo: .
default_branch: main
runtime:
  terminal: tmux
  session_prefix: wt
context:
  state_dir: .aom/state
`
	_ = os.WriteFile(filepath.Join(aomDir, "project.yaml"), []byte(projectYAML), 0o644)

	r := Runner{app: &app.App{}, stdout: &bytes.Buffer{}}
	var out bytes.Buffer
	r.stdout = &out

	// 1s timeout — no message sent.
	start := time.Now()
	err = r.executeMessageWatch([]string{"--agent", "agent-x", "--timeout", "1s"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("watch exited too early (%v) — should wait ~1s", elapsed)
	}
	if !strings.Contains(out.String(), "timed out") {
		t.Errorf("expected timeout message, got: %q", out.String())
	}
}

// TestMessageWatchSkipsExistingContent verifies that messages already in the
// mailbox when watch starts are NOT re-printed (only new arrivals are shown).
func TestMessageWatchSkipsExistingContent(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	aomDir := filepath.Join(dir, ".aom")
	_ = os.MkdirAll(aomDir, 0o755)
	projectYAML := `name: watch-skip-test
repo: .
default_branch: main
runtime:
  terminal: tmux
  session_prefix: wt
context:
  state_dir: .aom/state
`
	_ = os.WriteFile(filepath.Join(aomDir, "project.yaml"), []byte(projectYAML), 0o644)

	mailboxDir := filepath.Join(aomDir, "mailbox")
	_ = os.MkdirAll(mailboxDir, 0o755)
	mailboxPath := filepath.Join(mailboxDir, "agent-c.md")

	// Pre-existing message — must NOT be printed.
	existing := "# Mailbox: agent-c\n\n## Messages\n\n### 2026-01-01T00:00:00Z | MSG-OLD | from: x\nold message\n\n"
	_ = os.WriteFile(mailboxPath, []byte(existing), 0o644)

	// New message arrives 300ms after watch starts.
	go func() {
		time.Sleep(300 * time.Millisecond)
		f, _ := os.OpenFile(mailboxPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if f != nil {
			defer f.Close()
			_, _ = f.WriteString("### 2026-01-01T00:01:00Z | MSG-NEW | from: y\nnew message\n\n")
		}
	}()

	r := Runner{app: &app.App{}, stdout: &bytes.Buffer{}}
	var out bytes.Buffer
	r.stdout = &out

	_ = r.executeMessageWatch([]string{"--agent", "agent-c", "--timeout", "5s"})

	output := out.String()
	if strings.Contains(output, "old message") {
		t.Errorf("watch printed pre-existing message — should have been skipped. output: %q", output)
	}
	if !strings.Contains(output, "new message") {
		t.Errorf("watch did not print new message. output: %q", output)
	}
}
