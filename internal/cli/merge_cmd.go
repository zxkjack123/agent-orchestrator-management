package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/artifact"
	aommerge "github.com/lattapon-aek/Agents-Orchestfator-Management/internal/merge"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/step"
)

// resolveAgentArtifactConflicts auto-resolves a failed git merge when every
// unmerged file is under .agent/ (per-task metadata that should never block a
// code merge). Returns true if resolved and committed, false if any non-.agent/
// conflict exists so the caller can propagate the original error.
func resolveAgentArtifactConflicts(repoPath string) (bool, error) {
	// git ls-files -u lists unmerged index entries (stages 2 & 3).
	// Output format per line: "<mode> <hash> <stage>\t<path>"
	// --name-only is not available on all git versions, so we parse the path ourselves.
	out, err := exec.Command("git", "-C", repoPath, "ls-files", "-u").Output()
	if err != nil {
		return false, err
	}
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		// Tab separates the "<mode> <hash> <stage>" prefix from the path.
		idx := strings.Index(line, "\t")
		if idx < 0 {
			continue
		}
		seen[strings.TrimSpace(line[idx+1:])] = true
	}
	unmerged := make([]string, 0, len(seen))
	for f := range seen {
		unmerged = append(unmerged, f)
	}
	if len(unmerged) == 0 {
		return false, nil
	}
	for _, f := range unmerged {
		if !strings.HasPrefix(f, ".agent/") {
			return false, nil // real code conflict — caller must handle
		}
	}
	// All conflicts are .agent/ files. Take the incoming (theirs) version,
	// which is the latest task's artifacts, then stage and commit.
	agentFiles := unmerged
	checkoutArgs := append([]string{"-C", repoPath, "checkout", "--theirs", "--"}, agentFiles...)
	if o, err := exec.Command("git", checkoutArgs...).CombinedOutput(); err != nil {
		return false, fmt.Errorf("checkout --theirs failed: %s", strings.TrimSpace(string(o)))
	}
	// Use -f to override .gitignore (agent artifacts may be gitignored in the root repo).
	addArgs := append([]string{"-C", repoPath, "add", "-f", "--"}, agentFiles...)
	if o, err := exec.Command("git", addArgs...).CombinedOutput(); err != nil {
		return false, fmt.Errorf("git add failed: %s", strings.TrimSpace(string(o)))
	}
	if o, err := exec.Command("git", "-C", repoPath, "commit", "--no-edit").CombinedOutput(); err != nil {
		return false, fmt.Errorf("git commit failed: %s", strings.TrimSpace(string(o)))
	}
	return true, nil
}

func (r Runner) executeMergeCheck(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	taskID := strings.TrimSpace(args[0])
	var againstFlag string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--against":
			i++
			if i >= len(args) {
				return fmt.Errorf("--against requires a value")
			}
			againstFlag = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if view.Worktree == nil {
		return fmt.Errorf("task %q has no worktree", taskID)
	}

	sourceBranch := view.Worktree.BranchName

	// Resolve --against: accept a task ID or a branch name.
	otherBranch := againstFlag
	if strings.HasPrefix(againstFlag, "TASK-") {
		otherView, err := r.loadTaskView(result, againstFlag)
		if err != nil {
			return err
		}
		if otherView == nil {
			return fmt.Errorf("task %q not found", againstFlag)
		}
		if otherView.Worktree == nil {
			return fmt.Errorf("task %q has no worktree", againstFlag)
		}
		otherBranch = otherView.Worktree.BranchName
	}

	if otherBranch == "" {
		otherBranch = result.Project.DefaultBranch
	}

	base := result.Project.DefaultBranch

	checkResult, err := aommerge.CheckOverlaps(result.Project.RepoPath, sourceBranch, otherBranch, base)
	if err != nil {
		return fmt.Errorf("merge check: %w", err)
	}

	fmt.Fprintf(r.stdout, "Merge check: %s → %s\n", taskID, base)
	fmt.Fprintf(r.stdout, "Source branch: %s\n", sourceBranch)
	fmt.Fprintf(r.stdout, "Against:       %s\n", otherBranch)
	fmt.Fprintf(r.stdout, "Conflict score: %s (%d overlapping files)\n\n", checkResult.Score, len(checkResult.Overlaps))

	if len(checkResult.Overlaps) == 0 {
		fmt.Fprintln(r.stdout, "No overlapping files. Safe to merge.")
	} else {
		fmt.Fprintln(r.stdout, "Overlapping files:")
		for _, o := range checkResult.Overlaps {
			fmt.Fprintf(r.stdout, "  %s   (also in: %s)\n", o.Path, o.OtherBranch)
		}
		fmt.Fprintln(r.stdout, "\nReview overlapping files before merging.")
	}

	return nil
}

func (r Runner) executeMergePrepare(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	taskID := strings.TrimSpace(args[0])
	intoFlag := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--into":
			i++
			if i >= len(args) {
				return fmt.Errorf("--into requires a value")
			}
			intoFlag = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if view.Worktree == nil {
		return fmt.Errorf("task %q has no worktree", taskID)
	}

	targetBranch := intoFlag
	if targetBranch == "" {
		targetBranch = result.Project.DefaultBranch
	}

	sourceBranch := view.Worktree.BranchName
	base := result.Project.DefaultBranch

	checkResult, err := aommerge.CheckOverlaps(result.Project.RepoPath, sourceBranch, targetBranch, base)
	if err != nil {
		return fmt.Errorf("merge check: %w", err)
	}

	// Write merge-plan.md.
	svc := artifact.NewService(result.Project.RepoPath, result.StateDir)
	overlaps := make([]artifact.MergePlanOverlap, 0, len(checkResult.Overlaps))
	for _, o := range checkResult.Overlaps {
		overlaps = append(overlaps, artifact.MergePlanOverlap{
			Path:        o.Path,
			OtherBranch: o.OtherBranch,
		})
	}

	if err := svc.WriteMergePlan(artifact.SyncParams{
		Task:  view.Task,
		Steps: view.Steps,
	}, artifact.MergePlanParams{
		TaskID:        taskID,
		TargetBranch:  targetBranch,
		ConflictScore: string(checkResult.Score),
		Overlaps:      overlaps,
	}); err != nil {
		return err
	}

	// Create an integration step only when the task is still active.
	stepEffect := "merge plan written"
	if view.Task.Status != "Done" && view.Task.Status != "Archived" {
		stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
		if err != nil {
			return err
		}
		defer stepDB.Close()

		_, err = stepService.Create(step.CreateParams{
			ProjectID: result.Project.ID,
			TaskID:    taskID,
			StepType:  "integration",
			Title:     fmt.Sprintf("Merge %s into %s", sourceBranch, targetBranch),
			RoleName:  "operator",
		})
		if err != nil {
			return err
		}
		stepEffect = "integration step created"
	}

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "merge.prepared",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Merge plan prepared: %s → %s (score: %s)", sourceBranch, targetBranch, checkResult.Score),
		StateEffect: stepEffect,
	}, false); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Merge plan prepared\n\n")
	fmt.Fprintf(r.stdout, "Task:           %s\n", taskID)
	fmt.Fprintf(r.stdout, "Target branch:  %s\n", targetBranch)
	fmt.Fprintf(r.stdout, "Conflict score: %s\n", checkResult.Score)
	fmt.Fprintf(r.stdout, "Overlapping files: %d\n", len(checkResult.Overlaps))
	fmt.Fprintf(r.stdout, "merge-plan.md written to task artifact root.\n")
	return nil
}

func (r Runner) executeMergeCommit(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	taskID := strings.TrimSpace(args[0])
	intoFlag := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--into":
			i++
			if i >= len(args) {
				return fmt.Errorf("--into requires a value")
			}
			intoFlag = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	// Auto-run merge check before committing so conflicts are caught early.
	if err := r.executeMergeCheck([]string{taskID}); err != nil {
		return fmt.Errorf("pre-merge check: %w", err)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if view.Worktree == nil {
		return fmt.Errorf("task %q has no worktree", taskID)
	}

	targetBranch := intoFlag
	if targetBranch == "" {
		targetBranch = result.Project.DefaultBranch
	}

	sourceBranch := view.Worktree.BranchName

	// Safety check: require the task to be Done before merging.
	if view.Task.Status != "Done" {
		return fmt.Errorf("task %q is %s; close the task before merging (aom task close %s)", taskID, view.Task.Status, taskID)
	}

	// Guard: require at least one commit on the source branch ahead of the target.
	commitsOut, commitsErr := exec.Command("git", "-C", result.Project.RepoPath,
		"log", "--oneline", targetBranch+".."+sourceBranch).Output()
	if commitsErr != nil {
		return fmt.Errorf("check commits on %q: %w", sourceBranch, commitsErr)
	}
	if strings.TrimSpace(string(commitsOut)) == "" {
		return fmt.Errorf(
			"branch %q has no commits ahead of %q — nothing to merge\n"+
				"  hint: the agent must commit its work before the branch can be merged",
			sourceBranch, targetBranch)
	}

	// Run git merge --no-ff from the repo root.
	mergeMsg := fmt.Sprintf("Merge %s into %s\n\nTask: %s\n%s", sourceBranch, targetBranch, taskID, view.Task.Title)
	cmd := exec.Command("git", "merge", "--no-ff", sourceBranch, "-m", mergeMsg)
	cmd.Dir = result.Project.RepoPath

	// git merge must run on the target branch; verify and switch if needed.
	headOut, headErr := exec.Command("git", "-C", result.Project.RepoPath, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if headErr != nil {
		return fmt.Errorf("check current branch: %w", headErr)
	}
	currentBranch := strings.TrimSpace(string(headOut))
	if currentBranch != targetBranch {
		return fmt.Errorf("current branch is %q but target is %q — check out %q first, then re-run merge commit", currentBranch, targetBranch, targetBranch)
	}

	out, mergeErr := cmd.CombinedOutput()
	if mergeErr != nil {
		// Auto-resolve conflicts that are exclusively in .agent/ (task artifact files).
		// These are per-worktree metadata and should never block a code merge.
		if resolved, resolveErr := resolveAgentArtifactConflicts(result.Project.RepoPath); resolveErr != nil {
			return fmt.Errorf(
				"git merge failed — conflicts detected:\n%s\n\n"+
					"Resolve conflicts manually:\n"+
					"  1. Edit conflicting files and remove the conflict markers (<<<<< / ===== / >>>>>)\n"+
					"  2. git add <resolved-files>\n"+
					"  3. aom merge continue %s    ← commit the resolved merge\n"+
					"     OR: aom merge abort %s   ← discard and start over",
				strings.TrimSpace(string(out)), taskID, taskID,
			)
		} else if !resolved {
			return fmt.Errorf(
				"git merge failed — conflicts detected:\n%s\n\n"+
					"Resolve conflicts manually:\n"+
					"  1. Edit conflicting files and remove the conflict markers (<<<<< / ===== / >>>>>)\n"+
					"  2. git add <resolved-files>\n"+
					"  3. aom merge continue %s    ← commit the resolved merge\n"+
					"     OR: aom merge abort %s   ← discard and start over",
				strings.TrimSpace(string(out)), taskID, taskID,
			)
		}
		// Conflicts were only in .agent/ and have been auto-resolved; read the
		// commit summary for the success message.
		out, _ = exec.Command("git", "-C", result.Project.RepoPath, "log", "--oneline", "-1").Output()
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	steps, err := stepService.ListByTask(taskID)
	if err != nil {
		return err
	}
	completedSteps, err := autoCompleteIntegrationSteps(stepService, steps)
	if err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "merge.committed",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Merged %s into %s", sourceBranch, targetBranch),
		StateEffect: "branch merged",
	}, false); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Merged\n\n")
	fmt.Fprintf(r.stdout, "Source branch: %s\n", sourceBranch)
	fmt.Fprintf(r.stdout, "Target branch: %s\n", targetBranch)
	if len(completedSteps) > 0 {
		fmt.Fprintf(r.stdout, "Integration steps completed: %d\n", len(completedSteps))
	}
	fmt.Fprintf(r.stdout, "%s\n", strings.TrimSpace(string(out)))
	return nil
}

// executeMergeContinue completes a merge that was paused by conflicts.
// The operator must have resolved all conflicts and staged them (git add) before calling this.
func (r Runner) executeMergeContinue(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	taskID := strings.TrimSpace(args[0])

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// Verify we are actually in a MERGING state.
	mergeHeadOut, mergeHeadErr := exec.Command("git", "-C", result.Project.RepoPath,
		"rev-parse", "-q", "--verify", "MERGE_HEAD").Output()
	if mergeHeadErr != nil || strings.TrimSpace(string(mergeHeadOut)) == "" {
		return fmt.Errorf("no merge in progress in %q — nothing to continue", result.Project.RepoPath)
	}

	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if view.Worktree == nil {
		return fmt.Errorf("task %q has no worktree", taskID)
	}

	targetBranch := result.Project.DefaultBranch
	sourceBranch := view.Worktree.BranchName

	mergeMsg := fmt.Sprintf("Merge %s into %s\n\nTask: %s\n%s (conflict resolution)",
		sourceBranch, targetBranch, taskID, view.Task.Title)

	out, commitErr := exec.Command("git", "-C", result.Project.RepoPath,
		"commit", "--no-edit", "-m", mergeMsg).CombinedOutput()
	if commitErr != nil {
		return fmt.Errorf("git commit failed:\n%s\nIf there are still unresolved conflicts, fix them first (git add <file>)", strings.TrimSpace(string(out)))
	}

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "merge.committed",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Merged %s into %s (after conflict resolution)", sourceBranch, targetBranch),
		StateEffect: "branch merged",
	}, false); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Merge completed\n\n")
	fmt.Fprintf(r.stdout, "Source branch: %s\n", sourceBranch)
	fmt.Fprintf(r.stdout, "Target branch: %s\n", targetBranch)
	fmt.Fprintf(r.stdout, "%s\n", strings.TrimSpace(string(out)))
	return nil
}

// executeMergeAbort aborts a merge that was paused by conflicts and restores HEAD.
func (r Runner) executeMergeAbort(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// Verify we are actually in a MERGING state.
	mergeHeadOut, mergeHeadErr := exec.Command("git", "-C", result.Project.RepoPath,
		"rev-parse", "-q", "--verify", "MERGE_HEAD").Output()
	if mergeHeadErr != nil || strings.TrimSpace(string(mergeHeadOut)) == "" {
		return fmt.Errorf("no merge in progress in %q — nothing to abort", result.Project.RepoPath)
	}

	out, abortErr := exec.Command("git", "-C", result.Project.RepoPath, "merge", "--abort").CombinedOutput()
	if abortErr != nil {
		return fmt.Errorf("git merge --abort failed:\n%s", strings.TrimSpace(string(out)))
	}

	fmt.Fprintln(r.stdout, "Merge aborted — working tree restored to pre-merge state")
	fmt.Fprintf(r.stdout, "Task %s branch was not merged. Re-run \"aom merge commit %s\" when ready.\n",
		strings.TrimSpace(args[0]), strings.TrimSpace(args[0]))
	return nil
}
