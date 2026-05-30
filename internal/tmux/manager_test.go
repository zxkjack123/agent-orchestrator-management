package tmux

import (
	"errors"
	"testing"
)

func TestManagerAvailabilityReturnsFoundBinary(t *testing.T) {
	manager := NewManagerWithLookPath(func(name string) (string, error) {
		if name != "tmux" {
			t.Fatalf("looked up %q, want tmux", name)
		}

		return "/usr/bin/tmux", nil
	})

	availability := manager.Availability()
	if !availability.Available {
		t.Fatalf("Available = false, want true")
	}
	if availability.BinaryPath != "/usr/bin/tmux" {
		t.Fatalf("BinaryPath = %q, want %q", availability.BinaryPath, "/usr/bin/tmux")
	}
}

func TestManagerAvailabilityReturnsUnavailableWhenMissing(t *testing.T) {
	manager := NewManagerWithLookPath(func(name string) (string, error) {
		return "", errors.New("not found")
	})

	availability := manager.Availability()
	if availability.Available {
		t.Fatalf("Available = true, want false")
	}
	if availability.BinaryPath != "" {
		t.Fatalf("BinaryPath = %q, want empty", availability.BinaryPath)
	}
}

func TestManagerProjectSessionNameSanitizesPrefix(t *testing.T) {
	manager := NewManagerWithLookPath(nil)

	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{name: "simple", prefix: "my-app", want: "aom-my-app"},
		{name: "spaces and caps", prefix: "My App", want: "aom-my-app"},
		{name: "symbols removed", prefix: "qa/review@1", want: "aom-qareview1"},
		{name: "empty fallback", prefix: "   ", want: "aom-project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.ProjectSessionName(tt.prefix)
			if got != tt.want {
				t.Fatalf("ProjectSessionName(%q) = %q, want %q", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestManagerSessionTargetMatchesProjectSessionName(t *testing.T) {
	manager := NewManagerWithLookPath(nil)

	got := manager.SessionTarget("demo-app")
	if got != "aom-demo-app" {
		t.Fatalf("SessionTarget = %q, want %q", got, "aom-demo-app")
	}
}

func TestManagerEnsureWorkspaceReturnsUnavailableErrorWhenTmuxMissing(t *testing.T) {
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "", errors.New("not found") },
		func(string, ...string) ([]byte, error) {
			t.Fatal("exec should not be called when tmux is unavailable")
			return nil, nil
		},
		nil,
	)

	_, err := manager.EnsureWorkspace("my-app", "/repo")
	if err == nil {
		t.Fatal("EnsureWorkspace should fail when tmux is unavailable")
	}
	if err.Error() != "tmux is not available in the current environment" {
		t.Fatalf("error = %q, want availability error", err)
	}
}

func TestManagerEnsureWorkspaceReusesExistingSession(t *testing.T) {
	var calls [][]string
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			call := append([]string{name}, args...)
			calls = append(calls, call)
			return nil, nil
		},
		nil,
	)

	workspace, err := manager.EnsureWorkspace("my-app", "/repo")
	if err != nil {
		t.Fatalf("EnsureWorkspace failed: %v", err)
	}
	if workspace.Created {
		t.Fatal("workspace should be reused, not created")
	}
	if workspace.Target != "aom-my-app" {
		t.Fatalf("Target = %q, want %q", workspace.Target, "aom-my-app")
	}
	if len(calls) != 1 {
		t.Fatalf("command call count = %d, want 1", len(calls))
	}
}

func TestManagerEnsureWorkspaceCreatesSessionWhenMissing(t *testing.T) {
	var calls [][]string
	firstCall := true
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			call := append([]string{name}, args...)
			calls = append(calls, call)
			if firstCall {
				firstCall = false
				return nil, errors.New("session not found")
			}
			return nil, nil
		},
		nil,
	)

	workspace, err := manager.EnsureWorkspace("my-app", "/repo")
	if err != nil {
		t.Fatalf("EnsureWorkspace failed: %v", err)
	}
	if !workspace.Created {
		t.Fatal("workspace should be marked created")
	}
	if len(calls) != 3 {
		t.Fatalf("command call count = %d, want 3", len(calls))
	}
	if calls[1][1] != "new-session" {
		t.Fatalf("second command = %v, want new-session", calls[1])
	}
	if calls[2][1] != "rename-window" {
		t.Fatalf("third command = %v, want rename-window", calls[2])
	}
}

func TestManagerCreatePaneParsesBindingOutput(t *testing.T) {
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			return []byte("@3 %7\n"), nil
		},
		nil,
	)

	binding, err := manager.CreatePane("aom-my-app", "/repo", "echo hello")
	if err != nil {
		t.Fatalf("CreatePane failed: %v", err)
	}
	if binding.WindowID != "@3" {
		t.Fatalf("WindowID = %q, want %q", binding.WindowID, "@3")
	}
	if binding.PaneID != "%7" {
		t.Fatalf("PaneID = %q, want %q", binding.PaneID, "%7")
	}
}

func TestManagerAnnotatePaneSetsUserOptions(t *testing.T) {
	var calls [][]string
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			call := append([]string{name}, args...)
			calls = append(calls, call)
			return nil, nil
		},
		nil,
	)

	err := manager.AnnotatePane("%7", map[string]string{
		"@aom_session_id": "SESS-1",
		"@aom_agent":      "backend-main",
	})
	if err != nil {
		t.Fatalf("AnnotatePane failed: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("command call count = %d, want 2", len(calls))
	}
}

func TestManagerAttachPaneSelectsPaneThenAttaches(t *testing.T) {
	var execCalls [][]string
	var runCalls [][]string
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			execCalls = append(execCalls, append([]string{name}, args...))
			return nil, nil
		},
		func(name string, args ...string) error {
			runCalls = append(runCalls, append([]string{name}, args...))
			return nil
		},
	)

	if err := manager.AttachPane("aom-my-app", "%7"); err != nil {
		t.Fatalf("AttachPane failed: %v", err)
	}
	if len(execCalls) != 1 || execCalls[0][1] != "select-pane" {
		t.Fatalf("exec calls = %v, want select-pane first", execCalls)
	}
	if len(runCalls) != 1 || runCalls[0][1] != "attach-session" {
		t.Fatalf("run calls = %v, want attach-session", runCalls)
	}
}

func TestManagerCapturePaneReturnsOutput(t *testing.T) {
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			return []byte("hello\nworld\n"), nil
		},
		nil,
	)

	output, err := manager.CapturePane("%7")
	if err != nil {
		t.Fatalf("CapturePane failed: %v", err)
	}
	if output != "hello\nworld\n" {
		t.Fatalf("output = %q, want capture text", output)
	}
}

func TestManagerSendKeysSendsLiteralMessageThenEnter(t *testing.T) {
	var calls [][]string
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			calls = append(calls, append([]string{name}, args...))
			return nil, nil
		},
		nil,
	)

	if err := manager.SendKeys("%7", "read .agent/task.md and begin work"); err != nil {
		t.Fatalf("SendKeys failed: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("command call count = %d, want 2", len(calls))
	}
	if calls[0][1] != "send-keys" || calls[0][4] != "-l" {
		t.Fatalf("first command = %v, want literal send-keys", calls[0])
	}
	if calls[1][1] != "send-keys" || calls[1][4] != "Enter" {
		t.Fatalf("second command = %v, want Enter send-keys", calls[1])
	}
}

func TestManagerSendKeysUsesBufferForMultilineMessage(t *testing.T) {
	var calls [][]string
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			calls = append(calls, append([]string{name}, args...))
			return nil, nil
		},
		nil,
	)

	multiline := "line one\nline two\nline three"
	if err := manager.SendKeys("%7", multiline); err != nil {
		t.Fatalf("SendKeys failed: %v", err)
	}
	// Expect: set-buffer, paste-buffer, send-keys Enter
	if len(calls) != 3 {
		t.Fatalf("command call count = %d, want 3 (set-buffer + paste-buffer + Enter)", len(calls))
	}
	if calls[0][1] != "set-buffer" {
		t.Fatalf("first command = %v, want set-buffer", calls[0])
	}
	// The message must be the last arg to set-buffer.
	if calls[0][len(calls[0])-1] != multiline {
		t.Fatalf("set-buffer data = %q, want %q", calls[0][len(calls[0])-1], multiline)
	}
	if calls[1][1] != "paste-buffer" {
		t.Fatalf("second command = %v, want paste-buffer", calls[1])
	}
	if calls[2][1] != "send-keys" || calls[2][len(calls[2])-1] != "Enter" {
		t.Fatalf("third command = %v, want Enter send-keys", calls[2])
	}
}

func TestManagerPaneExistsReturnsTrueForLivePane(t *testing.T) {
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			return []byte("%7\n"), nil
		},
		nil,
	)

	exists, err := manager.PaneExists("%7")
	if err != nil {
		t.Fatalf("PaneExists failed: %v", err)
	}
	if !exists {
		t.Fatal("PaneExists = false, want true")
	}
}

func TestManagerPaneExistsReturnsFalseWhenPaneIsMissing(t *testing.T) {
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			return nil, errors.New("pane not found")
		},
		nil,
	)

	exists, err := manager.PaneExists("%7")
	if err != nil {
		t.Fatalf("PaneExists failed: %v", err)
	}
	if exists {
		t.Fatal("PaneExists = true, want false")
	}
}

func TestManagerKillPaneInvokesTmuxKillPane(t *testing.T) {
	var calls [][]string
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			calls = append(calls, append([]string{name}, args...))
			return nil, nil
		},
		nil,
	)

	if err := manager.KillPane("%7"); err != nil {
		t.Fatalf("KillPane failed: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(calls))
	}
	if calls[0][1] != "kill-pane" {
		t.Fatalf("call = %v, want kill-pane", calls[0])
	}
}

// TestKillPaneAndDescendantsIsIdempotentWhenPaneGone verifies that
// KillPaneAndDescendants returns nil without issuing a kill-pane command when
// the pane has already disappeared (PaneExists → false).
func TestKillPaneAndDescendantsIsIdempotentWhenPaneGone(t *testing.T) {
	var calls [][]string
	manager := NewManagerWithDeps(
		func(string) (string, error) { return "/usr/bin/tmux", nil },
		func(name string, args ...string) ([]byte, error) {
			calls = append(calls, append([]string{name}, args...))
			// display-message (PaneExists check) returns error → pane is gone.
			for _, a := range args {
				if a == "display-message" {
					return nil, errors.New("no such pane")
				}
			}
			return nil, nil
		},
		nil,
	)

	if err := manager.KillPaneAndDescendants("%99"); err != nil {
		t.Fatalf("KillPaneAndDescendants on gone pane: want nil, got %v", err)
	}
	for _, call := range calls {
		for _, arg := range call {
			if arg == "kill-pane" {
				t.Errorf("kill-pane should not be called when pane is already gone; calls=%v", calls)
			}
		}
	}
}
