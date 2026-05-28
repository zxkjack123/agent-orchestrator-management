package cli

import (
	"fmt"
	"strings"
	"time"

	aomruntime "github.com/lattapon-aek/agent-orchestrator-management/internal/runtime"
)

// executeRunPipeline runs a complete task pipeline in sequence without operator
// intervention at each step:
//
//	spawn → wait(task.completed) → verify → accept → merge
//
// Usage: aom run-pipeline <task-id> [--agent <name>] [--timeout <dur>] [--real|--mock] [--skip-merge]
//
// Defaults: --timeout 60m, --mock (explicit --real required to launch real sessions).
// Each stage polls every 15 s and consumes from the shared timeout budget.
// On timeout at any stage, the command prints stage-specific escalation hints and exits.
func (r Runner) executeRunPipeline(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task ID is required\nUsage: aom run-pipeline <task-id> [--agent <name>] [--timeout <dur>] [--real|--mock] [--skip-merge]")
	}

	taskID := strings.TrimSpace(args[0])
	agentOverride := ""
	timeout := 60 * time.Minute
	skipMerge := false
	launchMode := aomruntime.LaunchModePlaceholder

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			agentOverride = strings.TrimSpace(args[i])
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
		case "--skip-merge":
			skipMerge = true
		case "--mock":
			if err := setLaunchMode(&launchMode, aomruntime.LaunchModeMock); err != nil {
				return err
			}
		case "--real":
			if err := setLaunchMode(&launchMode, aomruntime.LaunchModeReal); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if launchMode == aomruntime.LaunchModePlaceholder {
		return fmt.Errorf("--mock or --real is required\n  Use --mock for testing; --real to spawn an actual agent session")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// Resolve agent for this task.
	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	agentName := agentOverride
	if agentName == "" {
		agentName = view.Task.PreferredAgent
	}
	if agentName == "" {
		return fmt.Errorf(
			"no agent assigned to task %q\n  Assign one with: aom task update %s --agent <name>",
			taskID, taskID,
		)
	}

	launchFlag := "--mock"
	if launchMode == aomruntime.LaunchModeReal {
		launchFlag = "--real"
	}
	pollInterval := 15 * time.Second
	deadline := time.Now().Add(timeout)

	// ── helpers ─────────────────────────────────────────────────────────────

	printStage := func(n int, name string) {
		fmt.Fprintf(r.stdout, "\n━━━ Stage %d: %s ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n", n, name)
	}

	// escalate prints stage-specific diagnosis hints and returns a timeout error.
	escalate := func(stage, diagHint string) error {
		remaining := time.Until(deadline)
		fmt.Fprintf(r.stdout, "\n⚠  Pipeline timed out at stage %q  (remaining budget: %s)\n\n", stage, remaining.Round(time.Second))
		fmt.Fprintln(r.stdout, "Diagnose:")
		fmt.Fprintln(r.stdout, diagHint)
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintln(r.stdout, "Resume manually:")
		fmt.Fprintf(r.stdout, "  aom task verify %s\n", taskID)
		fmt.Fprintf(r.stdout, "  aom task accept %s\n", taskID)
		if !skipMerge {
			fmt.Fprintf(r.stdout, "  aom merge commit %s\n", taskID)
		}
		return fmt.Errorf("pipeline timeout at stage %q", stage)
	}

	// ── Header ───────────────────────────────────────────────────────────────

	stageList := "spawn → wait(task.completed) → verify → accept → merge"
	if skipMerge {
		stageList = "spawn → wait(task.completed) → verify → accept"
	}
	fmt.Fprintf(r.stdout, "Pipeline: %s\n", taskID)
	fmt.Fprintf(r.stdout, "  agent:   %s  (%s)\n", agentName, launchFlag)
	fmt.Fprintf(r.stdout, "  timeout: %s\n", timeout)
	fmt.Fprintf(r.stdout, "  stages:  %s\n", stageList)
	fmt.Fprintln(r.stdout, "")

	// ── Stage 1: Spawn ───────────────────────────────────────────────────────

	printStage(1, "Spawn")
	spawnArgs := []string{agentName, "--task", taskID, launchFlag}
	if err := r.executeSessionSpawn(spawnArgs); err != nil {
		return fmt.Errorf("pipeline: spawn failed: %w", err)
	}
	fmt.Fprintln(r.stdout, "Spawn: OK")

	// ── Stage 2: Wait for task.completed ─────────────────────────────────────

	printStage(2, "Wait for task.completed")
	fmt.Fprintln(r.stdout, "Polling every 15s — waiting for agent to signal completion...")
	iteration := 1
	for {
		if time.Now().After(deadline) {
			return escalate("wait(task.completed)", fmt.Sprintf(
				"  aom capture %s --diff\n  aom session recover <session-id>",
				agentName,
			))
		}
		freshView, _ := r.loadTaskView(result, taskID)
		if freshView != nil {
			for _, c := range r.runTaskVerifyChecks(result, freshView) {
				if c.name == "task.completed in log" && c.ok {
					fmt.Fprintln(r.stdout, "task.completed detected.")
					goto stage3
				}
			}
		}
		fmt.Fprintf(r.stdout, "  #%d  %s  still waiting...\n", iteration, time.Now().Format("15:04:05"))
		time.Sleep(pollInterval)
		iteration++
	}
stage3:

	// ── Stage 3: Verify all checks ────────────────────────────────────────────

	printStage(3, "Verify")
	iteration = 1
	for {
		if time.Now().After(deadline) {
			return escalate("verify", fmt.Sprintf(
				"  aom task verify %s\n  aom capture %s --diff",
				taskID, agentName,
			))
		}
		freshView, verifyErr := r.loadTaskView(result, taskID)
		if verifyErr != nil || freshView == nil {
			return fmt.Errorf("pipeline: could not load task during verify: %v", verifyErr)
		}
		checks := r.runTaskVerifyChecks(result, freshView)
		allOK := true
		fmt.Fprintf(r.stdout, "  #%d  %s\n", iteration, time.Now().Format("15:04:05"))
		for _, c := range checks {
			icon := "ok"
			if !c.ok {
				icon = "FAIL"
				allOK = false
			}
			if c.note != "" {
				fmt.Fprintf(r.stdout, "    [%s]  %s — %s\n", icon, c.name, c.note)
			} else {
				fmt.Fprintf(r.stdout, "    [%s]  %s\n", icon, c.name)
			}
		}
		if allOK {
			fmt.Fprintln(r.stdout, "Verify: all checks passed.")
			break
		}
		time.Sleep(pollInterval)
		iteration++
	}

	// ── Stage 4: Accept ───────────────────────────────────────────────────────

	printStage(4, "Accept")
	if err := r.executeTaskAccept([]string{taskID}); err != nil {
		return fmt.Errorf("pipeline: accept failed: %w", err)
	}

	// ── Stage 5: Merge ────────────────────────────────────────────────────────

	if skipMerge {
		fmt.Fprintf(r.stdout, "\n✓  Pipeline complete (merge skipped)\n")
		fmt.Fprintf(r.stdout, "   Run when ready: aom merge commit %s\n", taskID)
		return nil
	}

	printStage(5, "Merge")
	if mergeErr := r.executeMergeCommit([]string{taskID}); mergeErr != nil {
		// Task was already accepted — surface merge failure without masking prior success.
		fmt.Fprintf(r.stdout, "\n⚠  Merge failed (task was accepted successfully):\n  %v\n\n", mergeErr)
		fmt.Fprintln(r.stdout, "Resolve and retry:")
		fmt.Fprintf(r.stdout, "  aom merge commit %s\n", taskID)
		fmt.Fprintf(r.stdout, "  aom merge continue %s   # after resolving conflicts\n", taskID)
		return fmt.Errorf("pipeline: merge failed: %w", mergeErr)
	}

	fmt.Fprintln(r.stdout, "\n✓  Pipeline complete — task accepted and merged.")
	return nil
}
