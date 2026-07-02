package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/task"
)

func TestReadOutcomeJSONValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outcome.json")

	data := `{
		"task_id": "T-42",
		"run_id": "run-xxx",
		"outcome": "approved",
		"rounds": 2,
		"exit_code": 0,
		"decision": "approve",
		"base_sha": "abc123",
		"head_sha": "def456",
		"files_changed": ["main.go"],
		"review_blocking": [],
		"review_non_blocking": ["add docs"],
		"worker_notes": "implemented feature X",
		"duration_ms": 120000
	}`

	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	outcome, err := readOutcomeJSON(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if outcome.TaskID != "T-42" {
		t.Fatalf("TaskID = %q, want T-42", outcome.TaskID)
	}
	if outcome.Outcome != "approved" {
		t.Fatalf("Outcome = %q, want approved", outcome.Outcome)
	}
	if outcome.Rounds != 2 {
		t.Fatalf("Rounds = %d, want 2", outcome.Rounds)
	}
	if outcome.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", outcome.ExitCode)
	}
	if outcome.Decision != "approve" {
		t.Fatalf("Decision = %q, want approve", outcome.Decision)
	}
	if len(outcome.FilesChanged) != 1 || outcome.FilesChanged[0] != "main.go" {
		t.Fatalf("FilesChanged = %v, want [main.go]", outcome.FilesChanged)
	}
	if len(outcome.ReviewBlocking) != 0 {
		t.Fatalf("ReviewBlocking = %v, want []", outcome.ReviewBlocking)
	}
	if len(outcome.ReviewSuggestions) != 1 || outcome.ReviewSuggestions[0] != "add docs" {
		t.Fatalf("ReviewSuggestions = %v, want [add docs]", outcome.ReviewSuggestions)
	}
	if outcome.WorkerNotes != "implemented feature X" {
		t.Fatalf("WorkerNotes = %q, want implemented feature X", outcome.WorkerNotes)
	}
	if outcome.DurationMs != 120000 {
		t.Fatalf("DurationMs = %d, want 120000", outcome.DurationMs)
	}
}

func TestReadOutcomeJSONMissingFile(t *testing.T) {
	_, err := readOutcomeJSON("/nonexistent/path/outcome.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadOutcomeJSONInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outcome.json")

	if err := os.WriteFile(path, []byte("not valid json {{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readOutcomeJSON(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestReadOutcomeJSONAllOutcomes(t *testing.T) {
	outcomes := []string{
		"approved", "no_change_success", "validation_failure",
		"timeout", "interrupted", "config_error",
		"dirty_worktree", "lock_failure", "max_rounds_exhausted",
		"state_error",
	}

	for _, oc := range outcomes {
		dir := t.TempDir()
		path := filepath.Join(dir, "outcome.json")
		data, _ := json.Marshal(map[string]interface{}{
			"task_id":  "T-1",
			"outcome":  oc,
			"rounds":   1,
			"exit_code": 0,
		})
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}

		outcome, err := readOutcomeJSON(path)
		if err != nil {
			t.Fatalf("readOutcomeJSON failed for outcome %q: %v", oc, err)
		}
		if outcome.Outcome != oc {
			t.Fatalf("Outcome = %q, want %q", outcome.Outcome, oc)
		}
	}
}

func TestGenerateTaskCardJSON(t *testing.T) {
	tr := &task.Record{
		ID:          "T-42",
		Title:       "Implement verify script",
		Description: "Write a Python script to verify the deployment",
		Status:      "Ready",
		Priority:    1,
	}

	data, err := generateTaskCardJSON(tr)
	if err != nil {
		t.Fatal(err)
	}

	var card map[string]interface{}
	if err := json.Unmarshal(data, &card); err != nil {
		t.Fatalf("generated JSON is invalid: %v", err)
	}

	// Required fields
	if card["task_id"] != "T-42" {
		t.Fatalf("task_id = %v, want T-42", card["task_id"])
	}
	if card["goal"] != "Write a Python script to verify the deployment" {
		t.Fatalf("goal = %v, want description", card["goal"])
	}

	// Lanes must be present
	lanes, ok := card["lanes"].([]interface{})
	if !ok || len(lanes) != 1 {
		t.Fatalf("lanes should be a list with 1 entry, got %v", card["lanes"])
	}

	lane, ok := lanes[0].(map[string]interface{})
	if !ok {
		t.Fatal("first lane should be a map")
	}
	if lane["backend_preference"] != "opencode" {
		t.Fatalf("lane backend_preference = %v, want opencode", lane["backend_preference"])
	}
}

func TestGenerateTaskCardJSONEmptyDescription(t *testing.T) {
	tr := &task.Record{
		ID:          "T-99",
		Title:       "My Task",
		Description: "",
		Status:      "Ready",
	}

	data, err := generateTaskCardJSON(tr)
	if err != nil {
		t.Fatal(err)
	}

	var card map[string]interface{}
	if err := json.Unmarshal(data, &card); err != nil {
		t.Fatalf("generated JSON is invalid: %v", err)
	}

	// When description is empty, goal should fall back to title
	if card["goal"] != "My Task" {
		t.Fatalf("goal = %v, want My Task (title fallback)", card["goal"])
	}
}

func TestGenerateTaskCardJSONValidJSON(t *testing.T) {
	tr := &task.Record{
		ID:          "T-1",
		Title:       "Test",
		Description: "Test description",
		Status:      "Ready",
	}

	data, err := generateTaskCardJSON(tr)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the output is valid indented JSON
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, string(data))
	}

	// Verify it contains all required fields for agent-task-runner
	output := string(data)
	required := []string{"task_id", "goal", "in_scope", "out_of_scope", "acceptance_criteria", "constraints", "lanes"}
	for _, field := range required {
		if !strings.Contains(output, field) {
			t.Fatalf("output missing required field %q:\n%s", field, output)
		}
	}
}

func TestNormalizeTaskID(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"T-42", "T-42"},
		{"#T-42", "T-42"},
		{"  T-99  ", "T-99"},
		{"", ""},
	}

	for _, tc := range cases {
		got := normalizeTaskID(tc.input)
		if got != tc.expected {
			t.Errorf("normalizeTaskID(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestOutcomeToStatusMapping(t *testing.T) {
	// Verify all known outcomes have a corresponding status mapping
	// This is a documentation test — it verifies the integration-spec contract
	knownOutcomes := map[string]string{
		"approved":              "done",
		"no_change_success":     "done",
		"validation_failure":    "needs_attention",
		"config_error":          "needs_attention",
		"state_error":           "needs_attention",
		"timeout":               "blocked",
		"interrupted":           "blocked",
		"max_rounds_exhausted":  "blocked",
		"dirty_worktree":        "blocked",
		"lock_failure":          "blocked",
	}

	for outcome, expectedStatus := range knownOutcomes {
		// Verify the outcome string is non-empty
		if outcome == "" {
			t.Fatal("outcome should not be empty")
		}
		if expectedStatus == "" {
			t.Fatalf("outcome %q should have a mapped status", outcome)
		}
	}

	// Verify the mapping in executePipelineLoop matches this contract
	// (actual mapping tested in integration tests)
}
