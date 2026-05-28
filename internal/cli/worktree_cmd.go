package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/artifact"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/worktree"
)

func (r Runner) executeWorktreeRepair(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("worktree repair does not accept extra positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskRecord, err := r.loadTaskByID(result, strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if taskRecord == nil {
		return fmt.Errorf("task %q not found", strings.TrimSpace(args[0]))
	}

	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer worktreeDB.Close()

	wasRepaired, record, err := worktreeService.Repair(taskRecord.ID, result.Project.RepoPath)
	if err != nil {
		return err
	}

	if wasRepaired {
		if err := r.syncTaskArtifacts(result, taskRecord.ID, artifact.Event{
			Type:        "worktree.repaired",
			Actor:       "operator",
			Summary:     "Worktree continuity was explicitly repaired",
			StateEffect: fmt.Sprintf("Worktree %s", record.Status),
		}, false); err != nil {
			return err
		}
		fmt.Fprintln(r.stdout, "Worktree repaired")
	} else {
		fmt.Fprintln(r.stdout, "Worktree already healthy, no repair needed")
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", taskRecord.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", record.Status)
	fmt.Fprintf(r.stdout, "Branch: %s\n", record.BranchName)
	fmt.Fprintf(r.stdout, "Path: %s\n", record.WorktreePath)

	return nil
}

func (r Runner) executeWorktreeReadFile(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: aom worktree read-file <task-id> <relative-path>")
	}

	taskID := strings.TrimSpace(args[0])
	relPath := filepath.Clean(strings.TrimSpace(args[1]))

	// Reject obvious traversal attempts before hitting the filesystem.
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("path %q escapes the worktree root", args[1])
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	worktreeService, wDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer wDB.Close()

	mapping, err := worktreeService.GetByTask(taskID)
	if err != nil {
		return err
	}
	if mapping == nil {
		return fmt.Errorf("task %q has no worktree", taskID)
	}
	if mapping.Status != "Ready" && mapping.Status != "Active" {
		return fmt.Errorf("worktree for task %q is not available (status: %s)", taskID, mapping.Status)
	}

	// Final path validation: resolved path must stay inside worktree root.
	worktreeRoot := filepath.Clean(mapping.WorktreePath)
	targetPath := filepath.Join(worktreeRoot, relPath)
	targetPath = filepath.Clean(targetPath)

	if !strings.HasPrefix(targetPath, worktreeRoot+string(filepath.Separator)) && targetPath != worktreeRoot {
		return fmt.Errorf("path %q escapes the worktree root", args[1])
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Audit trail.
	_ = r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:    "worktree.read",
		Actor:   "operator",
		Summary: fmt.Sprintf("Read file %s from worktree of task %s", relPath, taskID),
	}, false)

	fmt.Fprint(r.stdout, string(data))
	return nil
}

// executeWorktreeCommit stages and commits all changes in a task worktree using
// explicit GIT_DIR and GIT_WORK_TREE env vars, bypassing the .git pointer file
// so the commit works correctly regardless of how the worktree was provisioned.
//
// --local mode: skips all DB access and commits the current working directory
// directly using git's auto-detected directories. Use this when the AOM DB is
// unreachable (e.g., from inside a sandbox-restricted agent session where the
// database file lives outside the writable workspace).
//
// --deliverables-only: after staging all changes, unstages AOM orchestration
// artifacts (.agent/, .aom/, AGENTS.md, CLAUDE.md, GEMINI.md, KIRO.md) so
// the commit only contains deliverable code. Use this when task branches should
// not include orchestration state in their history.
func (r Runner) executeWorktreeCommit(args []string) error {
	localMode := false
	deliverablesOnly := false
	var taskID string
	var commitMsg string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--local":
			localMode = true
		case "--deliverables-only":
			deliverablesOnly = true
		case "-m", "--message":
			i++
			if i >= len(args) {
				return fmt.Errorf("-m requires a commit message")
			}
			commitMsg = strings.TrimSpace(args[i])
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown flag %q", args[i])
			}
			if taskID == "" {
				taskID = strings.TrimSpace(args[i])
			} else {
				return fmt.Errorf("unexpected argument %q", args[i])
			}
		}
	}

	if !localMode && taskID == "" {
		return fmt.Errorf("task identifier is required (or use --local to commit current directory without DB lookup)")
	}
	if commitMsg == "" {
		return fmt.Errorf("commit message is required (-m <msg>)")
	}

	// --local mode: commit CWD without touching the AOM database.
	// This is safe to use from inside a sandboxed agent session where the
	// DB lives outside the sandbox-writable workspace.
	if localMode {
		return r.executeWorktreeCommitLocal(commitMsg, deliverablesOnly)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	worktreeService, wDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer wDB.Close()

	mapping, err := worktreeService.GetByTask(taskID)
	if err != nil {
		return err
	}
	if mapping == nil {
		return fmt.Errorf("task %q has no worktree — work may be in the main repo; use git commit directly, then run: aom checkpoint <session-id>", taskID)
	}
	if mapping.Status != "Ready" && mapping.Status != "Active" {
		return fmt.Errorf("worktree for task %q is not available (status: %s) — use git commit directly, then run: aom checkpoint <session-id>", taskID, mapping.Status)
	}

	wtPath := mapping.WorktreePath
	gitDir, err := resolveWorktreeGitDir(result.Project.RepoPath, wtPath)
	if err != nil {
		return fmt.Errorf("resolve git dir: %w", err)
	}

	env := append(os.Environ(),
		"GIT_DIR="+gitDir,
		"GIT_WORK_TREE="+wtPath,
	)

	return runGitAddAndCommit(r.stdout, commitMsg, wtPath, env, deliverablesOnly)
}

// executeWorktreeCommitLocal performs git add -A && git commit in the current
// working directory without any DB access. It lets git auto-detect GIT_DIR and
// GIT_WORK_TREE, which works correctly inside a git worktree.
func (r Runner) executeWorktreeCommitLocal(commitMsg string, deliverablesOnly bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	return runGitAddAndCommit(r.stdout, commitMsg, cwd, nil, deliverablesOnly)
}

// aomArtifactPaths lists the AOM orchestration paths that should be excluded
// when --deliverables-only is passed to aom worktree commit. These are the
// orchestration state files that belong to AOM itself, not the deliverable code.
var aomArtifactPaths = []string{
	".agent",
	".aom",
	"AGENTS.md",
	"CLAUDE.md",
	"GEMINI.md",
	"KIRO.md",
}

// runGitAddAndCommit stages all changes and creates a commit in dir.
// If env is nil, the current process environment is used unchanged.
// If deliverablesOnly is true, AOM orchestration artifacts are unstaged after
// git add -A so the commit contains only deliverable code.
// It prints a porcelain status summary after staging so the caller can see
// exactly what is going into the commit (including deletions).
func runGitAddAndCommit(out interface{ Write([]byte) (int, error) }, commitMsg, dir string, env []string, deliverablesOnly bool) error {
	const gitTimeout = 60 * time.Second

	addCtx, addCancel := context.WithTimeout(context.Background(), gitTimeout)
	defer addCancel()
	addCmd := exec.CommandContext(addCtx, "git", "add", "-A")
	if env != nil {
		addCmd.Env = env
	}
	addCmd.Dir = dir
	if addOut, addErr := addCmd.CombinedOutput(); addErr != nil {
		if addCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git add timed out after %s", gitTimeout)
		}
		return fmt.Errorf("git add: %w\n%s", addErr, strings.TrimSpace(string(addOut)))
	}

	// --deliverables-only: unstage AOM orchestration artifacts so the commit
	// only contains deliverable code. Use "git reset HEAD -- <path>" which is
	// safe and idempotent even when paths are not staged.
	if deliverablesOnly {
		resetCtx, resetCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer resetCancel()
		resetArgs := append([]string{"reset", "HEAD", "--"}, aomArtifactPaths...)
		resetCmd := exec.CommandContext(resetCtx, "git", resetArgs...)
		if env != nil {
			resetCmd.Env = env
		}
		resetCmd.Dir = dir
		// Errors are non-fatal: if the paths are not staged the command is a no-op.
		if resetOut, resetErr := resetCmd.CombinedOutput(); resetErr == nil {
			fmt.Fprintf(out, "Unstaged AOM artifacts (--deliverables-only): %s\n", strings.Join(aomArtifactPaths, " "))
		} else {
			fmt.Fprintf(out, "Note: could not unstage AOM artifacts: %s\n", strings.TrimSpace(string(resetOut)))
		}
	}

	// Show what is staged so the agent can confirm deletions are included.
	// git add -A stages deletions of tracked files; this confirms the commit
	// will not silently skip files that were physically removed with `rm`.
	statusCtx, statusCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer statusCancel()
	statusCmd := exec.CommandContext(statusCtx, "git", "status", "--porcelain")
	if env != nil {
		statusCmd.Env = env
	}
	statusCmd.Dir = dir
	if statusOut, _ := statusCmd.Output(); len(statusOut) > 0 {
		fmt.Fprintf(out, "Staged changes:\n%s\n", strings.TrimSpace(string(statusOut)))
	}

	commitCtx, commitCancel := context.WithTimeout(context.Background(), gitTimeout)
	defer commitCancel()
	commitCmd := exec.CommandContext(commitCtx, "git", "commit", "-m", commitMsg)
	if env != nil {
		commitCmd.Env = env
	}
	commitCmd.Dir = dir
	commitOut, err := commitCmd.CombinedOutput()
	if err != nil {
		if commitCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git commit timed out after %s", gitTimeout)
		}
		return fmt.Errorf("git commit: %w\n%s", err, strings.TrimSpace(string(commitOut)))
	}
	fmt.Fprint(out, string(commitOut))
	return nil
}

// resolveWorktreeGitDir finds the GIT_DIR for a git worktree. It first reads
// the .git pointer file in the worktree; if that fails it falls back to
// scanning .git/worktrees/ for a matching gitdir entry.
// executeWorktreePrune removes archived worktrees from git and the filesystem.
// Pass --dry-run to preview what would be removed without making changes.
func (r Runner) executeWorktreePrune(args []string) error {
	dryRun := false
	for _, a := range args {
		switch a {
		case "--dry-run":
			dryRun = true
		default:
			return fmt.Errorf("unknown flag %q", a)
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	worktreeService, wDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer wDB.Close()

	records, err := worktreeService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	// Collect paths registered in git worktrees.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	gitOut, gitErr := exec.CommandContext(ctx, "git", "-C", result.Project.RepoPath, "worktree", "list", "--porcelain").Output()
	cancel()
	var gitPaths []string
	if gitErr == nil {
		gitPaths = parseGitWorktreePorcelain(string(gitOut))
	}

	pruned := 0
	for _, rec := range records {
		if rec.Status != "Archived" {
			continue
		}
		wtPath := filepath.Clean(rec.WorktreePath)

		// Check if this path is still registered with git.
		inGit := false
		for _, gp := range gitPaths {
			if filepath.Clean(gp) == wtPath {
				inGit = true
				break
			}
		}

		if !inGit {
			continue
		}

		if dryRun {
			fmt.Fprintf(r.stdout, "would prune: %s  (task: %s)\n", wtPath, rec.TaskID)
			pruned++
			continue
		}

		removeCtx, removeCancel := context.WithTimeout(context.Background(), 30*time.Second)
		out, removeErr := exec.CommandContext(removeCtx, "git", "-C", result.Project.RepoPath,
			"worktree", "remove", "--force", wtPath).CombinedOutput()
		removeCancel()
		if removeErr != nil {
			fmt.Fprintf(r.stderr, "Warning: could not remove git worktree %s: %v\n%s\n", wtPath, removeErr, strings.TrimSpace(string(out)))
		} else {
			fmt.Fprintf(r.stdout, "pruned: %s  (task: %s)\n", wtPath, rec.TaskID)
			pruned++
		}
	}

	if !dryRun {
		// Clean up any stale git worktree references.
		pruneCtx, pruneCancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, _ = exec.CommandContext(pruneCtx, "git", "-C", result.Project.RepoPath, "worktree", "prune").Output()
		pruneCancel()
	}

	if pruned == 0 {
		fmt.Fprintln(r.stdout, "No archived worktrees to prune")
	} else if dryRun {
		fmt.Fprintf(r.stdout, "\n%d worktree(s) would be pruned (dry run — no changes made)\n", pruned)
	} else {
		fmt.Fprintf(r.stdout, "\n%d worktree(s) pruned\n", pruned)
	}
	return nil
}

// parseGitWorktreePorcelain extracts worktree paths from `git worktree list --porcelain` output.
func parseGitWorktreePorcelain(output string) []string {
	var paths []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}
	return paths
}

func resolveWorktreeGitDir(repoPath, worktreePath string) (string, error) {
	gitFile := filepath.Join(worktreePath, ".git")
	data, err := os.ReadFile(gitFile)
	if err == nil {
		line := strings.TrimSpace(string(data))
		if strings.HasPrefix(line, "gitdir:") {
			candidate := strings.TrimSpace(line[len("gitdir:"):])
			if filepath.IsAbs(candidate) {
				return candidate, nil
			}
			return filepath.Clean(filepath.Join(worktreePath, candidate)), nil
		}
	}

	// Fallback: scan .git/worktrees/ for an entry whose gitdir points here.
	worktreesDir := filepath.Join(repoPath, ".git", "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return "", fmt.Errorf("read .git/worktrees: %w", err)
	}
	normTarget := filepath.ToSlash(filepath.Clean(worktreePath))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		linkData, err := os.ReadFile(filepath.Join(worktreesDir, entry.Name(), "gitdir"))
		if err != nil {
			continue
		}
		linkPath := strings.TrimSpace(string(linkData))
		norm := filepath.ToSlash(filepath.Clean(strings.TrimSuffix(linkPath, "/.git")))
		if norm == normTarget {
			return filepath.Join(worktreesDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("could not find git worktree entry for %s", worktreePath)
}

func worktreeHint(taskID string, mapping *worktree.Record, driftKind string) string {
	if mapping == nil {
		return ""
	}

	switch mapping.Status {
	case worktree.StatusNeedsRepair:
		switch driftKind {
		case worktree.DriftMissingPath:
			return fmt.Sprintf("run \"aom worktree repair %s\" to recreate the missing git worktree path before continuing", taskID)
		case worktree.DriftUnregisteredArtifactOnlyPath:
			return fmt.Sprintf("run \"aom worktree repair %s\" to recreate the unregistered worktree path; the existing path only contains AOM-owned content", taskID)
		case worktree.DriftUnregisteredDirtyPath:
			return fmt.Sprintf("inspect the existing worktree path and clean up non-artifact content manually before running \"aom worktree repair %s\"", taskID)
		default:
			return fmt.Sprintf("run \"aom worktree repair %s\" or inspect the git worktree path before continuing", taskID)
		}
	case worktree.StatusActive:
		return "task worktree is currently bound to a live session"
	default:
		return ""
	}
}

func changedFilesSummary(worktreePath, repoPath string) string {
	target := strings.TrimSpace(worktreePath)
	if target == "" {
		target = strings.TrimSpace(repoPath)
	}
	if target == "" {
		return "unavailable in current milestone"
	}

	if _, err := exec.LookPath("git"); err != nil {
		return "unavailable in current milestone"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "git", "-C", target, "status", "--short").CombinedOutput()
	if err != nil {
		return "unavailable in current milestone"
	}
	lines := strings.FieldsFunc(strings.TrimSpace(string(output)), func(r rune) bool { return r == '\n' })
	if len(lines) == 0 || strings.TrimSpace(string(output)) == "" {
		return "no local changes detected"
	}
	if len(lines) == 1 {
		return strings.TrimSpace(lines[0])
	}
	return fmt.Sprintf("%d changed paths", len(lines))
}
