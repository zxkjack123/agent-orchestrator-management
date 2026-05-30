package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/artifact"
	aommerge "github.com/lattapon-aek/agent-orchestrator-management/internal/merge"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/step"
)

// aomIdentityFiles are AOM-generated runtime identity files. When these appear
// as add/add conflicts they are always resolved with "ours" (target branch) because
// AOM regenerates them at the next session spawn.
var aomIdentityFiles = map[string]bool{
	"CLAUDE.md": true,
	"AGENTS.md": true,
	"GEMINI.md": true,
}

// parseUnmergedFiles returns unmerged paths from `git ls-files -u` output along
// with a set of paths that are pure add/add (have stage 2 or 3 but no stage 1).
func parseUnmergedFiles(lsOut string) (paths []string, addAdd map[string]bool) {
	stagesByPath := make(map[string]map[int]bool)
	for _, line := range strings.Split(strings.TrimSpace(lsOut), "\n") {
		idx := strings.Index(line, "\t")
		if idx < 0 {
			continue
		}
		path := strings.TrimSpace(line[idx+1:])
		fields := strings.Fields(line[:idx])
		if len(fields) < 3 {
			continue
		}
		stage := 0
		fmt.Sscanf(fields[2], "%d", &stage)
		if stagesByPath[path] == nil {
			stagesByPath[path] = make(map[int]bool)
		}
		stagesByPath[path][stage] = true
	}
	addAdd = make(map[string]bool)
	for p, stages := range stagesByPath {
		paths = append(paths, p)
		if !stages[1] {
			addAdd[p] = true
		}
	}
	return paths, addAdd
}

// resolveAgentArtifactConflicts auto-resolves a failed git merge for files that
// AOM owns and that should never block a code merge:
//   - .agent/ files → take "theirs" (the incoming task's latest artifacts)
//   - AOM identity files (CLAUDE.md, AGENTS.md, GEMINI.md) → take "ours" (target
//     branch version; AOM regenerates these at next spawn so source changes are safe to drop)
//
// Returns true if all conflicts were resolved and committed, false when real code
// conflicts remain so the caller can propagate the error.
func resolveAgentArtifactConflicts(repoPath string) (bool, error) {
	out, err := exec.Command("git", "-C", repoPath, "ls-files", "-u").Output()
	if err != nil {
		return false, err
	}
	unmerged, addAdd := parseUnmergedFiles(string(out))
	if len(unmerged) == 0 {
		return false, nil
	}

	var agentFiles, oursFiles, realConflicts []string
	for _, f := range unmerged {
		base := f[strings.LastIndex(f, "/")+1:]
		switch {
		case strings.HasPrefix(f, ".agent/"):
			agentFiles = append(agentFiles, f)
		case aomIdentityFiles[base] && addAdd[f]:
			oursFiles = append(oursFiles, f)
		default:
			realConflicts = append(realConflicts, f)
		}
	}
	if len(realConflicts) > 0 {
		return false, nil // real code conflicts remain — caller must handle
	}
	if len(agentFiles) == 0 && len(oursFiles) == 0 {
		return false, nil
	}

	if len(agentFiles) > 0 {
		checkoutArgs := append([]string{"-C", repoPath, "checkout", "--theirs", "--"}, agentFiles...)
		if o, checkErr := exec.Command("git", checkoutArgs...).CombinedOutput(); checkErr != nil {
			return false, fmt.Errorf("checkout --theirs (.agent/) failed: %s", strings.TrimSpace(string(o)))
		}
		addArgs := append([]string{"-C", repoPath, "add", "-f", "--"}, agentFiles...)
		if o, addErr := exec.Command("git", addArgs...).CombinedOutput(); addErr != nil {
			return false, fmt.Errorf("git add (.agent/) failed: %s", strings.TrimSpace(string(o)))
		}
	}
	if len(oursFiles) > 0 {
		checkoutArgs := append([]string{"-C", repoPath, "checkout", "--ours", "--"}, oursFiles...)
		if o, checkErr := exec.Command("git", checkoutArgs...).CombinedOutput(); checkErr != nil {
			return false, fmt.Errorf("checkout --ours (identity files) failed: %s", strings.TrimSpace(string(o)))
		}
		addArgs := append([]string{"-C", repoPath, "add", "--"}, oursFiles...)
		if o, addErr := exec.Command("git", addArgs...).CombinedOutput(); addErr != nil {
			return false, fmt.Errorf("git add (identity files) failed: %s", strings.TrimSpace(string(o)))
		}
	}

	if o, commitErr := exec.Command("git", "-C", repoPath, "commit", "--no-edit").CombinedOutput(); commitErr != nil {
		return false, fmt.Errorf("git commit failed: %s", strings.TrimSpace(string(o)))
	}
	return true, nil
}

// stripAgentArtifactsFromMerge removes .agent/ and .aom/ files that the agent branch
// introduced into the target branch via the merge commit. These directories are
// AOM-internal bookkeeping that must never live on a shared branch like main.
//
// Strategy: after a successful merge, diff ORIG_HEAD..HEAD for those paths.
// If any changed, remove them from the index, restore what ORIG_HEAD had (if anything),
// and amend the merge commit. This is idempotent — if nothing changed, returns nil.
func stripAgentArtifactsFromMerge(repoPath string) error {
	diffOut, err := exec.Command("git", "-C", repoPath,
		"diff", "--name-only", "ORIG_HEAD", "HEAD", "--", ".agent/", ".aom/").Output()
	if err != nil || strings.TrimSpace(string(diffOut)) == "" {
		return nil // no agent artifacts leaked into the merge — nothing to do
	}

	// Remove .agent/ and .aom/ from the index entirely.
	if o, err := exec.Command("git", "-C", repoPath,
		"rm", "--cached", "-r", "--ignore-unmatch", ".agent/", ".aom/").CombinedOutput(); err != nil {
		return fmt.Errorf("strip .agent/ from merge: rm --cached: %s", strings.TrimSpace(string(o)))
	}

	// Restore any .agent/ and .aom/ files that existed on the target branch (ORIG_HEAD).
	// ls-tree returns nothing if ORIG_HEAD had no such files — that is the common case.
	origFilesOut, _ := exec.Command("git", "-C", repoPath,
		"ls-tree", "-r", "--name-only", "ORIG_HEAD", "--", ".agent/", ".aom/").Output()
	if origFiles := strings.Fields(string(origFilesOut)); len(origFiles) > 0 {
		checkoutArgs := append([]string{"-C", repoPath, "checkout", "ORIG_HEAD", "--"}, origFiles...)
		if o, err := exec.Command("git", checkoutArgs...).CombinedOutput(); err != nil {
			return fmt.Errorf("strip .agent/ from merge: restore ORIG_HEAD: %s", strings.TrimSpace(string(o)))
		}
		addArgs := append([]string{"-C", repoPath, "add", "--"}, origFiles...)
		if o, err := exec.Command("git", addArgs...).CombinedOutput(); err != nil {
			return fmt.Errorf("strip .agent/ from merge: re-add: %s", strings.TrimSpace(string(o)))
		}
	}

	// Amend the merge commit to exclude the agent artifacts.
	if o, err := exec.Command("git", "-C", repoPath,
		"commit", "--amend", "--no-edit").CombinedOutput(); err != nil {
		return fmt.Errorf("strip .agent/ from merge: amend: %s", strings.TrimSpace(string(o)))
	}
	return nil
}

// resolveAddAddConflicts resolves add/add conflicts using the source branch version
// ("theirs"). Used when --prefer-branch is set: skeleton files added independently
// in both branches are resolved by keeping the source branch's copy.
// Returns true if any add/add conflicts were found and resolved.
func resolveAddAddConflicts(repoPath string) (bool, error) {
	out, err := exec.Command("git", "-C", repoPath, "ls-files", "-u").Output()
	if err != nil {
		return false, err
	}
	_, addAdd := parseUnmergedFiles(string(out))
	if len(addAdd) == 0 {
		return false, nil
	}
	files := make([]string, 0, len(addAdd))
	for f := range addAdd {
		files = append(files, f)
	}
	checkoutArgs := append([]string{"-C", repoPath, "checkout", "--theirs", "--"}, files...)
	if o, checkErr := exec.Command("git", checkoutArgs...).CombinedOutput(); checkErr != nil {
		return false, fmt.Errorf("checkout --theirs (add/add) failed: %s", strings.TrimSpace(string(o)))
	}
	addArgs := append([]string{"-C", repoPath, "add", "--"}, files...)
	if o, addErr := exec.Command("git", addArgs...).CombinedOutput(); addErr != nil {
		return false, fmt.Errorf("git add (add/add) failed: %s", strings.TrimSpace(string(o)))
	}
	// Check if there are still unresolved conflicts after handling add/add.
	remaining, remainErr := exec.Command("git", "-C", repoPath, "ls-files", "-u").Output()
	if remainErr != nil || strings.TrimSpace(string(remaining)) != "" {
		return false, nil // real conflicts remain — caller must handle
	}
	if o, commitErr := exec.Command("git", "-C", repoPath, "commit", "--no-edit").CombinedOutput(); commitErr != nil {
		return false, fmt.Errorf("git commit (add/add resolved) failed: %s", strings.TrimSpace(string(o)))
	}
	return true, nil
}

func mergeConflictError(gitOutput, taskID string) error {
	return fmt.Errorf(
		"git merge failed — conflicts detected:\n%s\n\n"+
			"Resolve conflicts manually:\n"+
			"  1. Edit conflicting files and remove the conflict markers (<<<<< / ===== / >>>>>)\n"+
			"  2. git add <resolved-files>\n"+
			"  3. aom merge continue %s    ← commit the resolved merge\n"+
			"     OR: aom merge abort %s   ← discard and start over",
		strings.TrimSpace(gitOutput), taskID, taskID,
	)
}

// detectAddAddFiles returns files that were added independently in both branches
// (add/add pattern) by comparing each branch to their common merge base.
func detectAddAddFiles(repoPath, sourceBranch, otherBranch string) ([]string, error) {
	mbOut, err := exec.Command("git", "-C", repoPath, "merge-base", sourceBranch, otherBranch).Output()
	if err != nil || strings.TrimSpace(string(mbOut)) == "" {
		return nil, nil
	}
	mergeBase := strings.TrimSpace(string(mbOut))

	srcOut, err := exec.Command("git", "-C", repoPath, "diff", "--name-only", "--diff-filter=A", mergeBase+".."+sourceBranch).Output()
	if err != nil {
		return nil, nil
	}
	otherOut, err := exec.Command("git", "-C", repoPath, "diff", "--name-only", "--diff-filter=A", mergeBase+".."+otherBranch).Output()
	if err != nil {
		return nil, nil
	}

	srcFiles := make(map[string]bool)
	for _, f := range strings.Split(strings.TrimSpace(string(srcOut)), "\n") {
		if f = strings.TrimSpace(f); f != "" {
			srcFiles[f] = true
		}
	}
	var both []string
	for _, f := range strings.Split(strings.TrimSpace(string(otherOut)), "\n") {
		if f = strings.TrimSpace(f); f != "" && srcFiles[f] {
			both = append(both, f)
		}
	}
	return both, nil
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

	// Resolve source branch — workspace agent or legacy per-task worktree.
	// Pass empty targetBranch to skip the [TASK-xxx] tag check (check is for commit phase).
	sourceBranch, err := r.resolveSourceBranch(result, view, taskID, "")
	if err != nil {
		return err
	}

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
		otherBranch, err = r.resolveSourceBranch(result, otherView, againstFlag, "")
		if err != nil {
			return err
		}
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

	// Detect add/add risk: files added independently in both branches.
	addAddFiles, _ := detectAddAddFiles(result.Project.RepoPath, sourceBranch, otherBranch)
	if len(addAddFiles) > 0 {
		fmt.Fprintf(r.stdout, "\nAdd/add conflict risk: %d file(s) added independently in both branches:\n", len(addAddFiles))
		for _, f := range addAddFiles {
			fmt.Fprintf(r.stdout, "  + %s\n", f)
		}
		fmt.Fprintln(r.stdout, "  Hint: use --prefer-branch with 'aom merge commit' to auto-resolve by keeping the source branch version.")
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

	targetBranch := intoFlag
	if targetBranch == "" {
		targetBranch = result.Project.DefaultBranch
	}

	// Resolve source branch — workspace agent or legacy per-task worktree.
	sourceBranch, err := r.resolveSourceBranch(result, view, taskID, "")
	if err != nil {
		return err
	}
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

// resolveSourceBranch returns the git branch that holds the task's implementation work.
//
// For workspace-mode agents (agent.WorkspacePath != ""), the permanent per-agent branch
// "agents/<name>" is used as the source. The task's commits are expected to be tagged
// with "[TASK-xxx]" in their commit messages; this function verifies that at least one
// such commit exists ahead of targetBranch (caller passes "" to skip that check).
//
// For legacy per-task worktree agents (WorkspacePath == ""), the branch comes from
// view.Worktree.BranchName; a nil worktree is an error.
func (r Runner) resolveSourceBranch(result *project.OpenResult, view *taskView, taskID, targetBranch string) (string, error) {
	agentName := strings.TrimSpace(view.Task.PreferredAgent)
	if agentName != "" {
		agentRepo, agentDB, err := r.app.OpenAgentRepository(result.DBPath)
		if err != nil {
			return "", err
		}
		defer agentDB.Close()

		agents, err := agentRepo.ListByProjectID(result.Project.ID)
		if err != nil {
			return "", err
		}
		for _, ag := range agents {
			if ag.Name == agentName && strings.TrimSpace(ag.WorkspacePath) != "" {
				sourceBranch := "agents/" + agentName
				// Verify at least one [TASK-xxx] tagged commit exists on the agent branch.
				// Use --fixed-strings so "[TASK-xxx]" is treated as a literal string,
				// not a regex character class — git's default POSIX regex would error on it.
				if targetBranch != "" {
					taskTag := "[" + taskID + "]"
					taggedOut, _ := exec.Command("git", "-C", result.Project.RepoPath,
						"log", "--oneline", "--fixed-strings", "--grep="+taskTag,
						targetBranch+".."+sourceBranch).Output()
					if strings.TrimSpace(string(taggedOut)) == "" {
						return "", fmt.Errorf(
							"no commits tagged %q found on branch %q ahead of %q\n"+
								"  hint: commit task work with [%s] prefix in the message, e.g.:\n"+
								"    git commit -m %q",
							taskTag, sourceBranch, targetBranch, taskID,
							"["+taskID+"] implement feature X")
					}
				}
				return sourceBranch, nil
			}
		}
	}

	// Legacy per-task worktree path.
	if view.Worktree == nil {
		return "", fmt.Errorf("task %q has no worktree — agent has no workspace and no per-task worktree", taskID)
	}
	return view.Worktree.BranchName, nil
}

func (r Runner) executeMergeCommit(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	taskID := strings.TrimSpace(args[0])
	intoFlag := ""
	preferBranch := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--into":
			i++
			if i >= len(args) {
				return fmt.Errorf("--into requires a value")
			}
			intoFlag = strings.TrimSpace(args[i])
		case "--prefer-branch":
			preferBranch = true
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

	targetBranch := intoFlag
	if targetBranch == "" {
		targetBranch = result.Project.DefaultBranch
	}

	// Safety check: require the task to be Done before merging.
	if view.Task.Status != "Done" {
		return fmt.Errorf("task %q is %s; close the task before merging (aom task close %s)", taskID, view.Task.Status, taskID)
	}

	// Resolve source branch — workspace agent ("agents/<name>") or legacy per-task worktree branch.
	sourceBranch, err := r.resolveSourceBranch(result, view, taskID, targetBranch)
	if err != nil {
		return err
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
		// Step 1: auto-resolve .agent/ and AOM identity file conflicts.
		resolved, resolveErr := resolveAgentArtifactConflicts(result.Project.RepoPath)
		if resolveErr != nil {
			return mergeConflictError(string(out), taskID)
		}
		// Step 2: if --prefer-branch, also resolve remaining add/add conflicts.
		if !resolved && preferBranch {
			resolved, resolveErr = resolveAddAddConflicts(result.Project.RepoPath)
			if resolveErr != nil {
				return mergeConflictError(string(out), taskID)
			}
		}
		if !resolved {
			hint := ""
			if !preferBranch {
				// Check whether add/add conflicts exist so we can suggest the flag.
				addAddFiles, _ := detectAddAddFiles(result.Project.RepoPath, sourceBranch, targetBranch)
				if len(addAddFiles) > 0 {
					hint = fmt.Sprintf("\n  Tip: %d add/add conflict(s) detected — re-run with --prefer-branch to auto-resolve by keeping the source branch version.", len(addAddFiles))
				}
			}
			return fmt.Errorf(
				"git merge failed — conflicts detected:\n%s%s\n\n"+
					"Resolve conflicts manually:\n"+
					"  1. Edit conflicting files and remove the conflict markers (<<<<< / ===== / >>>>>)\n"+
					"  2. git add <resolved-files>\n"+
					"  3. aom merge continue %s    ← commit the resolved merge\n"+
					"     OR: aom merge abort %s   ← discard and start over",
				strings.TrimSpace(string(out)), hint, taskID, taskID,
			)
		}
		out, _ = exec.Command("git", "-C", result.Project.RepoPath, "log", "--oneline", "-1").Output()
	}

	// Remove any .agent/ and .aom/ files that leaked into the target branch via the merge.
	// Agent artifacts must never live on a shared branch like main — they are bookkeeping
	// for AOM's own state and would pollute the codebase history for other contributors.
	if stripErr := stripAgentArtifactsFromMerge(result.Project.RepoPath); stripErr != nil {
		fmt.Fprintf(r.stderr, "warning: could not strip .agent/ from merge commit: %v\n", stripErr)
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

	targetBranch := result.Project.DefaultBranch

	// Resolve source branch — workspace agent ("agents/<name>") or legacy per-task worktree branch.
	// Pass empty targetBranch so resolveSourceBranch skips the [TASK-xxx] tag check (merge is
	// already in progress; tag verification was done by executeMergeCommit).
	sourceBranch, err := r.resolveSourceBranch(result, view, taskID, "")
	if err != nil {
		return err
	}

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
