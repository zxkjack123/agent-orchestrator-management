package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/provider"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/task"
)

// OutcomeJSON mirrors agent-task-runner's summary.json (integration-spec.md §3).
type OutcomeJSON struct {
	TaskID           string   `json:"task_id"`
	RunID            string   `json:"run_id"`
	Outcome          string   `json:"outcome"`
	Rounds           int      `json:"rounds"`
	ExitCode         int      `json:"exit_code"`
	Decision         string   `json:"decision"`
	BaseSHA          string   `json:"base_sha"`
	HeadSHA          string   `json:"head_sha"`
	FilesChanged     []string `json:"files_changed"`
	ReviewBlocking   []string `json:"review_blocking"`
	ReviewSuggestions []string `json:"review_non_blocking"`
	WorkerNotes      string   `json:"worker_notes"`
	DurationMs       int      `json:"duration_ms"`
}

// executePipelineLoop runs a task through agent-task-runner's PM→Worker→Reviewer loop.
//
// Usage: aom pipeline-loop <task-id> [--timeout <dur>]
//
// Flow:
//  1. Load task + project context
//  2. Generate task_card.json from task.Record
//  3. Ensure worktree is provisioned
//  4. Execute agent-task-runner as subprocess
//  5. Read outcome.json and map to task status
func (r Runner) executePipelineLoop(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task ID is required\nUsage: aom pipeline-loop <task-id> [--timeout <dur>]")
	}

	taskID := normalizeTaskID(args[0])
	timeout := 60 * time.Minute

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--timeout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--timeout requires a value (e.g. 60m, 2h)")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--timeout: %w", err)
			}
			timeout = d
		default:
			return fmt.Errorf("unknown flag: %s", args[i])
		}
	}

	// 1. Open project context
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return fmt.Errorf("failed to open project: %w", err)
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open task service: %w", err)
	}
	defer taskDB.Close()

	taskRecord, err := taskService.Get(taskID)
	if err != nil {
		return fmt.Errorf("failed to load task %s: %w", taskID, err)
	}

	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open worktree service: %w", err)
	}
	defer worktreeDB.Close()

	// 2. Generate task_card.json
	cardData, err := generateTaskCardJSON(taskRecord)
	if err != nil {
		return fmt.Errorf("failed to generate task card: %w", err)
	}

	// 3. Resolve worktree
	wt, err := worktreeService.EnsureProvisioned(taskID, result.Project.RepoPath)
	if err != nil {
		return fmt.Errorf("failed to provision worktree: %w", err)
	}

	loopDir := filepath.Join(wt.WorktreePath, ".loop")
	tasksDir := filepath.Join(loopDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		return fmt.Errorf("failed to create .loop/tasks: %w", err)
	}

	cardPath := filepath.Join(tasksDir, taskID+"_task_card.json")
	if err := os.WriteFile(cardPath, cardData, 0o644); err != nil {
		return fmt.Errorf("failed to write task_card.json: %w", err)
	}

	// 4. Execute agent-task-runner
	outcomePath := filepath.Join(loopDir, "outcome.json")
	cmdArgs := []string{
		"run", "--task", cardPath,
		"--worker-backend", "opencode",
		"--reviewer-backend", "opencode",
		"--auto-dispatch",
		"--max-rounds", "5",
		"--timeout", fmt.Sprintf("%d", int(timeout.Seconds())),
		"--dispatch-timeout", "900",
		"--artifact-timeout", "300",
		"--max-parallel-workers", "1",
		"--outcome-file", outcomePath,
		"--allow-dirty",
	}

	python, _ := provider.ResolveLoopKitBinary()
	cmd := exec.Command(python, append([]string{"-m", "loop_kit"}, cmdArgs...)...)
	cmd.Dir = wt.WorktreePath
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr

	fmt.Fprintf(r.stdout, "pipeline-loop: launching agent-task-runner for %s\n", taskID)
	fmt.Fprintf(r.stdout, "  worktree: %s\n", wt.WorktreePath)

	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(r.stderr, "agent-task-runner exited with error: %v\n", err)
	}

	elapsed := time.Since(startTime).Milliseconds()
	fmt.Fprintf(r.stdout, "  elapsed: %dms\n", elapsed)

	// 5. Read outcome and update task status
	outcome, err := readOutcomeJSON(outcomePath)
	if err != nil {
		return fmt.Errorf("failed to read outcome.json: %w", err)
	}

	fmt.Fprintf(r.stdout, "  outcome: %s (rounds=%d)\n", outcome.Outcome, outcome.Rounds)

	switch outcome.Outcome {
	case "approved", "no_change_success":
		_, err = taskService.Close(taskID)
		fmt.Fprintf(r.stdout, "  action: task closed (Done)\n")
	case "validation_failure", "config_error", "state_error":
		_, err = taskService.Update(taskID, task.UpdateParams{Status: "NeedsAttention"})
		fmt.Fprintf(r.stdout, "  action: task NeedsAttention\n")
	case "timeout", "interrupted", "max_rounds_exhausted", "dirty_worktree", "lock_failure":
		_, err = taskService.Update(taskID, task.UpdateParams{Status: "Blocked"})
		fmt.Fprintf(r.stdout, "  action: task Blocked\n")
	default:
		fmt.Fprintf(r.stdout, "  action: (unknown outcome %q, no status change)\n", outcome.Outcome)
	}
	// Sync outcome to PM system (if pm_outcome_handler is available)
	_ = syncPMOutcome(outcomePath, taskID)
	return err
}


// syncPMOutcome calls the PM outcome handler script to update PM task status.
func syncPMOutcome(outcomePath string, taskID string) error {
	// Try common locations for the pm_outcome_handler.py script
	for _, candidate := range []string{
		filepath.Join(os.Getenv("HOME"), "opt", "project_management", "scripts", "pm_outcome_handler.py"),
		"/home/gw/opt/project_management/scripts/pm_outcome_handler.py",
	} {
		if _, err := os.Stat(candidate); err == nil {
			cmd := exec.Command("python3", candidate, outcomePath, taskID)
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}
	return nil
}


// readOutcomeJSON reads the outcome.json file written by agent-task-runner.
func readOutcomeJSON(path string) (*OutcomeJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var o OutcomeJSON
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &o, nil
}

// generateTaskCardJSON creates agent-task-runner task_card.json from an AOM task record.
func generateTaskCardJSON(tr *task.Record) ([]byte, error) {
	goal := tr.Description
	if goal == "" {
		goal = tr.Title
	}
	card := map[string]interface{}{
		"task_id":             tr.ID,
		"goal":                goal,
		"in_scope":            []string{},
		"out_of_scope":        []string{},
		"acceptance_criteria": []string{},
		"constraints":         []string{},
		"lanes": []map[string]interface{}{
			{
				"lane_id":            tr.ID,
				"owner_paths":        []string{},
				"backend_preference": "opencode",
			},
		},
	}
	return json.MarshalIndent(card, "", "  ")
}

func normalizeTaskID(raw string) string {
	if len(raw) > 0 && raw[0] == '#' {
		return raw[1:]
	}
	return raw
}
