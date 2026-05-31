package cli

import (
	"strings"
	"testing"
)

// ── aom goal ──────────────────────────────────────────────────────────────────

func TestGoalHelp(t *testing.T) {
	var out strings.Builder
	err := Execute([]string{"goal", "--help"}, &out, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "aom goal set") {
		t.Errorf("expected goal help output, got: %s", out.String())
	}
}

func TestGoalSetRequiresText(t *testing.T) {
	err := runCLI(t, "goal", "set")
	if err == nil {
		t.Fatal("expected error for missing goal text")
	}
	if !strings.Contains(err.Error(), "goal text is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGoalUnknownSubcommand(t *testing.T) {
	err := runCLI(t, "goal", "bogus")
	if err == nil {
		t.Fatal("expected error for unknown goal subcommand")
	}
	if !strings.Contains(err.Error(), "unknown goal command") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGoalNoSubcommand(t *testing.T) {
	err := runCLI(t, "goal")
	if err == nil {
		t.Fatal("expected error when no subcommand given")
	}
	if !strings.Contains(err.Error(), "goal subcommand required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── aom orchestrator ──────────────────────────────────────────────────────────

func TestOrchestratorModeHelp(t *testing.T) {
	var out strings.Builder
	err := Execute([]string{"orchestrator", "--help"}, &out, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "aom orchestrator start") {
		t.Errorf("expected orchestrator help output, got: %s", out.String())
	}
}

func TestOrchestratorModeNoSubcommand(t *testing.T) {
	err := runCLI(t, "orchestrator")
	if err == nil {
		t.Fatal("expected error when no subcommand given")
	}
	if !strings.Contains(err.Error(), "orchestrator subcommand required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOrchestratorModeUnknownSubcommand(t *testing.T) {
	err := runCLI(t, "orchestrator", "bogus")
	if err == nil {
		t.Fatal("expected error for unknown orchestrator subcommand")
	}
	if !strings.Contains(err.Error(), "unknown orchestrator command") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOrchestratorStartFlagValidation(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "--goal missing value",
			args:    []string{"orchestrator", "start", "--goal"},
			wantErr: "--goal requires a value",
		},
		{
			name:    "unknown flag",
			args:    []string{"orchestrator", "start", "--bogus"},
			wantErr: "unknown flag",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runCLI(t, tc.args...)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("want %q in error, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestOrchestratorViewFlagValidation(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "--layout missing value",
			args:    []string{"orchestrator", "view", "--layout"},
			wantErr: "--layout requires a value",
		},
		{
			name:    "unknown flag",
			args:    []string{"orchestrator", "view", "--bogus"},
			wantErr: "unknown flag",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runCLI(t, tc.args...)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("want %q in error, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestOrchestratorViewNoProject(t *testing.T) {
	err := runCLI(t, "orchestrator", "view")
	if err == nil {
		t.Fatal("expected error when no project found")
	}
	if !strings.Contains(err.Error(), "project") && !strings.Contains(err.Error(), "no AOM") {
		t.Errorf("expected project-not-found error, got: %v", err)
	}
}

func TestOrchestratorStartNoProject(t *testing.T) {
	err := runCLI(t, "orchestrator", "start", "--mock")
	if err == nil {
		t.Fatal("expected error when no project found")
	}
	// Should get a "no AOM project found" or similar error.
	if !strings.Contains(err.Error(), "project") && !strings.Contains(err.Error(), "no AOM") {
		t.Errorf("expected project-not-found error, got: %v", err)
	}
}
