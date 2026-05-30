package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/events"
)

// hookNames maps event type constants to the corresponding hook script name.
var hookNames = map[string]string{
	events.TaskCreated:        "on-task-created",
	events.TaskDone:           "on-task-done",
	events.TaskReady:          "on-task-ready",
	events.TaskBlocked:        "on-blocked",
	events.TaskNeedsAttention: "on-needs-attention",
	events.TaskApprovalNeeded: "on-approval-required",
	events.TaskPlanProposed:   "on-plan-proposed",
	events.TaskPlanApproved:   "on-plan-approved",
	events.TaskPlanRejected:   "on-plan-rejected",
	events.SessionSpawned:     "on-session-spawn",
	events.SessionIdle:        "on-idle",
}

// hookRunnerSubscriber returns an async handler that fires the .aom/hooks/<name>.sh
// for each received event. Unknown event types with no hook mapping are silently ignored.
func hookRunnerSubscriber() events.AsyncHandler {
	return func(e events.Event) {
		name, ok := hookNames[e.Type]
		if !ok {
			return
		}
		runHook(e.RepoPath, name, e.TaskID, e.Title, e.Status)
	}
}

// blockingHookRunnerSubscriber returns a sync handler that fires .aom/hooks/<name>.sh
// and blocks the originating operation when the script exits with code 2.
// A non-zero exit code other than 2 is treated as a warning and does not block.
func blockingHookRunnerSubscriber() events.SyncHandler {
	return func(e events.Event) error {
		name, ok := hookNames[e.Type]
		if !ok {
			return nil
		}
		return runHookBlocking(e.RepoPath, name, e.TaskID, e.Title, e.Status)
	}
}

// runHookBlocking runs a hook synchronously. Exit code 2 returns an error that
// blocks the caller; other non-zero codes print a warning but do not block.
func runHookBlocking(repoPath, hookName string, args ...string) error {
	hookPath := filepath.Join(repoPath, ".aom", "hooks", hookName+".sh")
	if _, err := os.Stat(hookPath); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", append([]string{hookPath}, args...)...)
	cmd.Env = append(os.Environ(),
		"AOM_REPO="+repoPath,
		"AOM_HOOK="+hookName,
	)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Fprintf(os.Stderr, "[hook: %s]\n%s\n", hookName, strings.TrimSpace(string(out)))
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = fmt.Sprintf("hook %q blocked operation (exit 2)", hookName)
			}
			return fmt.Errorf("%s", msg)
		}
		// Non-2 exit: warn only.
		fmt.Fprintf(os.Stderr, "[hook: %s] warning: exit %v\n", hookName, err)
	}
	return nil
}

// runHook executes .aom/hooks/<name>.sh if it exists. Args are passed as
// positional arguments to the script. Non-fatal: errors and non-zero exits
// are printed to stderr but never block the main AOM command.
// A 15-second timeout prevents a stalled hook from keeping its sh subprocess
// alive after aom exits.
func runHook(repoPath, hookName string, args ...string) {
	hookPath := filepath.Join(repoPath, ".aom", "hooks", hookName+".sh")
	if _, err := os.Stat(hookPath); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", append([]string{hookPath}, args...)...)
	cmd.Env = append(os.Environ(),
		"AOM_REPO="+repoPath,
		"AOM_HOOK="+hookName,
	)
	cmd.Dir = repoPath
	out, _ := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Fprintf(os.Stderr, "[hook: %s]\n%s\n", hookName, strings.TrimSpace(string(out)))
	}
}
